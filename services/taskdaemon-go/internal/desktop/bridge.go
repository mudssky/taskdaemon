package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// UnavailableReason 描述 Desktop capability 不可用的原因。
// 与前端 CapabilityState.reason 对齐；not-desktop 仅由前端判断。
type UnavailableReason string

const (
	// ReasonPlatform 表示当前 OS/壳不支持该能力。
	ReasonPlatform UnavailableReason = "platform"
	// ReasonPermission 表示能力存在但权限未授予。
	ReasonPermission UnavailableReason = "permission"
	// ReasonError 表示检测本身失败。
	ReasonError UnavailableReason = "error"
)

// DESKTOP_* 错误码（路线图 §9.1 / C-5）。
const (
	ErrCodeCapabilityUnavailable = "DESKTOP_CAPABILITY_UNAVAILABLE"
	ErrCodeCapabilityNotFound    = "DESKTOP_CAPABILITY_NOT_FOUND"
	ErrCodePermissionDenied      = "DESKTOP_PERMISSION_DENIED"
	ErrCodeInvokeFailed          = "DESKTOP_INVOKE_FAILED"
)

// Capability 是 Go 侧 Desktop 能力的统一注册接口。
// D2/D3 实现本接口并在装配处 Register，不改注册框架。
type Capability interface {
	// Name 返回 capability 具名常量，例如 desktop.environment。
	Name() string
	// Available 返回当前平台是否可用及结构化原因。
	Available() (bool, UnavailableReason, string)
	// Invoke 执行能力；payload 为 JSON 字节，返回 JSON 字节。
	Invoke(ctx context.Context, payload []byte) ([]byte, error)
}

// CapabilityDescriptor 是暴露给前端的能力清单条目。
type CapabilityDescriptor struct {
	Name      string            `json:"name"`
	Available bool              `json:"available"`
	Reason    UnavailableReason `json:"reason,omitempty"`
	Message   string            `json:"message,omitempty"`
}

// CapabilityError 是 capability 调用的结构化错误。
type CapabilityError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error 实现 error 接口。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 稳定错误码与消息拼接。
func (e *CapabilityError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

// InvokeResult 是 Wails binding 的统一调用结果。
// 业务失败放在 Code/Message，不依赖 Go error 传播。
type InvokeResult struct {
	OK      bool            `json:"ok"`
	Data    json.RawMessage `json:"data,omitempty"`
	Code    string          `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
}

// Registry 集中管理 Desktop capability 注册与调用。
type Registry struct {
	mu    sync.RWMutex
	items map[string]Capability
	order []string
}

// NewRegistry 创建空的 capability 注册表。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *Registry: 可 Register 的注册表实例。
func NewRegistry() *Registry {
	return &Registry{
		items: make(map[string]Capability),
		order: make([]string, 0),
	}
}

// Register 注册一个 capability；同名覆盖并保持首次顺序。
//
// 参数:
//   - c: 待注册的 capability，nil 被忽略。
//
// 返回值:
//   - 无。
func (r *Registry) Register(c Capability) {
	if r == nil || c == nil {
		return
	}
	name := c.Name()
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[name]; !exists {
		r.order = append(r.order, name)
	}
	r.items[name] = c
}

// List 返回已注册能力的描述清单（按注册顺序）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []CapabilityDescriptor: 供前端缓存的能力清单。
func (r *Registry) List() []CapabilityDescriptor {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CapabilityDescriptor, 0, len(r.order))
	for _, name := range r.order {
		c := r.items[name]
		available, reason, message := c.Available()
		desc := CapabilityDescriptor{
			Name:      name,
			Available: available,
			Message:   message,
		}
		if !available {
			desc.Reason = reason
		}
		out = append(out, desc)
	}
	return out
}

// Names 返回已注册 capability 名称（按注册顺序）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []string: capability 名称列表。
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Invoke 按名称调用 capability；外层统一 recover，禁止 panic 杀进程。
//
// 参数:
//   - ctx: 调用上下文。
//   - name: capability 具名常量。
//   - payload: JSON 负载，可为 nil。
//
// 返回值:
//   - []byte: 成功时的 JSON 结果。
//   - error: 结构化 CapabilityError 或包装错误。
func (r *Registry) Invoke(ctx context.Context, name string, payload []byte) (result []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = &CapabilityError{
				Code:    ErrCodeInvokeFailed,
				Message: fmt.Sprintf("capability %q panicked: %v", name, recovered),
			}
		}
	}()

	if r == nil {
		return nil, &CapabilityError{
			Code:    ErrCodeCapabilityNotFound,
			Message: fmt.Sprintf("capability %q is not registered", name),
		}
	}

	r.mu.RLock()
	c, ok := r.items[name]
	r.mu.RUnlock()
	if !ok {
		return nil, &CapabilityError{
			Code:    ErrCodeCapabilityNotFound,
			Message: fmt.Sprintf("capability %q is not registered", name),
		}
	}

	available, reason, message := c.Available()
	if !available {
		code := ErrCodeCapabilityUnavailable
		if reason == ReasonPermission {
			code = ErrCodePermissionDenied
		}
		if message == "" {
			message = fmt.Sprintf("capability %q is unavailable", name)
		}
		return nil, &CapabilityError{Code: code, Message: message}
	}

	if ctx == nil {
		ctx = context.Background()
	}
	data, invokeErr := c.Invoke(ctx, payload)
	if invokeErr != nil {
		var capErr *CapabilityError
		if asCapabilityError(invokeErr, &capErr) {
			return nil, capErr
		}
		return nil, &CapabilityError{
			Code:    ErrCodeInvokeFailed,
			Message: invokeErr.Error(),
		}
	}
	return data, nil
}

// asCapabilityError 将 error 断言为 *CapabilityError。
//
// 参数:
//   - err: 原始错误。
//   - target: 输出指针。
//
// 返回值:
//   - bool: 是否为 CapabilityError。
func asCapabilityError(err error, target **CapabilityError) bool {
	if err == nil || target == nil {
		return false
	}
	if capErr, ok := err.(*CapabilityError); ok {
		*target = capErr
		return true
	}
	return false
}

// invokeAsResult 将 Registry.Invoke 结果折叠为 InvokeResult。
//
// 参数:
//   - ctx: 调用上下文。
//   - registry: capability 注册表。
//   - name: capability 名称。
//   - payloadJSON: JSON 字符串负载。
//
// 返回值:
//   - InvokeResult: 始终可序列化的调用结果。
func invokeAsResult(ctx context.Context, registry *Registry, name string, payloadJSON string) InvokeResult {
	var payload []byte
	if payloadJSON != "" {
		payload = []byte(payloadJSON)
	}
	data, err := registry.Invoke(ctx, name, payload)
	if err != nil {
		code := ErrCodeInvokeFailed
		message := err.Error()
		if capErr, ok := err.(*CapabilityError); ok {
			code = capErr.Code
			message = capErr.Message
		}
		return InvokeResult{OK: false, Code: code, Message: message}
	}
	return InvokeResult{OK: true, Data: json.RawMessage(data)}
}
