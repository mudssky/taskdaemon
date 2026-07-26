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
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if list[0].Name != CapabilityEnvironment || !list[0].Available {
		t.Fatalf("list[0] = %+v, want available environment", list[0])
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
