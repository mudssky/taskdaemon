package runlog

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestPathForDerivesMonthAndRunID 验证路径可从 runID 与时间推导。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestPathForDerivesMonthAndRunID(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	started := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	path, err := store.PathFor(42, started)
	if err != nil {
		t.Fatalf("path for: %v", err)
	}
	want := filepath.Join(root, "2026-07", "42.log")
	if path != want {
		t.Fatalf("path = %s, want %s", path, want)
	}
}

// TestPathForRejectsInvalidRunID 验证非法 runID 被拒绝。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestPathForRejectsInvalidRunID(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if _, err := store.PathFor(0, time.Now()); err == nil {
		t.Fatal("expected error for runID=0")
	}
	if _, err := store.PathFor(-1, time.Now()); err == nil {
		t.Fatal("expected error for negative runID")
	}
}

// TestEnsureInsideRootBlocksEscape 验证路径逃逸被拒绝。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestEnsureInsideRootBlocksEscape(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if _, err := store.ensureInsideRoot(filepath.Join(root, "..", "etc", "passwd")); err == nil {
		t.Fatal("expected path escape error")
	}
}

// TestWriterPrefixesStdoutStderr 验证 stdout/stderr 前缀与时序。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestWriterPrefixesStdoutStderr(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	started := time.Now()
	archive, err := OpenArchive(store, 7, started, nil)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	if _, err := archive.StdoutWriter().Write([]byte("hello\n")); err != nil {
		t.Fatalf("stdout write: %v", err)
	}
	if _, err := archive.StderrWriter().Write([]byte("oops\n")); err != nil {
		t.Fatalf("stderr write: %v", err)
	}
	if _, err := archive.StdoutWriter().Write([]byte("world")); err != nil {
		t.Fatalf("stdout write2: %v", err)
	}
	size, err := archive.Close()
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if size <= 0 {
		t.Fatalf("size = %d, want > 0", size)
	}
	data, err := os.ReadFile(archive.Path())
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	got := string(data)
	want := "[stdout] hello\n[stderr] oops\n[stdout] world"
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	info, err := os.Stat(archive.Path())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("file perm = %o, want 0600", info.Mode().Perm())
		}
	}
}

// TestWriterFailureDoesNotPropagate 验证写失败不向上返回 error。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestWriterFailureDoesNotPropagate(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	archive, err := OpenArchive(store, 9, time.Now(), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// 关闭底层文件；写入超过缓冲以触发真正的磁盘写失败
	_ = archive.file.Close()
	payload := make([]byte, writeBufferSize+64)
	for i := range payload {
		payload[i] = 'x'
	}
	n, writeErr := archive.StdoutWriter().Write(payload)
	if writeErr != nil {
		t.Fatalf("write error propagated: %v", writeErr)
	}
	if n != len(payload) {
		t.Fatalf("n = %d, want %d", n, len(payload))
	}
	if !archive.WriteFailed() {
		// 若缓冲尚未 flush 失败，Close 也应标记失败且仍不 panic
		_, _ = archive.Close()
		if !archive.WriteFailed() {
			t.Fatal("expected write failed mark")
		}
		return
	}
	_, _ = archive.Close()
}

// TestReadTailAndRange 验证尾部与范围读取。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestReadTailAndRange(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	path := filepath.Join(root, "2026-07", "1.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := []byte("line1\nline2\nline3\nline4\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	tail, err := store.ReadTail(path, 2)
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if string(tail) != "line3\nline4\n" {
		t.Fatalf("tail = %q", string(tail))
	}
	ranged, err := store.ReadRange(path, 6, 5)
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if string(ranged) != "line2" {
		t.Fatalf("range = %q", string(ranged))
	}
	if _, err := store.ReadRange(path, -1, 1); err == nil {
		t.Fatal("expected invalid range")
	}
	if _, err := store.ReadTail(path, 0); err == nil {
		t.Fatal("expected invalid lines")
	}
}

// TestParseRunIDFromNameRejectsTraversal 验证文件名解析拒绝非数字。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestParseRunIDFromNameRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../1.log", "1../2.log", "abc.log", ".log", "1.txt"} {
		if _, err := parseRunIDFromName(name); err == nil {
			t.Fatalf("expected reject for %q", name)
		}
	}
	id, err := parseRunIDFromName("123.log")
	if err != nil || id != 123 {
		t.Fatalf("parse = %d, %v", id, err)
	}
}
