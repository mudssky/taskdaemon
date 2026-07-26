package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
)

// CapabilityEnvironment 是样板能力名：查询 Desktop 环境信息。
const CapabilityEnvironment = "desktop.environment"

// appVersion 是 Environment 返回的应用版本占位；后续可由 build ldflags 注入。
const appVersion = "0.1.0"

// EnvironmentInfo 是 Environment capability 的返回形状。
type EnvironmentInfo struct {
	Platform     string   `json:"platform"`
	Arch         string   `json:"arch"`
	OSVersion    string   `json:"osVersion"`
	AppVersion   string   `json:"appVersion"`
	Capabilities []string `json:"capabilities"`
}

// environmentCapability 实现最小真实能力，验证注册 → 调用 → 前端链路。
type environmentCapability struct {
	registry *Registry
	version  string
}

// NewEnvironmentCapability 创建 Environment 样板 capability。
//
// 参数:
//   - registry: 用于列出已注册能力；可为 nil（capabilities 返回空）。
//   - version: 应用版本；空则使用默认 appVersion。
//
// 返回值:
//   - Capability: 可 Register 的 Environment 实现。
func NewEnvironmentCapability(registry *Registry, version string) Capability {
	if version == "" {
		version = appVersion
	}
	return &environmentCapability{
		registry: registry,
		version:  version,
	}
}

// Name 返回 capability 名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: desktop.environment。
func (e *environmentCapability) Name() string {
	return CapabilityEnvironment
}

// Available 始终可用（三平台、无权限要求）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: true。
//   - UnavailableReason: 空。
//   - string: 空消息。
func (e *environmentCapability) Available() (bool, UnavailableReason, string) {
	return true, "", ""
}

// Invoke 返回平台名、OS 版本、应用版本与已注册能力清单。
//
// 参数:
//   - ctx: 调用上下文（本能力不使用）。
//   - payload: 忽略；允许空。
//
// 返回值:
//   - []byte: EnvironmentInfo 的 JSON。
//   - error: 序列化失败时返回。
func (e *environmentCapability) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	_ = ctx
	_ = payload

	names := []string{}
	if e.registry != nil {
		names = e.registry.Names()
	}

	info := EnvironmentInfo{
		Platform:     runtime.GOOS,
		Arch:         runtime.GOARCH,
		OSVersion:    runtime.GOOS + "/" + runtime.GOARCH,
		AppVersion:   e.version,
		Capabilities: names,
	}
	data, err := json.Marshal(info)
	if err != nil {
		return nil, &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: fmt.Sprintf("marshal environment info: %v", err),
		}
	}
	return data, nil
}
