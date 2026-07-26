package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/template"
)

// templateServiceForTest 返回默认内置模板服务。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *template.Service: 测试用服务。
func templateServiceForTest() *template.Service {
	return &template.Service{Registry: template.DefaultRegistry()}
}

// TestListTemplatesRequiresSession 验证未登录不能列模板。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestListTemplatesRequiresSession(t *testing.T) {
	router := NewRouter(Options{
		Auth:      &fakeAuthService{authErr: auth.ErrInvalidSession},
		Templates: templateServiceForTest(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "unauthorized", envelope.Error.Code)
}

// TestListTemplatesReturnsParamDefs 验证列表含完整参数定义。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestListTemplatesReturnsParamDefs(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), Templates: templateServiceForTest()})
	req := authorizedRequest(http.MethodGet, "/api/templates", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Templates []templateDefinitionResponse `json:"templates"`
	}
	decodeAPIData(t, rec.Body.Bytes(), &body)
	require.Len(t, body.Templates, 3)
	var pg *templateDefinitionResponse
	for i := range body.Templates {
		if body.Templates[i].ID == "postgres-pg-dump" {
			pg = &body.Templates[i]
		}
	}
	require.NotNil(t, pg)
	require.NotEmpty(t, pg.Params)
	require.Equal(t, "shell", pg.RunnerType)
	// 含 secret_ref 参数
	foundSecret := false
	for _, p := range pg.Params {
		if p.Name == "passwordEnv" {
			foundSecret = true
			require.Equal(t, template.ParamSecretRef, p.Type)
			require.True(t, p.Sensitive)
		}
	}
	require.True(t, foundSecret)
}

// TestGetTemplateNotFound 验证未知 id。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestGetTemplateNotFound(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), Templates: templateServiceForTest()})
	req := authorizedRequest(http.MethodGet, "/api/templates/no-such", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, template.CodeNotFound, envelope.Error.Code)
}

// TestGetTemplateDetail 验证详情 API。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestGetTemplateDetail(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), Templates: templateServiceForTest()})
	req := authorizedRequest(http.MethodGet, "/api/templates/sqlite-file-backup", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var body templateDefinitionResponse
	decodeAPIData(t, rec.Body.Bytes(), &body)
	require.Equal(t, "sqlite-file-backup", body.ID)
	require.NotEmpty(t, body.Params)
}

// TestRenderTemplateSuccess 验证渲染成功草稿形状。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRenderTemplateSuccess(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), Templates: templateServiceForTest()})
	payload := map[string]any{
		"params": map[string]any{
			"name":           "pg-nightly",
			"host":           "127.0.0.1",
			"user":           "postgres",
			"database":       "app",
			"outputPath":     "/backups/app.dump",
			"cronExpression": "30 2 * * *",
			"timezone":       "Asia/Shanghai",
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	req := authorizedRequest(http.MethodPost, "/api/templates/postgres-pg-dump/render", bytes.NewBuffer(raw))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var draft template.TaskDraft
	decodeAPIData(t, rec.Body.Bytes(), &draft)
	require.Equal(t, "pg-nightly", draft.Name)
	require.Equal(t, "shell", draft.Runner.Type)
	require.Contains(t, draft.Runner.Inline, "pg_dump")
	require.NotNil(t, draft.Enabled)
	require.False(t, *draft.Enabled)
	require.NotEmpty(t, draft.CommandPreview)
	require.Equal(t, "postgres-pg-dump", draft.TemplateID)
	// 无密码明文
	require.NotContains(t, draft.Runner.Inline, "super-secret")
	require.Empty(t, draft.Runner.Env)
}

// TestRenderTemplateFieldErrors 验证字段级错误与 C-1 同形。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRenderTemplateFieldErrors(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), Templates: templateServiceForTest()})
	body := bytes.NewBufferString(`{"params":{"host":"h"}}`)
	req := authorizedRequest(http.MethodPost, "/api/templates/postgres-pg-dump/render", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Contains(t, []string{
		template.CodeFieldRequired,
		template.CodeValidationFailed,
		template.CodeFieldInvalid,
	}, envelope.Error.Code)
	detailsJSON, err := json.Marshal(envelope.Error.Details)
	require.NoError(t, err)
	var details map[string]any
	require.NoError(t, json.Unmarshal(detailsJSON, &details))
	fieldsRaw, ok := details["fields"]
	require.True(t, ok)
	fieldsJSON, err := json.Marshal(fieldsRaw)
	require.NoError(t, err)
	var fields []template.FieldError
	require.NoError(t, json.Unmarshal(fieldsJSON, &fields))
	require.NotEmpty(t, fields)
	require.NotEmpty(t, fields[0].Path)
	require.NotEmpty(t, fields[0].Code)
}

// TestRenderTemplateInjectionPayload 验证 HTTP 路径下注入仍被 quote。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRenderTemplateInjectionPayload(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), Templates: templateServiceForTest()})
	payload := map[string]any{
		"params": map[string]any{
			"name":           "t",
			"host":           "; dropdb evil",
			"user":           "u",
			"database":       "d",
			"outputPath":     "/tmp/o.dump",
			"cronExpression": "0 1 * * *",
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	req := authorizedRequest(http.MethodPost, "/api/templates/postgres-pg-dump/render", bytes.NewBuffer(raw))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var draft template.TaskDraft
	decodeAPIData(t, rec.Body.Bytes(), &draft)
	require.Contains(t, draft.Runner.Inline, "-h '; dropdb evil'")
	require.NotContains(t, draft.Runner.Inline, "-h ; dropdb")
}

// TestRenderTemplateUnavailable 验证未装配服务。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRenderTemplateUnavailable(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), Templates: nil})
	req := authorizedRequest(http.MethodPost, "/api/templates/postgres-pg-dump/render", bytes.NewBufferString(`{"params":{}}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, template.CodeUnavailable, envelope.Error.Code)
}

// TestRenderInvalidJSON 验证非法 body。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRenderInvalidJSON(t *testing.T) {
	router := NewRouter(Options{Auth: loggedInAuthService(), Templates: templateServiceForTest()})
	req := authorizedRequest(http.MethodPost, "/api/templates/postgres-pg-dump/render", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	envelope := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, template.CodeInvalidJSON, envelope.Error.Code)
}
