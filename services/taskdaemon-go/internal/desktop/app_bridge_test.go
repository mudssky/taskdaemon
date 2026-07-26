package desktop

import (
	"encoding/json"
	"testing"

	"taskdaemon/internal/config"
)

// TestAppListAndInvokeEnvironment 验证 Wails binding 入口拉清单并调用 Environment。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestAppListAndInvokeEnvironment(t *testing.T) {
	bindings := newApp(config.Default())
	list := bindings.ListCapabilities()
	// TD1 Environment + TD2 Tray/Autostart/WindowState
	if len(list) != 4 {
		t.Fatalf("list len = %d, want 4 (%v)", len(list), list)
	}
	if list[0].Name != CapabilityEnvironment || !list[0].Available {
		t.Fatalf("list[0] = %+v, want available environment", list[0])
	}
	wantNames := map[string]bool{
		CapabilityEnvironment: false,
		CapabilityTray:        false,
		CapabilityAutostart:   false,
		CapabilityWindowState: false,
	}
	for _, item := range list {
		if _, ok := wantNames[item.Name]; !ok {
			t.Fatalf("unexpected capability %q", item.Name)
		}
		wantNames[item.Name] = true
	}
	for name, seen := range wantNames {
		if !seen {
			t.Fatalf("missing capability %q", name)
		}
	}

	result := bindings.InvokeCapability(CapabilityEnvironment, "")
	if !result.OK {
		t.Fatalf("invoke failed: %+v", result)
	}
	var info EnvironmentInfo
	if err := json.Unmarshal(result.Data, &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info.AppVersion == "" || len(info.Capabilities) == 0 {
		t.Fatalf("info incomplete: %+v", info)
	}

	missing := bindings.InvokeCapability("desktop.missing", "")
	if missing.OK || missing.Code != ErrCodeCapabilityNotFound {
		t.Fatalf("missing = %+v, want NOT_FOUND", missing)
	}
}
