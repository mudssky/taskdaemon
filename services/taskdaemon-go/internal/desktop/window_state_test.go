package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadWindowStateCorruptFallsBack 验证损坏文件回退默认且不阻塞。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestLoadWindowStateCorruptFallsBack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "window-state.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, ok := LoadWindowState(path)
	if ok {
		t.Fatal("corrupt file should not report ok")
	}
	def := DefaultWindowState()
	if state.Width != def.Width || state.Height != def.Height {
		t.Fatalf("state = %+v, want default", state)
	}
}

// TestLoadAndSaveWindowStateRoundTrip 验证正常读写。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestLoadAndSaveWindowStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-state.json")
	want := WindowState{X: 10, Y: 20, Width: 1000, Height: 700, Maximised: true}
	if err := SaveWindowState(path, want); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadWindowState(path)
	if !ok {
		t.Fatal("expected ok load")
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

// TestClampWindowStateOffscreen 验证落在屏外时夹回主屏。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestClampWindowStateOffscreen(t *testing.T) {
	displays := []DisplayRect{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	off := WindowState{X: 8000, Y: 8000, Width: 1000, Height: 700}
	clamped, usedDefault := ClampWindowState(off, displays)
	if usedDefault {
		t.Fatal("should clamp to primary, not full default")
	}
	if clamped.X < 0 || clamped.Y < 0 || clamped.X > 200 {
		t.Fatalf("clamped position unexpected: %+v", clamped)
	}
	if !windowIntersectsAny(clamped, displays) {
		t.Fatalf("clamped still invisible: %+v", clamped)
	}
}

// TestClampWindowStateNoDisplays 验证无显示器时回默认。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestClampWindowStateNoDisplays(t *testing.T) {
	state, usedDefault := ClampWindowState(WindowState{X: 1, Y: 1, Width: 1000, Height: 700}, nil)
	if !usedDefault {
		t.Fatal("expected default layout")
	}
	if state.Width != defaultWindowWidth {
		t.Fatalf("width = %d", state.Width)
	}
}
