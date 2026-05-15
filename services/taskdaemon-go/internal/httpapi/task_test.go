package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
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

// TestListTasksCallsTaskService 验证登录后任务列表 API 会返回任务摘要和运行态。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestListTasksCallsTaskService(t *testing.T) {
	service := &recordingTaskService{
		tasks: []*ent.Task{
			{
				ID:             9,
				Name:           "backup",
				Enabled:        true,
				CronExpression: "30 9 * * *",
				Timezone:       "Asia/Hong_Kong",
				RunnerConfig:   map[string]any{"inline": "echo ok"},
			},
		},
		runningIDs: map[int]bool{9: true},
	}
	router := NewRouter(Options{Auth: loggedInAuthService(), Tasks: service})
	req := authorizedRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 100, service.listLimit)
	var response struct {
		Tasks []taskResponse `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Len(t, response.Tasks, 1)
	require.Equal(t, 9, response.Tasks[0].ID)
	require.Equal(t, "backup", response.Tasks[0].Name)
	require.True(t, response.Tasks[0].Enabled)
	require.True(t, response.Tasks[0].Running)
	require.Equal(t, "echo ok", response.Tasks[0].RunnerConfig["inline"])
}

// TestCreateTaskRequiresSession 验证未登录请求不能创建任务。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestCreateTaskRequiresSession(t *testing.T) {
	router := NewRouter(Options{Auth: &fakeAuthService{authErr: auth.ErrInvalidSession}, Tasks: fakeTaskService{}})
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
	var response taskResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, 9, response.ID)
	require.Equal(t, "backup", response.Name)
	require.Equal(t, "30 9 * * *", response.CronExpression)
	require.Equal(t, "Asia/Hong_Kong", response.Timezone)
}

// TestUpdateTaskCallsTaskService 验证编辑任务 API 会传递完整任务定义。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestUpdateTaskCallsTaskService(t *testing.T) {
	service := &recordingTaskService{
		updated: &ent.Task{ID: 9, Name: "backup-new", CronExpression: "*/5 * * * *", Timezone: "Local"},
	}
	router := NewRouter(Options{Auth: loggedInAuthService(), Tasks: service})
	body := bytes.NewBufferString(`{"name":"backup-new","enabled":true,"cronExpression":"*/5 * * * *","timezone":"Local","runner":{"type":"python","scriptPath":"jobs/backup.py","timeoutSeconds":120}}`)
	req := authorizedRequest(http.MethodPut, "/api/tasks/9", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 9, service.updatedID)
	require.Equal(t, "backup-new", service.updateInput.Name)
	require.Equal(t, runner.TypePython, service.updateInput.Runner.Type)
	require.Equal(t, "jobs/backup.py", service.updateInput.Runner.ScriptPath)
	var response taskResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "backup-new", response.Name)
}

// TestSetTaskEnabledCallsTaskService 验证启停任务 API 会调用调度服务。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 require 终止。
func TestSetTaskEnabledCallsTaskService(t *testing.T) {
	service := &recordingTaskService{
		enabledTask: &ent.Task{ID: 9, Name: "backup", Enabled: false},
	}
	router := NewRouter(Options{Auth: loggedInAuthService(), Tasks: service})
	body := bytes.NewBufferString(`{"enabled":false}`)
	req := authorizedRequest(http.MethodPatch, "/api/tasks/9/enabled", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 9, service.enabledID)
	require.False(t, service.enabledValue)
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
	var response runResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, 11, response.ID)
	require.Equal(t, entrun.StatusSuccess, response.Status)
	require.NotNil(t, response.ExitCode)
	require.Equal(t, 0, *response.ExitCode)
	require.Equal(t, "ok", response.Stdout)
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
	var response struct {
		Runs []runResponse `json:"runs"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Len(t, response.Runs, 1)
	require.Equal(t, 11, response.Runs[0].ID)
	require.Equal(t, entrun.StatusSuccess, response.Runs[0].Status)
}

// loggedInAuthService 返回测试用已登录认证服务。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *fakeAuthService: 会认证通过的认证服务替身。
func loggedInAuthService() *fakeAuthService {
	return &fakeAuthService{
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
	createInput  scheduler.CreateTaskInput
	created      *ent.Task
	listLimit    int
	tasks        []*ent.Task
	updatedID    int
	updateInput  scheduler.CreateTaskInput
	updated      *ent.Task
	enabledID    int
	enabledValue bool
	enabledTask  *ent.Task
	triggeredID  int
	triggered    *ent.Run
	cancelledID  int
	cancelErr    error
	runsTaskID   int
	runs         []*ent.Run
	runningIDs   map[int]bool
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

// ListTasks 记录列表限制并返回预设任务列表。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - limit: 返回条数限制。
//
// 返回值:
//   - []*ent.Task: 预设任务列表。
//   - error: 当前测试替身不返回错误。
func (service *recordingTaskService) ListTasks(_ context.Context, limit int) ([]*ent.Task, error) {
	service.listLimit = limit
	return service.tasks, nil
}

// UpdateTask 记录更新输入并返回预设任务。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - taskID: 任务 ID。
//   - input: 更新任务输入。
//
// 返回值:
//   - *ent.Task: 预设任务。
//   - error: 当前测试替身不返回错误。
func (service *recordingTaskService) UpdateTask(_ context.Context, taskID int, input scheduler.CreateTaskInput) (*ent.Task, error) {
	service.updatedID = taskID
	service.updateInput = input
	return service.updated, nil
}

// SetTaskEnabled 记录启停输入并返回预设任务。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - taskID: 任务 ID。
//   - enabled: 目标启停状态。
//
// 返回值:
//   - *ent.Task: 预设任务。
//   - error: 当前测试替身不返回错误。
func (service *recordingTaskService) SetTaskEnabled(_ context.Context, taskID int, enabled bool) (*ent.Task, error) {
	service.enabledID = taskID
	service.enabledValue = enabled
	return service.enabledTask, nil
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

// IsTaskRunning 返回预设运行态。
//
// 参数:
//   - taskID: 任务 ID。
//
// 返回值:
//   - bool: true 表示任务正在运行。
func (service *recordingTaskService) IsTaskRunning(taskID int) bool {
	return service.runningIDs[taskID]
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

// ListTasks 返回空任务列表。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 返回条数限制，测试替身不使用。
//
// 返回值:
//   - []*ent.Task: 空列表。
//   - error: 当前测试替身不返回错误。
func (fakeTaskService) ListTasks(context.Context, int) ([]*ent.Task, error) {
	return nil, nil
}

// UpdateTask 返回空任务。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 任务 ID，测试替身不使用。
//   - _ : 更新任务输入，测试替身不使用。
//
// 返回值:
//   - *ent.Task: 空任务。
//   - error: 当前测试替身不返回错误。
func (fakeTaskService) UpdateTask(context.Context, int, scheduler.CreateTaskInput) (*ent.Task, error) {
	return &ent.Task{}, nil
}

// SetTaskEnabled 返回空任务。
//
// 参数:
//   - _ : 请求 context，测试替身不使用。
//   - _ : 任务 ID，测试替身不使用。
//   - _ : 启停状态，测试替身不使用。
//
// 返回值:
//   - *ent.Task: 空任务。
//   - error: 当前测试替身不返回错误。
func (fakeTaskService) SetTaskEnabled(context.Context, int, bool) (*ent.Task, error) {
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

// IsTaskRunning 总是返回 false。
//
// 参数:
//   - _ : 任务 ID，测试替身不使用。
//
// 返回值:
//   - bool: false。
func (fakeTaskService) IsTaskRunning(int) bool {
	return false
}
