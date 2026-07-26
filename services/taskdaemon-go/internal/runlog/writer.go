package runlog

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	stdoutPrefix = "[stdout] "
	stderrPrefix = "[stderr] "
	// writeBufferSize 是落盘缓冲大小；写路径失败不影响 runner。
	writeBufferSize = 64 * 1024
)

// Archive 表示一次 run 的落盘会话。
type Archive struct {
	runID   int
	path    string
	file    *os.File
	buf     *bufio.Writer
	mu      sync.Mutex
	failed  bool
	failErr error
	closed  bool
	onFail  func(error)

	stdout *streamWriter
	stderr *streamWriter
}

// OpenArchive 创建并打开一次 run 的归档写入器。
//
// 参数:
//   - store: 归档存储。
//   - runID: 执行历史 ID。
//   - startedAt: run 开始时间。
//   - onFail: 首次写失败回调；可为 nil。
//
// 返回值:
//   - *Archive: 成功打开时的写入器。
//   - error: 打开失败时返回错误（调用方应记进程日志并继续 run）。
func OpenArchive(store *Store, runID int, startedAt time.Time, onFail func(error)) (*Archive, error) {
	path, err := store.PathFor(runID, startedAt)
	if err != nil {
		return nil, err
	}
	if err := store.EnsureDir(path); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open runlog file: %w", err)
	}
	archive := &Archive{
		runID:  runID,
		path:   path,
		file:   file,
		buf:    bufio.NewWriterSize(file, writeBufferSize),
		onFail: onFail,
	}
	archive.stdout = &streamWriter{archive: archive, prefix: stdoutPrefix, atBOL: true}
	archive.stderr = &streamWriter{archive: archive, prefix: stderrPrefix, atBOL: true}
	return archive, nil
}

// RunID 返回关联的执行历史 ID。
//
// 参数:
//   - 无。
//
// 返回值:
//   - int: runID。
func (archive *Archive) RunID() int {
	return archive.runID
}

// Path 返回归档文件绝对路径。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 绝对路径。
func (archive *Archive) Path() string {
	return archive.path
}

// StdoutWriter 返回 stdout 镜像写入器；写失败永不向上传播。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *streamWriter: stdout 前缀写入器。
func (archive *Archive) StdoutWriter() *streamWriter {
	return archive.stdout
}

// StderrWriter 返回 stderr 镜像写入器；写失败永不向上传播。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *streamWriter: stderr 前缀写入器。
func (archive *Archive) StderrWriter() *streamWriter {
	return archive.stderr
}

// WriteFailed 返回是否发生过写失败。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: true 表示至少一次写失败。
func (archive *Archive) WriteFailed() bool {
	archive.mu.Lock()
	defer archive.mu.Unlock()
	return archive.failed
}

// WriteError 返回首次写失败错误。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 首次失败错误；无失败时为 nil。
func (archive *Archive) WriteError() error {
	archive.mu.Lock()
	defer archive.mu.Unlock()
	return archive.failErr
}

// Close 刷盘并关闭文件。
//
// 参数:
//   - 无。
//
// 返回值:
//   - int64: 关闭后文件大小；失败时为 0。
//   - error: 关闭过程错误（不影响 run 成败，仅供标记）。
func (archive *Archive) Close() (int64, error) {
	archive.mu.Lock()
	defer archive.mu.Unlock()
	if archive.closed {
		return 0, nil
	}
	archive.closed = true
	var closeErr error
	if archive.buf != nil {
		if err := archive.buf.Flush(); err != nil {
			archive.markFailedLocked(err)
			closeErr = err
		}
	}
	if archive.file != nil {
		if err := archive.file.Close(); err != nil && closeErr == nil {
			closeErr = err
			archive.markFailedLocked(err)
		}
	}
	if archive.failed {
		return 0, archive.failErr
	}
	info, err := os.Stat(archive.path)
	if err != nil {
		return 0, err
	}
	return info.Size(), closeErr
}

// streamWriter 为 stdout/stderr 添加行级前缀并串行写入同一文件。
type streamWriter struct {
	archive *Archive
	prefix  string
	atBOL   bool
}

// Write 写入带前缀的输出；永远返回成功，避免 MultiWriter 中断 runner。
//
// 参数:
//   - p: 本次输出字节。
//
// 返回值:
//   - int: 始终等于 len(p)。
//   - error: 始终为 nil。
func (writer *streamWriter) Write(p []byte) (int, error) {
	total := len(p)
	if total == 0 {
		return 0, nil
	}
	writer.archive.mu.Lock()
	defer writer.archive.mu.Unlock()
	if writer.archive.closed || writer.archive.failed {
		return total, nil
	}
	remaining := p
	for len(remaining) > 0 {
		if writer.atBOL {
			if err := writer.writeRawLocked([]byte(writer.prefix)); err != nil {
				return total, nil
			}
			writer.atBOL = false
		}
		idx := indexByte(remaining, '\n')
		if idx < 0 {
			if err := writer.writeRawLocked(remaining); err != nil {
				return total, nil
			}
			break
		}
		if err := writer.writeRawLocked(remaining[:idx+1]); err != nil {
			return total, nil
		}
		writer.atBOL = true
		remaining = remaining[idx+1:]
	}
	return total, nil
}

// writeRawLocked 在已持锁状态下写入缓冲。
//
// 参数:
//   - p: 原始字节。
//
// 返回值:
//   - error: 写入失败时返回错误。
func (writer *streamWriter) writeRawLocked(p []byte) error {
	if _, err := writer.archive.buf.Write(p); err != nil {
		writer.archive.markFailedLocked(err)
		return err
	}
	return nil
}

// markFailedLocked 记录首次写失败并触发回调。
//
// 参数:
//   - err: 失败原因。
//
// 返回值:
//   - 无。
func (archive *Archive) markFailedLocked(err error) {
	if archive.failed {
		return
	}
	archive.failed = true
	archive.failErr = err
	if archive.onFail != nil {
		archive.onFail(err)
	}
}

// indexByte 返回字节在切片中的位置。
//
// 参数:
//   - p: 字节切片。
//   - c: 目标字节。
//
// 返回值:
//   - int: 下标；不存在时为 -1。
func indexByte(p []byte, c byte) int {
	for i, b := range p {
		if b == c {
			return i
		}
	}
	return -1
}
