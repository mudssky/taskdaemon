package template

import (
	"fmt"

	"taskdaemon/internal/runner"
)

// postgresPgDumpDefinition PostgreSQL 逻辑备份（pg_dump）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - Definition: 模板定义。
func postgresPgDumpDefinition() Definition {
	return Definition{
		ID:          "postgres-pg-dump",
		Name:        "PostgreSQL logical backup (pg_dump)",
		Description: "Render a scheduled pg_dump task draft. Password is never embedded; provide it via host env (default PGPASSWORD).",
		Scenario:    "Nightly or on-demand logical backup of a single PostgreSQL database.",
		RunnerType:  runner.TypeShell,
		Params: []ParamDef{
			{Name: "name", Type: ParamString, Required: true, Help: "Task name"},
			{Name: "description", Type: ParamString, Required: false, Default: "", Help: "Task description"},
			{Name: "host", Type: ParamString, Required: true, Default: "127.0.0.1", Help: "PostgreSQL host"},
			{Name: "port", Type: ParamNumber, Required: false, Default: float64(5432), Min: minMax(1), Max: minMax(65535), Help: "PostgreSQL port"},
			{Name: "user", Type: ParamString, Required: true, Help: "Database user"},
			{Name: "database", Type: ParamString, Required: true, Help: "Database name"},
			{Name: "passwordEnv", Type: ParamSecretRef, Required: false, Default: "PGPASSWORD", Help: "Env var name holding the password on the host (not the password itself)", Sensitive: true},
			{Name: "outputPath", Type: ParamPath, Required: true, Help: "Backup file output path"},
			{Name: "compress", Type: ParamBoolean, Required: false, Default: true, Help: "Use custom compressed format (-Fc)"},
			{Name: "cronExpression", Type: ParamString, Required: true, Default: "30 2 * * *", Help: "Cron expression"},
			{Name: "timezone", Type: ParamString, Required: false, Default: "Local", Help: "Timezone"},
			{Name: "timeoutSeconds", Type: ParamNumber, Required: false, Default: float64(3600), Min: minMax(1), Help: "Runner timeout seconds"},
			{Name: "retainDays", Type: ParamNumber, Required: false, Default: float64(7), Min: minMax(0), Max: minMax(3650), Help: "Retention hint in days (documented in description; not auto-pruned)"},
		},
		Render: renderPostgresPgDump,
	}
}

// renderPostgresPgDump 渲染 pg_dump 草稿。
//
// 参数:
//   - values: 已校验参数。
//
// 返回值:
//   - TaskDraft: 任务草稿。
//   - error: 引用失败。
func renderPostgresPgDump(values map[string]any) (TaskDraft, error) {
	host := stringValue(values, "host")
	user := stringValue(values, "user")
	database := stringValue(values, "database")
	outputPath := stringValue(values, "outputPath")
	passwordEnv := stringValue(values, "passwordEnv")
	if passwordEnv == "" {
		passwordEnv = "PGPASSWORD"
	}
	port := intValue(values, "port", 5432)
	compress := boolValue(values, "compress")
	retainDays := intValue(values, "retainDays", 7)

	qHost, err := shellQuote(host)
	if err != nil {
		return TaskDraft{}, validationErrorFromFields([]FieldError{{Path: "params.host", Reason: "invalid host", Code: CodeFieldInvalid}})
	}
	qUser, err := shellQuote(user)
	if err != nil {
		return TaskDraft{}, validationErrorFromFields([]FieldError{{Path: "params.user", Reason: "invalid user", Code: CodeFieldInvalid}})
	}
	qDB, err := shellQuote(database)
	if err != nil {
		return TaskDraft{}, validationErrorFromFields([]FieldError{{Path: "params.database", Reason: "invalid database", Code: CodeFieldInvalid}})
	}
	qOut, err := shellQuote(outputPath)
	if err != nil {
		return TaskDraft{}, validationErrorFromFields([]FieldError{{Path: "params.outputPath", Reason: "invalid output path", Code: CodeFieldInvalid}})
	}
	qPort, err := shellQuote(fmt.Sprintf("%d", port))
	if err != nil {
		return TaskDraft{}, validationErrorFromFields([]FieldError{{Path: "params.port", Reason: "invalid port", Code: CodeFieldInvalid}})
	}

	// 固定 argv 骨架；用户输入只出现在 shellQuote 字面量中。
	// 密码不进命令：依赖宿主环境中的 passwordEnv（libpq 读取 PGPASSWORD 等）。
	parts := []string{
		"pg_dump",
		"-h", qHost,
		"-p", qPort,
		"-U", qUser,
		"-d", qDB,
	}
	if compress {
		parts = append(parts, "-Fc")
	}
	parts = append(parts, "-f", qOut)
	inline := joinCommand(parts...)

	desc := stringValue(values, "description")
	if desc == "" {
		desc = fmt.Sprintf(
			"pg_dump of %s@%s:%d/%s; password via host env %s (not stored in task); retainDays hint=%d",
			user, host, port, database, passwordEnv, retainDays,
		)
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
			Inline:         inline,
			Args:           []string{},
			Env:            map[string]string{}, // 不写入密码；宿主需提供 passwordEnv
			TimeoutSeconds: intValue(values, "timeoutSeconds", 3600),
		},
	}, nil
}
