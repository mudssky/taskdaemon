package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"taskdaemon/internal/notify"
)

type stubNotificationBackend struct {
	authorized bool
	checkErr   error
	requestErr error
	sendErr    error
	sent       []notifications.NotificationOptions
	requests   int
}

func (s *stubNotificationBackend) RequestNotificationAuthorization() (bool, error) {
	s.requests++
	if s.requestErr != nil {
		return false, s.requestErr
	}
	return s.authorized, nil
}

func (s *stubNotificationBackend) CheckNotificationAuthorization() (bool, error) {
	if s.checkErr != nil {
		return false, s.checkErr
	}
	return s.authorized, nil
}

func (s *stubNotificationBackend) SendNotification(options notifications.NotificationOptions) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, options)
	return nil
}

// TestNotificationCapabilityAvailableWithBackend 验证有后端即可用。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationCapabilityAvailableWithBackend(t *testing.T) {
	cap := NewNotificationCapability(nil, NotificationCapabilityOptions{
		Backend: &stubNotificationBackend{authorized: false},
	})
	ok, reason, msg := cap.Available()
	if !ok || reason != "" || msg != "" {
		t.Fatalf("Available() = %v %q %q", ok, reason, msg)
	}
	if cap.Name() != CapabilityNotification {
		t.Fatalf("Name = %q", cap.Name())
	}
}

// TestNotificationCapabilityUnavailableWithoutBackend 验证无后端不可用。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationCapabilityUnavailableWithoutBackend(t *testing.T) {
	cap := NewNotificationCapability(nil, NotificationCapabilityOptions{})
	ok, reason, _ := cap.Available()
	if ok || reason != ReasonPlatform {
		t.Fatalf("Available() = %v %q, want platform unavailable", ok, reason)
	}
}

// TestNotificationCapabilityStatusAndShow 验证 status / show 路径。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationCapabilityStatusAndShow(t *testing.T) {
	backend := &stubNotificationBackend{authorized: true}
	cap := NewNotificationCapability(nil, NotificationCapabilityOptions{Backend: backend})

	statusJSON, err := cap.Invoke(context.Background(), []byte(`{"action":"status"}`))
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	var status notificationStatusResult
	if err := json.Unmarshal(statusJSON, &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if !status.Authorized {
		t.Fatal("expected authorized")
	}

	showJSON, err := cap.Invoke(context.Background(), []byte(`{"action":"show","title":"T","body":"B","id":"1"}`))
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	var show notificationShowResult
	if err := json.Unmarshal(showJSON, &show); err != nil {
		t.Fatalf("unmarshal show: %v", err)
	}
	if !show.OK || len(backend.sent) != 1 {
		t.Fatalf("show result incomplete: %+v sent=%d", show, len(backend.sent))
	}
	if backend.sent[0].Title != "T" || backend.sent[0].Body != "B" {
		t.Fatalf("sent = %+v", backend.sent[0])
	}
}

// TestNotificationCapabilityPermissionDeniedOnShow 验证未授权 show 返回 DESKTOP_PERMISSION_DENIED。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationCapabilityPermissionDeniedOnShow(t *testing.T) {
	backend := &stubNotificationBackend{authorized: false}
	cap := NewNotificationCapability(nil, NotificationCapabilityOptions{Backend: backend})
	_, err := cap.Invoke(context.Background(), []byte(`{"action":"show","title":"x"}`))
	var capErr *CapabilityError
	if !errors.As(err, &capErr) || capErr.Code != ErrCodePermissionDenied {
		t.Fatalf("err = %v, want PERMISSION_DENIED", err)
	}
	if len(backend.sent) != 0 {
		t.Fatal("should not send when unauthorized")
	}
}

// TestNotificationCapabilityRequestPermission 验证显式请求授权。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationCapabilityRequestPermission(t *testing.T) {
	backend := &stubNotificationBackend{authorized: true}
	cap := NewNotificationCapability(nil, NotificationCapabilityOptions{Backend: backend})
	data, err := cap.Invoke(context.Background(), []byte(`{"action":"requestPermission"}`))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if backend.requests != 1 {
		t.Fatalf("requests = %d", backend.requests)
	}
	var result notificationShowResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Authorized {
		t.Fatal("expected authorized after request")
	}
}

// TestNotificationCapabilityAsDesktopSender 验证 Bind 到 notify sink。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationCapabilityAsDesktopSender(t *testing.T) {
	t.Cleanup(notify.UnbindDesktopSender)
	backend := &stubNotificationBackend{authorized: true}
	cap := NewNotificationCapability(nil, NotificationCapabilityOptions{Backend: backend})
	cap.bindAsDesktopSender()

	if err := cap.SendNotification(context.Background(), "title", "body", "id-9"); err != nil {
		t.Fatalf("SendNotification: %v", err)
	}
	if len(backend.sent) != 1 || backend.sent[0].Title != "title" {
		t.Fatalf("sent = %+v", backend.sent)
	}
}

// TestNotificationClickActivatesWindow 验证默认点击激活主窗口。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestNotificationClickActivatesWindow(t *testing.T) {
	activated := false
	cap := NewNotificationCapability(nil, NotificationCapabilityOptions{
		Backend: &stubNotificationBackend{authorized: true},
		ActivateMainWindow: func() {
			activated = true
		},
	})
	cap.HandleNotificationResponse(notifications.NotificationResult{
		Response: notifications.NotificationResponse{
			ActionIdentifier: notifications.DefaultActionIdentifier,
		},
	})
	if !activated {
		t.Fatal("expected activateMainWindow")
	}
}
