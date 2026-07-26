package desktop

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

type stubWindowStateHost struct {
	enabled bool
	info    WindowStateInfo
	resets  int
}

func (s *stubWindowStateHost) Available() (bool, UnavailableReason, string) {
	if !s.enabled {
		return false, ReasonPlatform, "off"
	}
	return true, "", ""
}
func (s *stubWindowStateHost) Snapshot() WindowStateInfo { return s.info }
func (s *stubWindowStateHost) Reset() error {
	s.resets++
	s.info = WindowStateInfo{
		Width: defaultWindowWidth, Height: defaultWindowHeight, Enabled: true,
		Path: s.info.Path,
	}
	return nil
}

// TestWindowStateCapabilityGetReset 验证 get/reset。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestWindowStateCapabilityGetReset(t *testing.T) {
	host := &stubWindowStateHost{
		enabled: true,
		info: WindowStateInfo{
			X: 1, Y: 2, Width: 1000, Height: 700, Enabled: true,
			Path: filepath.Join(t.TempDir(), "ws.json"),
		},
	}
	cap := NewWindowStateCapability(host)
	if cap.Name() != CapabilityWindowState {
		t.Fatalf("name = %s", cap.Name())
	}
	data, err := cap.Invoke(context.Background(), []byte(`{"action":"get"}`))
	if err != nil {
		t.Fatal(err)
	}
	var info WindowStateInfo
	if err := json.Unmarshal(data, &info); err != nil || info.Width != 1000 {
		t.Fatalf("info = %+v err=%v", info, err)
	}
	if _, err := cap.Invoke(context.Background(), []byte(`{"action":"reset"}`)); err != nil {
		t.Fatal(err)
	}
	if host.resets != 1 {
		t.Fatalf("resets = %d", host.resets)
	}
}
