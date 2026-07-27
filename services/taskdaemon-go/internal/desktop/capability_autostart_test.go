package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type stubAutostartHost struct {
	available bool
	enabled   bool
	fail      error
}

func (s *stubAutostartHost) Available() (bool, UnavailableReason, string) {
	if !s.available {
		return false, ReasonPlatform, "no"
	}
	return true, "", ""
}
func (s *stubAutostartHost) Status() (AutostartStatusInfo, error) {
	if s.fail != nil {
		return AutostartStatusInfo{}, s.fail
	}
	return AutostartStatusInfo{Enabled: s.enabled, Note: AutostartVsServiceNote}, nil
}
func (s *stubAutostartHost) Enable() error {
	if s.fail != nil {
		return s.fail
	}
	s.enabled = true
	return nil
}
func (s *stubAutostartHost) Disable() error {
	if s.fail != nil {
		return s.fail
	}
	s.enabled = false
	return nil
}

// TestAutostartCapabilityToggle 验证 enable/disable/status。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestAutostartCapabilityToggle(t *testing.T) {
	host := &stubAutostartHost{available: true}
	cap := NewAutostartCapability(host)
	if cap.Name() != CapabilityAutostart {
		t.Fatalf("name = %s", cap.Name())
	}

	if _, err := cap.Invoke(context.Background(), []byte(`{"action":"enable"}`)); err != nil {
		t.Fatal(err)
	}
	data, err := cap.Invoke(context.Background(), []byte(`{"action":"status"}`))
	if err != nil {
		t.Fatal(err)
	}
	var info AutostartStatusInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	if !info.Enabled || info.Note == "" {
		t.Fatalf("info = %+v", info)
	}
	if _, err := cap.Invoke(context.Background(), []byte(`{"action":"disable"}`)); err != nil {
		t.Fatal(err)
	}
	if host.enabled {
		t.Fatal("expected disabled")
	}
}

// TestMapAutostartErrorCodes 验证权限与不支持映射。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestMapAutostartErrorCodes(t *testing.T) {
	err := mapAutostartError(application.ErrAutostartNotSupported)
	var capErr *CapabilityError
	if !asCapabilityError(err, &capErr) || capErr.Code != ErrCodeCapabilityUnavailable {
		t.Fatalf("unsupported map = %v", err)
	}
	err = mapAutostartError(errors.New("permission denied by user"))
	if !asCapabilityError(err, &capErr) || capErr.Code != ErrCodePermissionDenied {
		t.Fatalf("permission map = %v", err)
	}
}
