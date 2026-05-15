package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/auth"
	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
	"taskdaemon/internal/runner"
	"taskdaemon/internal/scheduler"
)

// TestCreateTaskRequiresSession 验证未登录请求不能创建任务。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestCreateTaskRequiresSession(t *testing.T) {
	router := NewRouter(Options{Auth: fakeAuthService{authErr: auth.ErrInvalidSession}, Tasks: fakeTaskService{}})
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"error":{"code":"unauthorized","message":"Authentication required","details":null}}`, rec.Body.String())
}

// TestCreateTaskCallsTaskService 验证登录后创建任务会调用调度服务并返回任务摘要。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestCreateTaskCallsTaskService(t *testing.T) {
	service := &recordingTaskService{
		created: &ent.Task{ID: 9, Name: "backup", CronExpression: "30 9 * * *", Timezone: "Asia/Hong_Kong"},
	}
	router := NewRouter(Options{Auth: loggedInAuthService(), Tasks: service})
	body := bytes.NewBufferString(`{"name":"backup","enabled":false,"cronExpression":"30 9 * * *","timezone":"Asia/Hong_Kong","confirmCronWarnings":true,"runner":{"type":"shell","inline":"echo ok","timeoutSeconds":60}}`)
	req := authorizedRequest(http.MethodPost, "/api/tasks", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "backup", service.createInput.Name)
	require.NotNil(t, service.createInput.Enabled)
	require.False(t, *service.createInput.Enabled)
	require.Equal(t, runner.TypeShell, service.createInput.Runner.Type)
	require.Equal(t, time.Minute, service.createInput.Runner.Timeout)
	require.JSONEq(t, `{"id":9,"name":"backup","cronExpression":"30 9 * * *","timezone":"Asia/Hong_Kong"}`, rec.Body.String())
}

// TestTriggerTaskCallsTaskService 验证手动触发 API 会调用调度服务并返回执行历史。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestTriggerTaskCallsTaskService(t *testing.T) {
	exitCode := 0
	service := &recordingTaskService{
		triggered: &ent.Run{ID: 11, Status: entrun.StatusSuccess, ExitCode: &exitCode, Stdout: "ok"},
	}
	router := NewRouter(Options{Auth: loggedInAuthService(), Tasks: service})
	req := authorizedRequest(http.MethodPost, "/api/tasks/7/trigger", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 7, service.triggeredID)
	require.JSONEq(t, `{"id":11,"status":"success","exitCode":0,"stdout":"ok","stderr":"","errorSummary":"","durationMs":0}`, rec.Body.String())
}

// TestCancelTaskCallsTaskService 验证取消 API 会调用调度服务。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestCancelTaskCallsTaskService(t *testing.T) {
	service := &recordingTaskService{}
	router := NewRouter(Options{Auth: loggedInAuthService(), Tasks: service})
	req := authorizedRequest(http.MethodPost, "/api/tasks/7/cancel", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, 7, service.cancelledID)
}

// TestCancelTaskMapsNotRunningError 验证未运行任务的取消错误映射为稳定机器码。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestCancelTaskMapsNotRunningError(t *testing.T) {
	service := &recordingTaskService{cancelErr: scheduler.ErrTaskNotRunning}
	router := NewRouter(Options{Auth: loggedInAuthService(), Tasks: service})
	req := authorizedRequest(http.MethodPost, "/api/tasks/7/cancel", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.JSONEq(t, `{"error":{"code":"task_not_running","message":"Task is not running","details":null}}`, rec.Body.String())
}

// TestListTaskRunsCallsTaskService 验证执行历史 API 会查询指定任务历史。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestListTaskRunsCallsTaskService(t *testing.T) {
	exitCode := 0
	service := &recordingTaskService{
		runs: []*ent.Run{{ID: 11, Status: entrun.StatusSuccess, ExitCode: &exitCode, Stdout: "ok"}},
	}
	router := NewRouter(Options{Auth: loggedInAuthService(), Tasks: service})
	req := authorizedRequest(http.MethodGet, "/api/tasks/7/runs", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 7, service.runsTaskID)
	require.JSONEq(t, `{"runs":[{"id":11,"status":"success","exitCode":0,"stdout":"ok","stderr":"","errorSummary":"","durationMs":0}]}`, rec.Body.String())
}

// loggedInAuthService 返回测试用已登录认证服务。
//
// 参数:
//   - 无。
//
// 返回值:
//   - fakeAuthService: 会认证通过的认证服务替身。
func loggedInAuthService() fakeAuthService {
	return fakeAuthService{
		login: auth.LoginResult{Principal: auth.Principal{AdminID: 1, Username: "admin"}},
	}
}

// authorizedRequest 创建带 session cookie 的 HTTP 请求。
//
// 参数:
//   - method: HTTP 方法。
//   - target: 请求路径。
//   - body: 请求体。
//
// 返回值:
//   - *http.Request: 带测试 session cookie 的请求。
func authorizedRequest(method string, target string, body *bytes.Buffer) *http.Request {
	var reader *bytes.Buffer
	if body == nil {
		reader = &bytes.Buffer{}
	} else {
		reader = body
	}
	req := httptest.NewRequest(method, target, reader)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	return req
}

// recordingTaskService 是任务 API 测试使用的调度服务替身。
type recordingTaskService struct {
	createInput scheduler.CreateTaskInput
	created     *ent.Task
	triggeredID int
	triggered   *ent.Run
	cancelledID int
	cancelErr   error
	runsTaskID  int
	runs        []*ent.Run
}

// CreateTask 记录创建输入并返回预设任务。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - input: 创建任务输入。
//
// 返回值:
//   - *ent.Task: 预设任务。
//   - error: 当前测试替身不返回错误。
func (service *recordingTaskService) CreateTask(_ context.Context, input scheduler.CreateTaskInput) (*ent.Task, error) {
	service.createInput = input
	return service.created, nil
}

// TriggerTask 记录触发任务 ID 并返回预设执行历史。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - taskID: 任务 ID。
//
// 返回值:
//   - *ent.Run: 预设执行历史。
//   - error: 当前测试替身不返回错误。
func (service *recordingTaskService) TriggerTask(_ context.Context, taskID int) (*ent.Run, error) {
	service.triggeredID = taskID
	return service.triggered, nil
}

// CancelTask 记录取消任务 ID。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - taskID: 任务 ID。
//
// 返回值:
//   - error: 当前测试替身不返回错误。
func (service *recordingTaskService) CancelTask(_ context.Context, taskID int) error {
	service.cancelledID = taskID
	return service.cancelErr
}

// ListTaskRuns 记录查询任务 ID 并返回预设执行历史列表。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - taskID: 任务 ID。
//   - _ : 返回条数限制，测试替身不使用。
//
// 返回值:
//   - []*ent.Run: 预设执行历史列表。
//   - error: 当前测试替身不返回错误。
func (service *recordingTaskService) ListTaskRuns(_ context.Context, taskID int, _ int) ([]*ent.Run, error) {
	service.runsTaskID = taskID
	return service.runs, nil
}

// fakeTaskService 是空任务服务替身。
type fakeTaskService struct{}

// CreateTask 返回空任务。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 创建任务输入，测试替身不使用。
//
// 返回值:
//   - *ent.Task: 空任务。
//   - error: 当前测试替身不返回错误。
func (fakeTaskService) CreateTask(context.Context, scheduler.CreateTaskInput) (*ent.Task, error) {
	return &ent.Task{}, nil
}

// TriggerTask 返回空执行历史。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 任务 ID，测试替身不使用。
//
// 返回值:
//   - *ent.Run: 空执行历史。
//   - error: 当前测试替身不返回错误。
func (fakeTaskService) TriggerTask(context.Context, int) (*ent.Run, error) {
	return &ent.Run{}, nil
}

// CancelTask 模拟取消成功。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 任务 ID，测试替身不使用。
//
// 返回值:
//   - error: 当前测试替身不返回错误。
func (fakeTaskService) CancelTask(context.Context, int) error {
	return nil
}

// ListTaskRuns 返回空执行历史列表。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 任务 ID，测试替身不使用。
//   - _ : 返回条数限制，测试替身不使用。
//
// 返回值:
//   - []*ent.Run: 空列表。
//   - error: 当前测试替身不返回错误。
func (fakeTaskService) ListTaskRuns(context.Context, int, int) ([]*ent.Run, error) {
	return nil, nil
}
