package desktop

import (
	"context"
	"encoding/json"
	"testing"
)

type stubTrayHost struct {
	available      bool
	reason         UnavailableReason
	message        string
	minimizeToTray bool
	showCalls      int
}

func (s *stubTrayHost) Available() (bool, UnavailableReason, string) {
	return s.available, s.reason, s.message
}
func (s *stubTrayHost) ShowMainWindow() { s.showCalls++ }
func (s *stubTrayHost) MinimizeToTrayEnabled() bool {
	return s.minimizeToTray
}
func (s *stubTrayHost) SetMinimizeToTray(enabled bool) { s.minimizeToTray = enabled }
func (s *stubTrayHost) StatusSnapshot() TrayStatusInfo {
	return TrayStatusInfo{
		ServiceStatus:  TrayServiceRunning,
		MinimizeToTray: s.minimizeToTray,
		Label:          "运行中",
	}
}

// TestTrayCapabilityActions 验证 status / show / setMinimize 路由。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestTrayCapabilityActions(t *testing.T) {
	host := &stubTrayHost{available: true, minimizeToTray: true}
	cap := NewTrayCapability(host)
	if name := cap.Name(); name != CapabilityTray {
		t.Fatalf("name = %q", name)
	}

	data, err := cap.Invoke(context.Background(), []byte(`{"action":"status"}`))
	if err != nil {
		t.Fatal(err)
	}
	var info TrayStatusInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	if info.ServiceStatus != TrayServiceRunning || !info.MinimizeToTray {
		t.Fatalf("info = %+v", info)
	}

	if _, err := cap.Invoke(context.Background(), []byte(`{"action":"showWindow"}`)); err != nil {
		t.Fatal(err)
	}
	if host.showCalls != 1 {
		t.Fatalf("showCalls = %d", host.showCalls)
	}

	enabled := false
	payload, _ := json.Marshal(map[string]any{"action": "setMinimizeToTray", "enabled": enabled})
	if _, err := cap.Invoke(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if host.minimizeToTray {
		t.Fatal("expected minimizeToTray false")
	}
}

// TestTrayCapabilityUnavailable 验证不可用时返回 DESKTOP_CAPABILITY_UNAVAILABLE。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestTrayCapabilityUnavailable(t *testing.T) {
	cap := NewTrayCapability(&stubTrayHost{available: false, reason: ReasonPlatform, message: "off"})
	_, err := cap.Invoke(context.Background(), nil)
	var capErr *CapabilityError
	if !asCapabilityError(err, &capErr) || capErr.Code != ErrCodeCapabilityUnavailable {
		t.Fatalf("err = %v", err)
	}
}
