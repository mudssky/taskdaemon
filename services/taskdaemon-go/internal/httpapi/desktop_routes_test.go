package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubDesktopBridge struct {
	list []DesktopCapabilityDTO
	inv  DesktopInvokeResultDTO
}

func (s stubDesktopBridge) ListDesktopCapabilities() []DesktopCapabilityDTO {
	return s.list
}

func (s stubDesktopBridge) InvokeDesktopCapability(name string, payloadJSON string) DesktopInvokeResultDTO {
	_ = name
	_ = payloadJSON
	return s.inv
}

func TestDesktopCapabilitiesRequiresSession(t *testing.T) {
	t.Cleanup(func() { SetDesktopBridge(nil) })
	SetDesktopBridge(stubDesktopBridge{
		list: []DesktopCapabilityDTO{{Name: "desktop.environment", Available: true}},
	})
	router := NewRouter(Options{})
	req := httptest.NewRequest(http.MethodGet, "/api/desktop/capabilities", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDesktopCapabilitiesListsWhenAuthed(t *testing.T) {
	t.Cleanup(func() { SetDesktopBridge(nil) })
	SetDesktopBridge(stubDesktopBridge{
		list: []DesktopCapabilityDTO{{Name: "desktop.environment", Available: true}},
	})
	router := NewRouter(Options{Auth: loggedInAuthService()})
	req := authorizedRequest(http.MethodGet, "/api/desktop/capabilities", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var items []DesktopCapabilityDTO
	decodeAPIData(t, rec.Body.Bytes(), &items)
	require.Equal(t, []DesktopCapabilityDTO{{Name: "desktop.environment", Available: true}}, items)
}

func TestDesktopInvokeWhenAuthed(t *testing.T) {
	t.Cleanup(func() { SetDesktopBridge(nil) })
	SetDesktopBridge(stubDesktopBridge{
		inv: DesktopInvokeResultDTO{
			OK:   true,
			Data: json.RawMessage(`{"platform":"windows"}`),
		},
	})
	router := NewRouter(Options{Auth: loggedInAuthService()})
	body := bytes.NewBufferString(`{"name":"desktop.environment","payloadJSON":""}`)
	req := authorizedRequest(http.MethodPost, "/api/desktop/invoke", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var result DesktopInvokeResultDTO
	decodeAPIData(t, rec.Body.Bytes(), &result)
	require.True(t, result.OK)
	require.JSONEq(t, `{"platform":"windows"}`, string(result.Data))
}

func TestDesktopBridgeUnavailable(t *testing.T) {
	t.Cleanup(func() { SetDesktopBridge(nil) })
	SetDesktopBridge(nil)
	router := NewRouter(Options{Auth: loggedInAuthService()})
	req := authorizedRequest(http.MethodGet, "/api/desktop/capabilities", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
