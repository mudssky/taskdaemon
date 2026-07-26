package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type stubCapability struct {
	name      string
	available bool
	reason    UnavailableReason
	message   string
	payload   []byte
	err       error
	panicMsg  string
}

func (s *stubCapability) Name() string { return s.name }

func (s *stubCapability) Available() (bool, UnavailableReason, string) {
	return s.available, s.reason, s.message
}

func (s *stubCapability) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	_ = ctx
	if s.panicMsg != "" {
		panic(s.panicMsg)
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.payload != nil {
		return s.payload, nil
	}
	return payload, nil
}

// TestRegistryRegisterAndList 验证注册顺序与 Available 描述。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRegistryRegisterAndList(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubCapability{name: "desktop.a", available: true})
	reg.Register(&stubCapability{
		name:      "desktop.b",
		available: false,
		reason:    ReasonPlatform,
		message:   "no tray on this OS",
	})

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].Name != "desktop.a" || !list[0].Available {
		t.Fatalf("first = %+v, want available desktop.a", list[0])
	}
	if list[1].Name != "desktop.b" || list[1].Available || list[1].Reason != ReasonPlatform {
		t.Fatalf("second = %+v, want unavailable platform", list[1])
	}
}

// TestRegistryInvokeNotFound 验证未注册名返回 DESKTOP_CAPABILITY_NOT_FOUND。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRegistryInvokeNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Invoke(context.Background(), "desktop.missing", nil)
	capErr, ok := err.(*CapabilityError)
	if !ok {
		t.Fatalf("err type = %T, want *CapabilityError", err)
	}
	if capErr.Code != ErrCodeCapabilityNotFound {
		t.Fatalf("code = %q, want %q", capErr.Code, ErrCodeCapabilityNotFound)
	}
}

// TestRegistryInvokeUnavailable 验证不可用能力返回结构化 reason 对应错误码。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRegistryInvokeUnavailable(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubCapability{
		name:      "desktop.locked",
		available: false,
		reason:    ReasonPermission,
		message:   "notification permission denied",
	})

	_, err := reg.Invoke(context.Background(), "desktop.locked", nil)
	capErr, ok := err.(*CapabilityError)
	if !ok {
		t.Fatalf("err type = %T, want *CapabilityError", err)
	}
	if capErr.Code != ErrCodePermissionDenied {
		t.Fatalf("code = %q, want %q", capErr.Code, ErrCodePermissionDenied)
	}
	if !strings.Contains(capErr.Message, "permission") {
		t.Fatalf("message = %q, want permission hint", capErr.Message)
	}
}

// TestRegistryInvokeRecoversPanic 验证 capability panic 被 recover，不向上传播。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRegistryInvokeRecoversPanic(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubCapability{
		name:      "desktop.boom",
		available: true,
		panicMsg:  "boom",
	})

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("panic escaped registry: %v", recovered)
		}
	}()

	_, err := reg.Invoke(context.Background(), "desktop.boom", nil)
	capErr, ok := err.(*CapabilityError)
	if !ok {
		t.Fatalf("err type = %T, want *CapabilityError", err)
	}
	if capErr.Code != ErrCodeInvokeFailed {
		t.Fatalf("code = %q, want %q", capErr.Code, ErrCodeInvokeFailed)
	}
	if !strings.Contains(capErr.Message, "panicked") {
		t.Fatalf("message = %q, want panicked", capErr.Message)
	}
}

// TestRegistryInvokeSuccess 验证可用能力返回 payload 透传结果。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRegistryInvokeSuccess(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubCapability{
		name:      "desktop.echo",
		available: true,
		payload:   []byte(`{"ok":true}`),
	})

	data, err := reg.Invoke(context.Background(), "desktop.echo", []byte(`{}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("data = %s, want ok payload", data)
	}
}

// TestRegistryInvokeWrapsPlainError 验证普通 error 被包装为 DESKTOP_INVOKE_FAILED。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRegistryInvokeWrapsPlainError(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubCapability{
		name:      "desktop.fail",
		available: true,
		err:       errors.New("disk full"),
	})

	_, err := reg.Invoke(context.Background(), "desktop.fail", nil)
	capErr, ok := err.(*CapabilityError)
	if !ok {
		t.Fatalf("err type = %T, want *CapabilityError", err)
	}
	if capErr.Code != ErrCodeInvokeFailed {
		t.Fatalf("code = %q, want %q", capErr.Code, ErrCodeInvokeFailed)
	}
}

// TestInvokeAsResult 验证折叠后的 InvokeResult 形状。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestInvokeAsResult(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubCapability{
		name:      "desktop.echo",
		available: true,
		payload:   []byte(`{"n":1}`),
	})

	okResult := invokeAsResult(context.Background(), reg, "desktop.echo", "")
	if !okResult.OK || string(okResult.Data) != `{"n":1}` {
		t.Fatalf("ok result = %+v", okResult)
	}

	failResult := invokeAsResult(context.Background(), reg, "desktop.missing", "")
	if failResult.OK || failResult.Code != ErrCodeCapabilityNotFound {
		t.Fatalf("fail result = %+v", failResult)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(mustJSON(failResult)), &raw); err != nil {
		t.Fatalf("marshal fail result: %v", err)
	}
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}
