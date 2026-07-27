// Package template 提供备份任务模板定义、参数校验与安全渲染。
// 模板只生成与任务创建 API 兼容的草稿，不落库、不执行。
package template

import "taskdaemon/internal/runner"

// ParamType 是模板参数的有限类型枚举。
type ParamType string

const (
	// ParamString 普通字符串。
	ParamString ParamType = "string"
	// ParamNumber 数值。
	ParamNumber ParamType = "number"
	// ParamBoolean 布尔。
	ParamBoolean ParamType = "boolean"
	// ParamEnum 枚举选项。
	ParamEnum ParamType = "enum"
	// ParamPath 本地路径字面量。
	ParamPath ParamType = "path"
	// ParamSecretRef 凭据引用（环境变量名），不是密钥明文。
	ParamSecretRef ParamType = "secret_ref"
)

// ParamDef 描述模板的一个参数槽位。
type ParamDef struct {
	// Name 参数名（params 对象键）。
	Name string `json:"name"`
	// Type 有限类型。
	Type ParamType `json:"type"`
	// Required 是否必填（无默认值时）。
	Required bool `json:"required"`
	// Default 可选默认值（JSON 兼容标量）。
	Default any `json:"default,omitempty"`
	// EnumOptions Type=enum 时的合法取值。
	EnumOptions []string `json:"enumOptions,omitempty"`
	// Help 帮助文本。
	Help string `json:"help,omitempty"`
	// Min 数值下限（含）。
	Min *float64 `json:"min,omitempty"`
	// Max 数值上限（含）。
	Max *float64 `json:"max,omitempty"`
	// Sensitive 预览/日志是否掩码（secret_ref 默认视为敏感）。
	Sensitive bool `json:"sensitive,omitempty"`
}

// Definition 是一条可注册的模板定义。
type Definition struct {
	// ID 稳定标识。
	ID string `json:"id"`
	// Name 展示名。
	Name string `json:"name"`
	// Description 描述。
	Description string `json:"description"`
	// Scenario 适用场景说明。
	Scenario string `json:"scenario"`
	// RunnerType 生成的 runner 类型，必须在白名单内。
	RunnerType runner.Type `json:"runnerType"`
	// Params 参数定义列表。
	Params []ParamDef `json:"params"`
	// Render 将已校验参数渲染为任务草稿（不含 commandPreview）。
	// 框架会再校验 runner 并生成脱敏预览。
	Render func(values map[string]any) (TaskDraft, error) `json:"-"`
}

// RunnerDraft 与 createRunnerRequest 字段同形。
type RunnerDraft struct {
	Type             string            `json:"type"`
	Inline           string            `json:"inline"`
	ScriptPath       string            `json:"scriptPath,omitempty"`
	Args             []string          `json:"args,omitempty"`
	WorkDir          string            `json:"workDir,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	TimeoutSeconds   int               `json:"timeoutSeconds"`
	OutputLimitBytes int               `json:"outputLimitBytes,omitempty"`
}

// TaskDraft 与 createTaskRequest 兼容，并附带预览与模板来源。
type TaskDraft struct {
	Name                string      `json:"name"`
	Description         string      `json:"description"`
	Enabled             *bool       `json:"enabled"`
	CronExpression      string      `json:"cronExpression"`
	Timezone            string      `json:"timezone"`
	ConfirmCronWarnings bool        `json:"confirmCronWarnings"`
	Runner              RunnerDraft `json:"runner"`
	// CommandPreview 可读命令预览（敏感值已掩码）。
	CommandPreview string `json:"commandPreview"`
	// TemplateID 来源模板。
	TemplateID string `json:"templateId"`
}
