package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/auth"
)

// TestAuthMeRequiresSession 验证受保护 API 在未登录时返回稳定 401 错误。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestAuthMeRequiresSession(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{authErr: auth.ErrInvalidSession}})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"error":{"code":"unauthorized","message":"Authentication required","details":null}}`, rec.Body.String())
}

// TestLoginSetsSessionCookie 验证登录成功后 API 写入 HttpOnly session cookie。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestLoginSetsSessionCookie(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{
		login: auth.LoginResult{
			Principal:    auth.Principal{AdminID: 7, Username: "admin"},
			SessionToken: "session-token",
			CSRFToken:    "csrf-token",
		},
	}})
	body := bytes.NewBufferString(`{"username":"admin","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, SessionCookieName, cookies[0].Name)
	require.Equal(t, "session-token", cookies[0].Value)
	require.True(t, cookies[0].HttpOnly)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "csrf-token", response["csrfToken"])
}

// TestInitializeAdminCreatesSession 验证初始化接口会创建首个管理员账号并直接写入登录态。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestInitializeAdminCreatesSession(t *testing.T) {
	service := &fakeAuthService{
		initLogin: auth.LoginResult{
			Principal:    auth.Principal{AdminID: 7, Username: "admin"},
			SessionToken: "session-token",
			CSRFToken:    "csrf-token",
		},
	}
	router := NewRouter(Options{Auth: service})
	body := bytes.NewBufferString(`{"username":" admin ","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/init", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "admin", service.initUsername)
	require.Equal(t, "secret", service.initPassword)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, SessionCookieName, cookies[0].Name)
	require.Equal(t, "session-token", cookies[0].Value)
	require.True(t, cookies[0].HttpOnly)
	require.JSONEq(t, `{"adminId":7,"username":"admin","csrfToken":"csrf-token"}`, rec.Body.String())
}

// TestInitializeAdminReportsMissingCredentialField 验证初始化接口会返回具体缺失字段，避免把字段错误误报成 body 错误。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestInitializeAdminReportsMissingCredentialField(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{}})
	body := bytes.NewBufferString(`{"username":"admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/init", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":{"code":"bad_request","message":"Password is required","details":{"field":"password"}}}`, rec.Body.String())
}

// TestInitializeAdminReportsMalformedBody 验证初始化接口仍会把 JSON 解析失败标记为 body 错误。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestInitializeAdminReportsMalformedBody(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{}})
	body := bytes.NewBufferString(`{"username":`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/init", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":{"code":"bad_request","message":"Invalid request body","details":{"field":"body"}}}`, rec.Body.String())
}

// TestLoginReportsMissingCredentialField 验证登录接口复用认证字段校验并返回具体字段。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestLoginReportsMissingCredentialField(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{}})
	body := bytes.NewBufferString(`{"username":" "}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":{"code":"bad_request","message":"Username is required","details":{"field":"username"}}}`, rec.Body.String())
}

// TestInitializeAdminRejectsRepeatedSetup 验证初始化接口会拒绝重复创建管理员。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestInitializeAdminRejectsRepeatedSetup(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{
		initErr: auth.ErrAdminAlreadyInitialized,
	}})
	body := bytes.NewBufferString(`{"username":"admin","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/init", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.JSONEq(t, `{"error":{"code":"admin_already_initialized","message":"Admin is already initialized","details":null}}`, rec.Body.String())
}

// TestAuthStatusReportsUninitialized 验证状态接口会暴露首次启动尚未初始化的状态。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestAuthStatusReportsUninitialized(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{initializedStatus: false}})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"initialized":false,"authenticated":false}`, rec.Body.String())
}

// TestAuthStatusReportsInitializedAnonymous 验证状态接口在已初始化但未登录时不返回管理员信息。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestAuthStatusReportsInitializedAnonymous(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{
		initializedStatus: true,
		authErr:           auth.ErrInvalidSession,
	}})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"initialized":true,"authenticated":false}`, rec.Body.String())
}

// TestAuthStatusReportsAuthenticated 验证状态接口在已有有效 session 时返回当前管理员。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestAuthStatusReportsAuthenticated(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{
		initializedStatus: true,
		login: auth.LoginResult{
			Principal: auth.Principal{AdminID: 7, Username: "admin"},
		},
	}})
	req := authorizedRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"initialized":true,"authenticated":true,"admin":{"adminId":7,"username":"admin"}}`, rec.Body.String())
}

// TestLogoutClearsSessionCookie 验证退出登录会删除服务端 session 并清理浏览器 Cookie。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestLogoutClearsSessionCookie(t *testing.T) {
	logoutToken := ""
	service := &fakeAuthService{logoutToken: &logoutToken}
	router := NewRouter(Options{Auth: service})
	req := authorizedRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "session-token", logoutToken)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, SessionCookieName, cookies[0].Name)
	require.Equal(t, -1, cookies[0].MaxAge)
}

// fakeAuthService 是 HTTP API 测试使用的认证服务替身。
type fakeAuthService struct {
	login             auth.LoginResult
	initLogin         auth.LoginResult
	initUsername      string
	initPassword      string
	initializedStatus bool
	authErr           error
	initErr           error
	statusErr         error
	logoutToken       *string
	logoutErr         error
}

// InitializeAdminWithSession 返回预设初始化登录结果或错误。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 用户名，测试替身不使用。
//   - _ : 密码，测试替身不使用。
//   - _ : 登录元数据，测试替身不使用。
//
// 返回值:
//   - auth.LoginResult: 预设初始化后的登录结果。
//   - error: 预设错误。
func (fake *fakeAuthService) InitializeAdminWithSession(_ context.Context, username, password string, _ auth.LoginMetadata) (auth.LoginResult, error) {
	fake.initUsername = username
	fake.initPassword = password
	if fake.initErr != nil {
		return auth.LoginResult{}, fake.initErr
	}
	return fake.initLogin, nil
}

// Login 返回预设登录结果或错误。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 用户名，测试替身不使用。
//   - _ : 密码，测试替身不使用。
//   - _ : 登录元数据，测试替身不使用。
//
// 返回值:
//   - auth.LoginResult: 预设登录结果。
//   - error: 预设错误。
func (fake fakeAuthService) Login(_ context.Context, _, _ string, _ auth.LoginMetadata) (auth.LoginResult, error) {
	return fake.login, nil
}

// AuthenticateSession 返回预设认证结果或错误。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : session token，测试替身不使用。
//
// 返回值:
//   - auth.Principal: 预设管理员主体。
//   - error: 预设错误。
func (fake fakeAuthService) AuthenticateSession(_ context.Context, _ string) (auth.Principal, error) {
	if fake.authErr != nil {
		return auth.Principal{}, fake.authErr
	}
	if fake.login.Principal.AdminID == 0 {
		return auth.Principal{}, errors.New("missing fake principal")
	}
	return fake.login.Principal, nil
}

// IsAdminInitialized 返回预设初始化状态。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//
// 返回值:
//   - bool: 是否已经创建管理员。
//   - error: 预设错误。
func (fake fakeAuthService) IsAdminInitialized(_ context.Context) (bool, error) {
	if fake.statusErr != nil {
		return false, fake.statusErr
	}
	return fake.initializedStatus, nil
}

// Logout 记录退出登录时收到的 session token。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - token: session token。
//
// 返回值:
//   - error: 预设错误。
func (fake fakeAuthService) Logout(_ context.Context, token string) error {
	if fake.logoutToken != nil {
		*fake.logoutToken = token
	}
	return fake.logoutErr
}
