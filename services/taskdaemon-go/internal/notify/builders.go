package notify

import (
	"fmt"
	"time"
)

// BuildTaskRunEvent 根据任务终态构造安全的通知事件。
// Title/Body 使用固定模板，Detail 仅含白名单字段，避免命令行/输出/env 泄漏。
//
// 参数:
//   - name: 事件名（succeeded/failed/timeout/cancelled/skipped）。
//   - taskID: 任务 ID。
//   - runID: 执行历史 ID。
//   - occurredAt: 事件发生时间。
//   - exitCode: 可选退出码。
//   - durationMs: 可选耗时毫秒。
//   - trigger: 触发来源（cron/manual）。
//
// 返回值:
//   - Event: 构造完成的事件。
//   - error: 非法事件名或 Severity 时返回错误。
func BuildTaskRunEvent(
	name Name,
	taskID int,
	runID int,
	occurredAt time.Time,
	exitCode *int,
	durationMs int64,
	trigger string,
) (Event, error) {
	severity, title, body, err := taskRunPresentation(name, taskID, runID)
	if err != nil {
		return Event{}, err
	}
	detail := map[string]any{
		"taskId": taskID,
		"runId":  runID,
		"status": string(name),
	}
	if exitCode != nil {
		detail["exitCode"] = *exitCode
	}
	if durationMs > 0 {
		detail["durationMs"] = durationMs
	}
	if trigger != "" {
		detail["trigger"] = trigger
	}
	return NewEventWithID(
		TaskRunEventID(name, runID),
		name,
		severity,
		occurredAt,
		Subject{Kind: string(SubjectKindRun), ID: fmt.Sprintf("%d", runID)},
		title,
		body,
		detail,
	)
}

// taskRunPresentation 返回任务终态对应的级别、标题与正文。
//
// 参数:
//   - name: 事件名。
//   - taskID: 任务 ID。
//   - runID: 执行历史 ID。
//
// 返回值:
//   - Severity: 严重级别。
//   - string: 标题。
//   - string: 正文。
//   - error: 未知事件名时返回错误。
func taskRunPresentation(name Name, taskID int, runID int) (Severity, string, string, error) {
	switch name {
	case NameTaskRunSucceeded:
		return SeverityInfo,
			"Task run succeeded",
			fmt.Sprintf("Task %d run %d completed successfully", taskID, runID),
			nil
	case NameTaskRunFailed:
		return SeverityError,
			"Task run failed",
			fmt.Sprintf("Task %d run %d failed", taskID, runID),
			nil
	case NameTaskRunTimeout:
		return SeverityError,
			"Task run timed out",
			fmt.Sprintf("Task %d run %d timed out", taskID, runID),
			nil
	case NameTaskRunCancelled:
		return SeverityWarning,
			"Task run cancelled",
			fmt.Sprintf("Task %d run %d was cancelled", taskID, runID),
			nil
	case NameTaskRunSkipped:
		return SeverityWarning,
			"Task run skipped",
			fmt.Sprintf("Task %d run %d was skipped due to overlap", taskID, runID),
			nil
	default:
		return "", "", "", fmt.Errorf("unsupported task run event name %q", name)
	}
}

// BuildSchedulerLifecycleEvent 构造调度器启停事件。
//
// 参数:
//   - name: started 或 stopped。
//   - occurredAt: 事件发生时间。
//
// 返回值:
//   - Event: 构造完成的事件。
//   - error: 非法事件名时返回错误。
func BuildSchedulerLifecycleEvent(name Name, occurredAt time.Time) (Event, error) {
	var severity Severity
	var title string
	var body string
	switch name {
	case NameSchedulerStarted:
		severity = SeverityInfo
		title = "Scheduler started"
		body = "Task scheduler has started"
	case NameSchedulerStopped:
		severity = SeverityInfo
		title = "Scheduler stopped"
		body = "Task scheduler has stopped"
	default:
		return Event{}, fmt.Errorf("unsupported scheduler event name %q", name)
	}
	return NewEvent(
		name,
		severity,
		occurredAt,
		Subject{Kind: string(SubjectKindScheduler)},
		title,
		body,
		nil,
	)
}
