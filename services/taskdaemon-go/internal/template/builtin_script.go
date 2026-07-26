package template

import "taskdaemon/internal/runner"

// genericScriptDefinition 通用脚本备份骨架。
//
// 参数:
//   - 无。
//
// 返回值:
//   - Definition: 模板定义。
func genericScriptDefinition() Definition {
	return Definition{
		ID:          "generic-script",
		Name:        "Generic script backup",
		Description: "Render a task draft from a user-provided shell command skeleton. Prefer structured templates when possible.",
		Scenario:    "Custom backup scripts already reviewed by the operator.",
		RunnerType:  runner.TypeShell,
		Params: []ParamDef{
			{Name: "name", Type: ParamString, Required: true, Help: "Task name"},
			{Name: "description", Type: ParamString, Required: false, Default: "", Help: "Task description"},
			{Name: "command", Type: ParamString, Required: true, Help: "Shell command to run (operator-authored; not auto-escaped as a whole)"},
			{Name: "workDir", Type: ParamPath, Required: false, Help: "Optional working directory"},
			{Name: "cronExpression", Type: ParamString, Required: true, Default: "0 4 * * *", Help: "Cron expression"},
			{Name: "timezone", Type: ParamString, Required: false, Default: "Local", Help: "Timezone"},
			{Name: "timeoutSeconds", Type: ParamNumber, Required: false, Default: float64(3600), Min: minMax(1), Help: "Runner timeout seconds"},
		},
		Render: renderGenericScript,
	}
}

// renderGenericScript 渲染通用脚本草稿。
//
// 参数:
//   - values: 已校验参数。
//
// 返回值:
//   - TaskDraft: 任务草稿。
//   - error: 校验错误。
func renderGenericScript(values map[string]any) (TaskDraft, error) {
	command := stringValue(values, "command")
	if command == "" {
		return TaskDraft{}, validationErrorFromFields([]FieldError{{
			Path: "params.command", Reason: "required", Code: CodeFieldRequired,
		}})
	}
	// 拒绝 NUL；整段命令由操作者负责（高级模板）。
	if containsNUL(command) {
		return TaskDraft{}, validationErrorFromFields([]FieldError{{
			Path: "params.command", Reason: "command must not contain NUL", Code: CodeFieldInvalid,
		}})
	}

	workDir := stringValue(values, "workDir")
	if workDir != "" {
		if fe := validatePathValue("params.workDir", workDir); fe != nil {
			return TaskDraft{}, validationErrorFromFields([]FieldError{*fe})
		}
	}

	desc := stringValue(values, "description")
	if desc == "" {
		desc = "Generic script backup draft"
	}

	return TaskDraft{
		Name:                stringValue(values, "name"),
		Description:         desc,
		Enabled:             new(false),
		CronExpression:      stringValue(values, "cronExpression"),
		Timezone:            stringValue(values, "timezone"),
		ConfirmCronWarnings: false,
		Runner: RunnerDraft{
			Type:           string(runner.TypeShell),
			Inline:         command,
			Args:           []string{},
			WorkDir:        workDir,
			Env:            map[string]string{},
			TimeoutSeconds: intValue(values, "timeoutSeconds", 3600),
		},
	}, nil
}

// containsNUL 判断字符串是否含 NUL。
//
// 参数:
//   - s: 输入。
//
// 返回值:
//   - bool: 是否含 NUL。
func containsNUL(s string) bool {
	for i := range s {
		if s[i] == 0 {
			return true
		}
	}
	return false
}
