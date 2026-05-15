package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/runner"
	"taskdaemon/internal/scheduler"
)

// registerTaskRoutes 注册任务创建、触发、取消和历史路由。
//
// 参数:
//   - router: Gin engine。
//   - authService: 认证服务接口。
//   - taskService: 任务调度服务接口。
//
// 返回值:
//   - 无。
func registerTaskRoutes(router *gin.Engine, authService AuthService, taskService TaskService) {
	group := router.Group("/api/tasks")
	group.Use(requireSession(authService))

	group.GET("", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		tasks, err := taskService.ListTasks(ctx.Request.Context(), 100)
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "task_list_failed", "Task list query failed", nil)
			return
		}
		responses := make([]taskResponse, 0, len(tasks))
		for _, taskRecord := range tasks {
			responses = append(responses, taskResponseFromEnt(taskRecord, taskService.IsTaskRunning(taskRecord.ID)))
		}
		writeAPIOK(ctx, gin.H{"tasks": responses})
	})

	group.POST("", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		var req createTaskRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid request body", gin.H{"field": "body"})
			return
		}
		taskRecord, err := taskService.CreateTask(ctx.Request.Context(), taskInputFromRequest(req))
		if err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "task_invalid", "Task definition is invalid", gin.H{"error": err.Error()})
			return
		}
		writeAPISuccess(ctx, http.StatusCreated, taskResponseFromEnt(taskRecord, taskService.IsTaskRunning(taskRecord.ID)))
	})

	group.PUT("/:id", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		taskID, ok := parseTaskID(ctx)
		if !ok {
			return
		}
		var req createTaskRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid request body", gin.H{"field": "body"})
			return
		}
		taskRecord, err := taskService.UpdateTask(ctx.Request.Context(), taskID, taskInputFromRequest(req))
		if err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "task_invalid", "Task definition is invalid", gin.H{"error": err.Error()})
			return
		}
		writeAPIOK(ctx, taskResponseFromEnt(taskRecord, taskService.IsTaskRunning(taskRecord.ID)))
	})

	group.PATCH("/:id/enabled", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		taskID, ok := parseTaskID(ctx)
		if !ok {
			return
		}
		var req setTaskEnabledRequest
		if err := ctx.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid request body", gin.H{"field": "enabled"})
			return
		}
		taskRecord, err := taskService.SetTaskEnabled(ctx.Request.Context(), taskID, *req.Enabled)
		if err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "task_invalid", "Task enabled state is invalid", gin.H{"error": err.Error()})
			return
		}
		writeAPIOK(ctx, taskResponseFromEnt(taskRecord, taskService.IsTaskRunning(taskRecord.ID)))
	})

	group.POST("/:id/trigger", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		taskID, ok := parseTaskID(ctx)
		if !ok {
			return
		}
		runRecord, err := taskService.TriggerTask(ctx.Request.Context(), taskID)
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "task_trigger_failed", "Task trigger failed", nil)
			return
		}
		writeAPIOK(ctx, runResponseFromEnt(runRecord))
	})

	group.POST("/:id/cancel", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		taskID, ok := parseTaskID(ctx)
		if !ok {
			return
		}
		if err := taskService.CancelTask(ctx.Request.Context(), taskID); err != nil {
			if errors.Is(err, scheduler.ErrTaskNotRunning) {
				writeAPIError(ctx, http.StatusConflict, "task_not_running", "Task is not running", nil)
				return
			}
			writeAPIError(ctx, http.StatusConflict, "task_cancel_failed", "Task cancel failed", nil)
			return
		}
		writeAPIOK(ctx, nil)
	})

	group.GET("/:id/runs", func(ctx *gin.Context) {
		if taskService == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		taskID, ok := parseTaskID(ctx)
		if !ok {
			return
		}
		runs, err := taskService.ListTaskRuns(ctx.Request.Context(), taskID, 50)
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "task_runs_failed", "Task runs query failed", nil)
			return
		}
		responses := make([]runResponse, 0, len(runs))
		for _, runRecord := range runs {
			responses = append(responses, runResponseFromEnt(runRecord))
		}
		writeAPIOK(ctx, gin.H{"runs": responses})
	})
}

// taskInputFromRequest 将 HTTP 创建/更新请求转换为调度服务输入。
//
// 参数:
//   - req: HTTP 创建/更新任务请求。
//
// 返回值:
//   - scheduler.CreateTaskInput: 调度服务输入。
func taskInputFromRequest(req createTaskRequest) scheduler.CreateTaskInput {
	return scheduler.CreateTaskInput{
		Name:                req.Name,
		Description:         req.Description,
		Enabled:             req.Enabled,
		CronExpression:      req.CronExpression,
		Timezone:            req.Timezone,
		ConfirmCronWarnings: req.ConfirmCronWarnings,
		Runner: runner.Config{
			Type:             runner.Type(req.Runner.Type),
			Inline:           req.Runner.Inline,
			ScriptPath:       req.Runner.ScriptPath,
			Args:             req.Runner.Args,
			WorkDir:          req.Runner.WorkDir,
			Env:              req.Runner.Env,
			Timeout:          time.Duration(req.Runner.TimeoutSeconds) * time.Second,
			OutputLimitBytes: req.Runner.OutputLimitBytes,
		},
	}
}

// parseTaskID 解析路由中的任务 ID。
//
// 参数:
//   - ctx: Gin 请求上下文。
//
// 返回值:
//   - int: 任务 ID。
//   - bool: true 表示解析成功。
func parseTaskID(ctx *gin.Context) (int, bool) {
	taskID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || taskID <= 0 {
		writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid task id", gin.H{"field": "id"})
		return 0, false
	}
	return taskID, true
}
