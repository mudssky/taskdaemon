package notify

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// 决策 A：Detail 白名单过滤放在 notify 包内，不 import httpapi。
// httpapi/redaction.go 只服务 HTTP 请求日志 body 脱敏；notify 作为底层事件包
// 若反向依赖 httpapi 会形成坏依赖。调用方构造事件时，未列入白名单的键会被静默丢弃。
//
// 决策 B：scheduler 通过 Publisher 接口直接使用 notify.Event（见 internal/scheduler），
// 避免在 scheduler 侧复制一套最小事件结构再由 app 层转换，降低契约漂移风险。

// Name 是通知事件名，采用 <domain>.<entity>.<action> 三段式。
// 新增只能追加常量，禁止改名或改语义（C-2 冻结契约）。
type Name string

const (
	// NameTaskRunSucceeded 表示任务运行成功。
	NameTaskRunSucceeded Name = "task.run.succeeded"
	// NameTaskRunFailed 表示任务运行失败。
	NameTaskRunFailed Name = "task.run.failed"
	// NameTaskRunTimeout 表示任务运行超时。
	NameTaskRunTimeout Name = "task.run.timeout"
	// NameTaskRunCancelled 表示任务运行被取消。
	NameTaskRunCancelled Name = "task.run.cancelled"
	// NameTaskRunSkipped 表示任务因重叠策略被跳过。
	NameTaskRunSkipped Name = "task.run.skipped"
	// NameSchedulerStarted 表示调度器已启动。
	NameSchedulerStarted Name = "scheduler.lifecycle.started"
	// NameSchedulerStopped 表示调度器已停止。
	NameSchedulerStopped Name = "scheduler.lifecycle.stopped"
)

// Severity 是通知严重级别有限枚举（与日志级别独立）。
type Severity string

const (
	// SeverityInfo 表示一般信息。
	SeverityInfo Severity = "info"
	// SeverityWarning 表示需要关注的警告。
	SeverityWarning Severity = "warning"
	// SeverityError 表示错误。
	SeverityError Severity = "error"
	// SeverityCritical 表示严重错误。
	SeverityCritical Severity = "critical"
)

// Valid 判断严重级别是否为已知枚举值。
//
// 参数:
//   - severity: 待校验的严重级别。
//
// 返回值:
//   - bool: 合法时返回 true。
func (severity Severity) Valid() bool {
	switch severity {
	case SeverityInfo, SeverityWarning, SeverityError, SeverityCritical:
		return true
	default:
		return false
	}
}

// SubjectKind 是事件关联实体类型。
type SubjectKind string

const (
	// SubjectKindTask 关联任务定义。
	SubjectKindTask SubjectKind = "task"
	// SubjectKindRun 关联一次执行记录。
	SubjectKindRun SubjectKind = "run"
	// SubjectKindScheduler 关联调度器本身。
	SubjectKindScheduler SubjectKind = "scheduler"
)

// Subject 描述事件关联的业务实体。
type Subject struct {
	Kind string // "task" | "run" | "scheduler"
	ID   string // 实体 ID；scheduler 类可为空
}

// Event 是 C-2 通知事件的稳定 payload。
// 字段一旦冻结，新增只能追加，禁止改名或改语义。
type Event struct {
	ID         string         // 事件唯一标识，sink 幂等键
	Name       Name           // 具名常量
	Severity   Severity       // 严重级别
	OccurredAt time.Time      // 事件发生时间
	Subject    Subject        // 关联实体
	Title      string         // 人类可读标题
	Body       string         // 人类可读正文
	Detail     map[string]any // 已脱敏的结构化补充
}

// NewEvent 构造通知事件，并在构造期拒绝非法 Severity、过滤 Detail 白名单外键。
//
// 参数:
//   - name: 事件名常量。
//   - severity: 严重级别。
//   - occurredAt: 事件发生时间；零值时使用当前时间。
//   - subject: 关联实体。
//   - title: 人类可读标题。
//   - body: 人类可读正文。
//   - detail: 结构化补充；仅白名单键会保留。
//
// 返回值:
//   - Event: 构造完成的事件。
//   - error: Severity 非法时返回错误。
func NewEvent(
	name Name,
	severity Severity,
	occurredAt time.Time,
	subject Subject,
	title string,
	body string,
	detail map[string]any,
) (Event, error) {
	if !severity.Valid() {
		return Event{}, fmt.Errorf("invalid notify severity %q", severity)
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		ID:         uuid.NewString(),
		Name:       name,
		Severity:   severity,
		OccurredAt: occurredAt.UTC(),
		Subject:    subject,
		Title:      title,
		Body:       body,
		Detail:     filterDetail(detail),
	}, nil
}

// NewEventWithID 使用指定 event ID 构造事件（便于 run 终态幂等）。
//
// 参数:
//   - id: 事件唯一标识；空字符串时自动生成。
//   - name: 事件名常量。
//   - severity: 严重级别。
//   - occurredAt: 事件发生时间；零值时使用当前时间。
//   - subject: 关联实体。
//   - title: 人类可读标题。
//   - body: 人类可读正文。
//   - detail: 结构化补充；仅白名单键会保留。
//
// 返回值:
//   - Event: 构造完成的事件。
//   - error: Severity 非法时返回错误。
func NewEventWithID(
	id string,
	name Name,
	severity Severity,
	occurredAt time.Time,
	subject Subject,
	title string,
	body string,
	detail map[string]any,
) (Event, error) {
	event, err := NewEvent(name, severity, occurredAt, subject, title, body, detail)
	if err != nil {
		return Event{}, err
	}
	if id != "" {
		event.ID = id
	}
	return event, nil
}

// allowedDetailKeys 是 Detail 允许出现的键集合。
// 命令行、环境变量、URL query、token、密码等敏感键一律不在此列。
var allowedDetailKeys = map[string]struct{}{
	"taskId":     {},
	"runId":      {},
	"status":     {},
	"exitCode":   {},
	"durationMs": {},
	"trigger":    {},
	"errorKind":  {},
}

// filterDetail 仅保留白名单键，并丢弃明显敏感的字符串值形态。
//
// 参数:
//   - detail: 调用方传入的结构化补充。
//
// 返回值:
//   - map[string]any: 过滤后的 detail；无合法键时返回 nil。
func filterDetail(detail map[string]any) map[string]any {
	if len(detail) == 0 {
		return nil
	}
	filtered := make(map[string]any, len(detail))
	for key, value := range detail {
		if _, ok := allowedDetailKeys[key]; !ok {
			continue
		}
		if text, ok := value.(string); ok && looksSensitive(text) {
			continue
		}
		filtered[key] = value
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// looksSensitive 对字符串值做保守敏感检测。
//
// 参数:
//   - value: 待检查的字符串。
//
// 返回值:
//   - bool: 疑似敏感时返回 true。
func looksSensitive(value string) bool {
	lower := toLowerASCII(value)
	// 常见密钥/口令形态：避免把误塞进 detail 的敏感串落库
	needles := []string{
		"password=",
		"token=",
		"secret=",
		"authorization:",
		"bearer ",
		"api_key=",
		"apikey=",
	}
	for _, needle := range needles {
		if containsASCII(lower, needle) {
			return true
		}
	}
	return false
}

// toLowerASCII 将 ASCII 字母转为小写，避免引入 strings 依赖之外的额外分配。
//
// 参数:
//   - value: 原始字符串。
//
// 返回值:
//   - string: 小写后的字符串。
func toLowerASCII(value string) string {
	buf := make([]byte, len(value))
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[i] = c
	}
	return string(buf)
}

// containsASCII 判断 haystack 是否包含 needle（均为已小写的 ASCII）。
//
// 参数:
//   - haystack: 被搜索字符串。
//   - needle: 子串。
//
// 返回值:
//   - bool: 包含时返回 true。
func containsASCII(haystack string, needle string) bool {
	if needle == "" {
		return true
	}
	n := len(needle)
	if len(haystack) < n {
		return false
	}
	for i := 0; i+n <= len(haystack); i++ {
		if haystack[i:i+n] == needle {
			return true
		}
	}
	return false
}

// TaskRunEventID 生成任务运行终态事件的稳定幂等 ID。
//
// 参数:
//   - name: 事件名。
//   - runID: 执行历史 ID。
//
// 返回值:
//   - string: 形如 "<name>:run:<id>" 的事件 ID。
func TaskRunEventID(name Name, runID int) string {
	return fmt.Sprintf("%s:run:%d", name, runID)
}
