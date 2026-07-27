package desktop

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
)

// TestEnvironmentCapabilityFields 验证 Environment 返回字段完整。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestEnvironmentCapabilityFields(t *testing.T) {
	reg := NewRegistry()
	env := NewEnvironmentCapability(reg, "9.9.9")
	reg.Register(env)
	reg.Register(&stubCapability{name: "desktop.other", available: true})

	available, reason, message := env.Available()
	if !available || reason != "" || message != "" {
		t.Fatalf("Available() = %v %q %q, want always available", available, reason, message)
	}
	if env.Name() != CapabilityEnvironment {
		t.Fatalf("Name() = %q, want %q", env.Name(), CapabilityEnvironment)
	}

	data, err := env.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	var info EnvironmentInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info.Platform != runtime.GOOS {
		t.Fatalf("platform = %q, want %q", info.Platform, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Fatalf("arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
	if info.AppVersion != "9.9.9" {
		t.Fatalf("appVersion = %q, want 9.9.9", info.AppVersion)
	}
	if info.OSVersion == "" {
		t.Fatal("osVersion should not be empty")
	}
	if len(info.Capabilities) != 2 {
		t.Fatalf("capabilities = %v, want 2 entries", info.Capabilities)
	}
	if info.Capabilities[0] != CapabilityEnvironment {
		t.Fatalf("first capability = %q, want %q", info.Capabilities[0], CapabilityEnvironment)
	}
}
