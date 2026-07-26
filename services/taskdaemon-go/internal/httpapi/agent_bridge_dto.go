package httpapi

// G6: agent → taskdaemon 入站桥接 DTO。

// triggerAgentTaskRequest 是 agent 触发任务的请求体。
type triggerAgentTaskRequest struct {
	// 可选备注，不落敏感字段。
	Note string `json:"note"`
}

// agentTaskRunResponse 是 agent 可见的 run 摘要。
type agentTaskRunResponse struct {
	ID        int    `json:"id"`
	TaskID    int    `json:"taskId"`
	Trigger   string `json:"trigger"`
	Status    string `json:"status"`
	ExitCode  *int   `json:"exitCode,omitempty"`
	StartedAt string `json:"startedAt,omitempty"`
	Finished  string `json:"finishedAt,omitempty"`
	Error     string `json:"errorSummary,omitempty"`
}

// agentTaskResponse 是 agent 可见的任务摘要（无管理员字段）。
type agentTaskResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
}
