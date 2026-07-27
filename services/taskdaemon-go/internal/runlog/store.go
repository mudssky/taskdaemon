package runlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ArchiveStatus 表示 run 日志归档三态。
type ArchiveStatus string

const (
	// StatusAbsent 表示无归档（旧数据或从未成功落盘）。
	StatusAbsent ArchiveStatus = "absent"
	// StatusArchived 表示有可下载归档文件。
	StatusArchived ArchiveStatus = "archived"
	// StatusPruned 表示曾有归档但已被保留策略清理。
	StatusPruned ArchiveStatus = "pruned"
)

// Store 负责归档根目录解析、路径推导与文件读写。
type Store struct {
	root string
}

// NewStore 创建归档存储。
//
// 参数:
//   - root: 归档根目录；空字符串时使用默认用户配置目录下的 runlogs。
//
// 返回值:
//   - *Store: 归档存储。
//   - error: 无法解析默认根目录时返回错误。
func NewStore(root string) (*Store, error) {
	resolved, err := resolveRoot(root)
	if err != nil {
		return nil, err
	}
	return &Store{root: resolved}, nil
}

// Root 返回归档根目录绝对路径。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 归档根目录。
func (store *Store) Root() string {
	return store.root
}

// PathFor 由 runID 与开始时间推导归档文件绝对路径。
//
// 参数:
//   - runID: 执行历史 ID，必须为正整数。
//   - startedAt: run 开始时间，用于年月分目录。
//
// 返回值:
//   - string: 受控绝对路径。
//   - error: runID 非法或路径逃逸时返回错误。
func (store *Store) PathFor(runID int, startedAt time.Time) (string, error) {
	if runID <= 0 {
		return "", ErrInvalidRunID
	}
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	month := startedAt.UTC().Format("2006-01")
	name := strconv.Itoa(runID) + ".log"
	candidate := filepath.Join(store.root, month, name)
	return store.ensureInsideRoot(candidate)
}

// EnsureDir 为指定路径创建父目录（0700）。
//
// 参数:
//   - path: 目标文件绝对路径。
//
// 返回值:
//   - error: 创建目录失败时返回错误。
func (store *Store) EnsureDir(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create runlog dir: %w", err)
	}
	return nil
}

// Stat 返回归档文件元信息。
//
// 参数:
//   - path: 归档文件绝对路径。
//
// 返回值:
//   - os.FileInfo: 文件信息。
//   - error: 文件不存在或其他 IO 错误。
func (store *Store) Stat(path string) (os.FileInfo, error) {
	clean, err := store.ensureInsideRoot(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(clean)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// OpenRead 以只读方式打开归档文件。
//
// 参数:
//   - path: 归档文件绝对路径。
//
// 返回值:
//   - *os.File: 只读文件句柄。
//   - error: 打开失败或路径非法时返回错误。
func (store *Store) OpenRead(path string) (*os.File, error) {
	clean, err := store.ensureInsideRoot(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(clean)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Remove 删除归档文件；文件不存在时视为成功。
//
// 参数:
//   - path: 归档文件绝对路径。
//
// 返回值:
//   - error: 删除失败时返回错误。
func (store *Store) Remove(path string) error {
	clean, err := store.ensureInsideRoot(path)
	if err != nil {
		return err
	}
	if err := os.Remove(clean); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListEntries 扫描归档根下全部 run 日志条目。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []FileEntry: 扫描到的日志文件。
//   - error: 遍历失败时返回错误。
func (store *Store) ListEntries() ([]FileEntry, error) {
	entries := make([]FileEntry, 0)
	if _, err := os.Stat(store.root); os.IsNotExist(err) {
		return entries, nil
	}
	err := filepath.WalkDir(store.root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".log") {
			return nil
		}
		runID, parseErr := parseRunIDFromName(d.Name())
		if parseErr != nil {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		clean, cleanErr := store.ensureInsideRoot(path)
		if cleanErr != nil {
			return nil
		}
		entries = append(entries, FileEntry{
			RunID:    runID,
			Path:     clean,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			MonthDir: filepath.Base(filepath.Dir(clean)),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk runlogs: %w", err)
	}
	return entries, nil
}

// FileEntry 描述磁盘上的一条 run 日志文件。
type FileEntry struct {
	RunID    int
	Path     string
	Size     int64
	ModTime  time.Time
	MonthDir string
}

// ReadTail 从文件末尾反向读取最多 lines 行。
//
// 参数:
//   - path: 归档文件绝对路径。
//   - lines: 行数，必须 > 0。
//
// 返回值:
//   - []byte: 尾部内容（含换行）。
//   - error: 参数非法或读取失败时返回错误。
func (store *Store) ReadTail(path string, lines int) ([]byte, error) {
	if lines <= 0 {
		return nil, fmt.Errorf("%w: lines must be > 0", ErrInvalidRange)
	}
	file, err := store.OpenRead(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return []byte{}, nil
	}

	const chunkSize int64 = 8192
	var (
		pos          = size
		newlineCount = 0
		chunks       [][]byte
	)
	for pos > 0 && newlineCount <= lines {
		readSize := chunkSize
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize
		buf := make([]byte, readSize)
		if _, err := file.ReadAt(buf, pos); err != nil && err != io.EOF {
			return nil, err
		}
		chunks = append([][]byte{buf}, chunks...)
		for i := len(buf) - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				newlineCount++
				if newlineCount > lines {
					// 保留从该换行之后的内容
					offsetInChunk := i + 1
					prefixLen := 0
					for _, c := range chunks[:len(chunks)-1] {
						prefixLen += len(c)
					}
					// 当前块是 chunks 的最后一块（最旧）
					// 重新组装从 offset 开始
					assembled := make([]byte, 0, size-pos)
					first := buf[offsetInChunk:]
					assembled = append(assembled, first...)
					for _, c := range chunks[1:] {
						assembled = append(assembled, c...)
					}
					return assembled, nil
				}
			}
		}
	}
	assembled := make([]byte, 0, size)
	for _, c := range chunks {
		assembled = append(assembled, c...)
	}
	return assembled, nil
}

// ReadRange 按字节偏移读取指定长度。
//
// 参数:
//   - path: 归档文件绝对路径。
//   - offset: 起始字节偏移，必须 >= 0。
//   - length: 读取长度，必须 > 0。
//
// 返回值:
//   - []byte: 读取到的内容（可能短于 length）。
//   - error: 参数非法或读取失败时返回错误。
func (store *Store) ReadRange(path string, offset int64, length int64) ([]byte, error) {
	if offset < 0 || length <= 0 {
		return nil, fmt.Errorf("%w: offset/length", ErrInvalidRange)
	}
	file, err := store.OpenRead(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if offset > info.Size() {
		return nil, fmt.Errorf("%w: offset beyond file size", ErrInvalidRange)
	}
	buf := make([]byte, length)
	n, err := file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}

// ensureInsideRoot 清理路径并确保位于归档根内。
//
// 参数:
//   - path: 候选路径。
//
// 返回值:
//   - string: 清理后的绝对路径。
//   - error: 逃逸根目录时返回 ErrPathEscape。
func (store *Store) ensureInsideRoot(path string) (string, error) {
	root := filepath.Clean(store.root)
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	clean := filepath.Clean(abs)
	sep := string(os.PathSeparator)
	if clean != root && !strings.HasPrefix(clean, root+sep) {
		return "", ErrPathEscape
	}
	return clean, nil
}

// resolveRoot 解析归档根目录。
//
// 参数:
//   - override: 测试或装配注入的根目录。
//
// 返回值:
//   - string: 绝对路径。
//   - error: 用户配置目录不可用时返回错误。
func resolveRoot(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return filepath.Clean(override), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(configDir, "taskdaemon", "runlogs"), nil
}

// parseRunIDFromName 从 `<runID>.log` 文件名解析 runID。
//
// 参数:
//   - name: 文件名。
//
// 返回值:
//   - int: runID。
//   - error: 文件名不符合约定时返回错误。
func parseRunIDFromName(name string) (int, error) {
	base := strings.TrimSuffix(name, ".log")
	if base == name || base == "" {
		return 0, fmt.Errorf("invalid runlog name: %s", name)
	}
	// 拒绝路径片段，只接受纯数字
	for _, r := range base {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid runlog name: %s", name)
		}
	}
	id, err := strconv.Atoi(base)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid runlog name: %s", name)
	}
	return id, nil
}
