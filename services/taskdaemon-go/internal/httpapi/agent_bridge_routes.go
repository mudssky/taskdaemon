package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"taskdaemon/internal/agentbridge"
	"taskdaemon/internal/config"
	"taskdaemon/internal/data/ent"
)

// AgentBridgeOptions 装配 G6 入站桥接。
type AgentBridgeOptions struct {
	// Config 返回当前桥接配置。
	Config func() config.AgentBridgeConfig
	// Tasks 任务调度能力。
	Tasks TaskService
	// Audio 既有入站音频（机器 Bearer）；可选，仅用于文档兼容路径。
	Audio AudioService
}

// registerAgentBridgeRoutes 注册 agent→taskdaemon 入站路由（机器 Bearer，非管理员 cookie）。
//
// 参数:
//   - router: Gin engine。
//   - opts: 桥接依赖。
//
// 返回值:
//   - 无。
func registerAgentBridgeRoutes(router *gin.Engine, opts AgentBridgeOptions) {
	if opts.Config == nil {
		return
	}
	group := router.Group("/api/inbound/agent")

	group.POST("/tasks/:id/trigger", func(ctx *gin.Context) {
		cfg := opts.Config()
		if err := agentbridge.AuthenticateInbound(cfg, bearerToken(ctx)); err != nil {
			writeAgentBridgeAuthError(ctx, err)
			return
		}
		depth := agentbridge.ParseLoopDepth(ctx.GetHeader(agentbridge.HeaderLoopDepth))
		if err := agentbridge.AssertLoopDepth(cfg, depth); err != nil {
			writeAPIError(ctx, http.StatusConflict, "MCP_LOOP_DETECTED", "Agent bridge loop depth exceeded", gin.H{
				"depth": depth,
				"max":   cfg.MaxLoopDepth,
			})
			return
		}
		taskID, ok := parsePositiveID(ctx, "id")
		if !ok {
			return
		}
		if err := agentbridge.AssertTaskAllowed(cfg, taskID); err != nil {
			writeAPIError(ctx, http.StatusForbidden, "MCP_TASK_NOT_ALLOWED", "Task is not allowed for agent trigger", gin.H{"taskId": taskID})
			return
		}
		if opts.Tasks == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		runCtx := agentbridge.WithLoopDepth(ctx.Request.Context(), depth+1)
		runRecord, err := opts.Tasks.TriggerTask(runCtx, taskID)
		if err != nil {
			if ent.IsNotFound(err) {
				writeAPIError(ctx, http.StatusNotFound, "task_not_found", "Task not found", nil)
				return
			}
			writeAPIError(ctx, http.StatusInternalServerError, "task_trigger_failed", "Task trigger failed", nil)
			return
		}
		writeAPISuccess(ctx, http.StatusOK, agentTaskRunResponseFromEnt(runRecord, taskID))
	})

	group.GET("/tasks/:id/runs", func(ctx *gin.Context) {
		cfg := opts.Config()
		if err := agentbridge.AuthenticateInbound(cfg, bearerToken(ctx)); err != nil {
			writeAgentBridgeAuthError(ctx, err)
			return
		}
		taskID, ok := parsePositiveID(ctx, "id")
		if !ok {
			return
		}
		if err := agentbridge.AssertTaskAllowed(cfg, taskID); err != nil {
			writeAPIError(ctx, http.StatusForbidden, "MCP_TASK_NOT_ALLOWED", "Task is not allowed for agent query", gin.H{"taskId": taskID})
			return
		}
		if opts.Tasks == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		runs, err := opts.Tasks.ListTaskRuns(ctx.Request.Context(), taskID, 20)
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "task_runs_failed", "Task runs query failed", nil)
			return
		}
		items := make([]agentTaskRunResponse, 0, len(runs))
		for _, runRecord := range runs {
			items = append(items, agentTaskRunResponseFromEnt(runRecord, taskID))
		}
		writeAPIOK(ctx, gin.H{"runs": items})
	})

	group.GET("/tasks/:id", func(ctx *gin.Context) {
		cfg := opts.Config()
		if err := agentbridge.AuthenticateInbound(cfg, bearerToken(ctx)); err != nil {
			writeAgentBridgeAuthError(ctx, err)
			return
		}
		taskID, ok := parsePositiveID(ctx, "id")
		if !ok {
			return
		}
		if err := agentbridge.AssertTaskAllowed(cfg, taskID); err != nil {
			writeAPIError(ctx, http.StatusForbidden, "MCP_TASK_NOT_ALLOWED", "Task is not allowed for agent query", gin.H{"taskId": taskID})
			return
		}
		if opts.Tasks == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "task_service_unavailable", "Task service is unavailable", nil)
			return
		}
		tasks, err := opts.Tasks.ListTasks(ctx.Request.Context(), 500)
		if err != nil {
			writeAPIError(ctx, http.StatusInternalServerError, "task_list_failed", "Task list query failed", nil)
			return
		}
		for _, taskRecord := range tasks {
			if taskRecord.ID == taskID {
				writeAPIOK(ctx, agentTaskResponse{
					ID:      taskRecord.ID,
					Name:    taskRecord.Name,
					Enabled: taskRecord.Enabled,
					Running: opts.Tasks.IsTaskRunning(taskID),
				})
				return
			}
		}
		writeAPIError(ctx, http.StatusNotFound, "task_not_found", "Task not found", nil)
	})

	// 既有入站音频：agent 应使用 /api/inbound/audio-play-requests + audio Bearer
	if opts.Audio != nil {
		group.POST("/audio-play-requests", func(ctx *gin.Context) {
			cfg := opts.Config()
			if err := agentbridge.AuthenticateInbound(cfg, bearerToken(ctx)); err != nil {
				writeAgentBridgeAuthError(ctx, err)
				return
			}
			writeAPIError(ctx, http.StatusNotImplemented, "use_audio_inbound", "Use /api/inbound/audio-play-requests with audio inbound Bearer token", nil)
		})
	}
}

// writeAgentBridgeAuthError 映射鉴权错误。
//
// 参数:
//   - ctx: Gin 上下文。
//   - err: 错误。
func writeAgentBridgeAuthError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, agentbridge.ErrDisabled):
		writeAPIError(ctx, http.StatusServiceUnavailable, "agent_bridge_disabled", "Agent bridge is disabled", nil)
	case errors.Is(err, agentbridge.ErrUnauthorized):
		writeAPIError(ctx, http.StatusUnauthorized, "unauthorized", "Invalid agent bridge token", nil)
	default:
		writeAPIError(ctx, http.StatusUnauthorized, "unauthorized", "Unauthorized", nil)
	}
}

// agentTaskRunResponseFromEnt 转换 run 记录。
//
// 参数:
//   - runRecord: Ent run。
//   - taskID: 任务 ID。
//
// 返回值:
//   - agentTaskRunResponse。
func agentTaskRunResponseFromEnt(runRecord *ent.Run, taskID int) agentTaskRunResponse {
	if runRecord == nil {
		return agentTaskRunResponse{TaskID: taskID}
	}
	resp := agentTaskRunResponse{
		ID:       runRecord.ID,
		TaskID:   taskID,
		Trigger:  string(runRecord.Trigger),
		Status:   string(runRecord.Status),
		ExitCode: runRecord.ExitCode,
		Error:    runRecord.ErrorSummary,
	}
	if !runRecord.StartedAt.IsZero() {
		resp.StartedAt = runRecord.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if runRecord.FinishedAt != nil && !runRecord.FinishedAt.IsZero() {
		resp.Finished = runRecord.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	return resp
}
