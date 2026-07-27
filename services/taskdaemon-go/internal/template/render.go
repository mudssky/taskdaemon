package template

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"taskdaemon/internal/runner"
)

// Service 面向 HTTP 的模板查询与渲染入口。
type Service struct {
	// Registry 模板注册表。
	Registry *Registry
	// Logger 可选；用于脱敏后的渲染日志。
	Logger *slog.Logger
}

// Render 根据模板 id 与参数生成任务草稿（不落库）。
//
// 参数:
//   - id: 模板标识。
//   - params: 用户参数（可部分，缺省走默认值）。
//
// 返回值:
//   - TaskDraft: 与 createTaskRequest 兼容的草稿 + 预览。
//   - error: 校验或领域错误。
func (s *Service) Render(id string, params map[string]any) (TaskDraft, error) {
	if s == nil || s.Registry == nil {
		return TaskDraft{}, &DomainError{Code: CodeUnavailable, Message: "template service is unavailable"}
	}
	def, err := s.Registry.Get(id)
	if err != nil {
		return TaskDraft{}, err
	}
	if params == nil {
		params = map[string]any{}
	}
	merged := applyDefaults(def, params)
	values, err := validateParams(def, merged)
	if err != nil {
		return TaskDraft{}, err
	}

	draft, err := def.Render(values)
	if err != nil {
		if ve, ok := AsValidationError(err); ok {
			return TaskDraft{}, ve
		}
		if de, ok := AsDomainError(err); ok {
			return TaskDraft{}, de
		}
		return TaskDraft{}, &DomainError{Code: CodeRenderFailed, Message: "template render failed"}
	}

	// 强制草稿元数据。
	draft.TemplateID = def.ID
	if draft.Enabled == nil {
		draft.Enabled = new(false)
	}
	if draft.Timezone == "" {
		draft.Timezone = "Local"
	}
	if draft.Runner.Type == "" {
		draft.Runner.Type = string(def.RunnerType)
	}
	if draft.Runner.Args == nil {
		draft.Runner.Args = []string{}
	}
	if draft.Runner.Env == nil {
		draft.Runner.Env = map[string]string{}
	}

	// 二次白名单：草稿 runner 必须可校验。
	timeout := time.Duration(draft.Runner.TimeoutSeconds) * time.Second
	if draft.Runner.TimeoutSeconds <= 0 {
		timeout = time.Hour
	}
	if err := runner.Validate(runner.Config{
		Type:             runner.Type(draft.Runner.Type),
		Inline:           draft.Runner.Inline,
		ScriptPath:       draft.Runner.ScriptPath,
		Args:             draft.Runner.Args,
		WorkDir:          draft.Runner.WorkDir,
		Env:              draft.Runner.Env,
		Timeout:          timeout,
		OutputLimitBytes: draft.Runner.OutputLimitBytes,
	}); err != nil {
		return TaskDraft{}, &DomainError{
			Code:    CodeRunnerUnsupported,
			Message: "rendered runner is not supported or invalid",
			Details: map[string]any{"runnerType": draft.Runner.Type},
		}
	}

	// 敏感参数不得进入 inline 明文（secret_ref 值是 env 名，允许出现在说明中，但禁止把“像密码的长串”误当 ref——校验已限制 env 名）。
	// 预览脱敏。
	draft.CommandPreview = maskPreview(draft.Runner.Inline, def, values)
	if draft.CommandPreview == "" {
		draft.CommandPreview = draft.Runner.Inline
	}

	if s.Logger != nil {
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		s.Logger.Info("template rendered",
			"templateId", def.ID,
			"paramKeys", keys,
			// 不记录参数值
		)
	}
	return draft, nil
}

// maskPreview 将敏感参数对应的字面量替换为 ***。
//
// 参数:
//   - inline: 原始命令。
//   - def: 模板定义。
//   - values: 规范化参数。
//
// 返回值:
//   - string: 脱敏预览。
func maskPreview(inline string, def Definition, values map[string]any) string {
	preview := inline
	for _, p := range def.Params {
		if !p.Sensitive && p.Type != ParamSecretRef {
			continue
		}
		raw, ok := values[p.Name]
		if !ok {
			continue
		}
		s, ok := raw.(string)
		if !ok || s == "" {
			continue
		}
		// secret_ref 是 env 名：预览中标注掩码而非替换 env 名本身（env 名非秘密）。
		// 若未来有真正敏感字符串进入 values，则替换其 quote 形式。
		if p.Type == ParamSecretRef {
			continue
		}
		quoted, err := shellQuote(s)
		if err != nil {
			continue
		}
		preview = strings.ReplaceAll(preview, quoted, "'***'")
		preview = strings.ReplaceAll(preview, s, "***")
	}
	// 附加 secret_ref 提示。
	for _, p := range def.Params {
		if p.Type != ParamSecretRef {
			continue
		}
		name := stringValue(values, p.Name)
		if name == "" {
			continue
		}
		preview = fmt.Sprintf("%s  # secret via env %s (not embedded)", preview, name)
	}
	return preview
}
