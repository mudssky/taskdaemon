package httpapi

import (
	"time"

	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
)

type createTaskRequest struct {
	Name                string              `json:"name" binding:"required"`
	Description         string              `json:"description"`
	Enabled             *bool               `json:"enabled"`
	CronExpression      string              `json:"cronExpression" binding:"required"`
	Timezone            string              `json:"timezone"`
	ConfirmCronWarnings bool                `json:"confirmCronWarnings"`
	Runner              createRunnerRequest `json:"runner" binding:"required"`
}

type createRunnerRequest struct {
	Type             string            `json:"type" binding:"required"`
	Inline           string            `json:"inline"`
	ScriptPath       string            `json:"scriptPath"`
	Args             []string          `json:"args"`
	WorkDir          string            `json:"workDir"`
	Env              map[string]string `json:"env"`
	TimeoutSeconds   int               `json:"timeoutSeconds"`
	OutputLimitBytes int               `json:"outputLimitBytes"`
}

type setTaskEnabledRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

type taskResponse struct {
	ID             int            `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Enabled        bool           `json:"enabled"`
	CronExpression string         `json:"cronExpression"`
	Timezone       string         `json:"timezone"`
	RunnerType     string         `json:"runnerType"`
	RunnerConfig   map[string]any `json:"runnerConfig"`
	TimeoutSeconds int            `json:"timeoutSeconds"`
	OverlapPolicy  string         `json:"overlapPolicy"`
	Running        bool           `json:"running"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type runResponse struct {
	ID           int           `json:"id"`
	Trigger      string        `json:"trigger"`
	Status       entrun.Status `json:"status"`
	ExitCode     *int          `json:"exitCode"`
	StartedAt    time.Time     `json:"startedAt"`
	FinishedAt   *time.Time    `json:"finishedAt"`
	DurationMs   int64         `json:"durationMs"`
	ErrorSummary string        `json:"errorSummary"`
	Stdout       string        `json:"stdout"`
	Stderr       string        `json:"stderr"`
}

// taskResponseFromEnt 将任务 Ent 实体转换为 API 响应。
//
// 参数:
//   - taskRecord: 任务 Ent 实体。
//   - running: 当前进程内是否正在执行该任务。
//
// 返回值:
//   - taskResponse: API 响应 DTO。
func taskResponseFromEnt(taskRecord *ent.Task, running bool) taskResponse {
	if taskRecord == nil {
		return taskResponse{}
	}
	return taskResponse{
		ID:             taskRecord.ID,
		Name:           taskRecord.Name,
		Description:    taskRecord.Description,
		Enabled:        taskRecord.Enabled,
		CronExpression: taskRecord.CronExpression,
		Timezone:       taskRecord.Timezone,
		RunnerType:     string(taskRecord.RunnerType),
		RunnerConfig:   taskRecord.RunnerConfig,
		TimeoutSeconds: taskRecord.TimeoutSeconds,
		OverlapPolicy:  string(taskRecord.OverlapPolicy),
		Running:        running,
		CreatedAt:      taskRecord.CreatedAt,
		UpdatedAt:      taskRecord.UpdatedAt,
	}
}

// runResponseFromEnt 将执行历史 Ent 实体转换为 API 响应。
//
// 参数:
//   - runRecord: 执行历史 Ent 实体。
//
// 返回值:
//   - runResponse: API 响应 DTO。
func runResponseFromEnt(runRecord *ent.Run) runResponse {
	if runRecord == nil {
		return runResponse{}
	}
	var exitCode *int
	if runRecord.ExitCode != nil {
		value := *runRecord.ExitCode
		exitCode = &value
	}
	return runResponse{
		ID:           runRecord.ID,
		Trigger:      string(runRecord.Trigger),
		Status:       runRecord.Status,
		ExitCode:     exitCode,
		StartedAt:    runRecord.StartedAt,
		FinishedAt:   runRecord.FinishedAt,
		DurationMs:   runRecord.DurationMs,
		ErrorSummary: runRecord.ErrorSummary,
		Stdout:       runRecord.Stdout,
		Stderr:       runRecord.Stderr,
	}
}
