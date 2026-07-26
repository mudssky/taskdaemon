package template

import (
	"fmt"

	"taskdaemon/internal/runner"
)

// sqliteFileBackupDefinition SQLite 文件备份。
//
// 参数:
//   - 无。
//
// 返回值:
//   - Definition: 模板定义。
func sqliteFileBackupDefinition() Definition {
	return Definition{
		ID:          "sqlite-file-backup",
		Name:        "SQLite file backup",
		Description: "Copy a SQLite database file to a destination path, optionally verifying integrity first.",
		Scenario:    "File-level backup of a SQLite database used by local services.",
		RunnerType:  runner.TypeShell,
		Params: []ParamDef{
			{Name: "name", Type: ParamString, Required: true, Help: "Task name"},
			{Name: "description", Type: ParamString, Required: false, Default: "", Help: "Task description"},
			{Name: "sourcePath", Type: ParamPath, Required: true, Help: "Source SQLite database path"},
			{Name: "destPath", Type: ParamPath, Required: true, Help: "Destination backup path"},
			{Name: "verifyIntegrity", Type: ParamBoolean, Required: false, Default: true, Help: "Run PRAGMA integrity_check before copy"},
			{Name: "cronExpression", Type: ParamString, Required: true, Default: "0 3 * * *", Help: "Cron expression"},
			{Name: "timezone", Type: ParamString, Required: false, Default: "Local", Help: "Timezone"},
			{Name: "timeoutSeconds", Type: ParamNumber, Required: false, Default: float64(1800), Min: minMax(1), Help: "Runner timeout seconds"},
		},
		Render: renderSQLiteFileBackup,
	}
}

// renderSQLiteFileBackup 渲染 SQLite 备份草稿。
//
// 参数:
//   - values: 已校验参数。
//
// 返回值:
//   - TaskDraft: 任务草稿。
//   - error: 引用失败。
func renderSQLiteFileBackup(values map[string]any) (TaskDraft, error) {
	source := stringValue(values, "sourcePath")
	dest := stringValue(values, "destPath")
	verify := boolValue(values, "verifyIntegrity")

	qSrc, err := shellQuote(source)
	if err != nil {
		return TaskDraft{}, validationErrorFromFields([]FieldError{{Path: "params.sourcePath", Reason: "invalid path", Code: CodeFieldInvalid}})
	}
	qDst, err := shellQuote(dest)
	if err != nil {
		return TaskDraft{}, validationErrorFromFields([]FieldError{{Path: "params.destPath", Reason: "invalid path", Code: CodeFieldInvalid}})
	}

	var inline string
	if verify {
		// 固定骨架：sqlite3 <src> 'PRAGMA integrity_check;' | grep -qx 'ok' && cp <src> <dst>
		// 用户路径只出现在 shellQuote 字面量中。
		qCheckSQL, _ := shellQuote("PRAGMA integrity_check;")
		qOK, _ := shellQuote("ok")
		inline = joinCommand(
			"sqlite3", qSrc, qCheckSQL, "|", "grep", "-qx", qOK,
			"&&", "cp", qSrc, qDst,
		)
	} else {
		inline = joinCommand("cp", qSrc, qDst)
	}

	desc := stringValue(values, "description")
	if desc == "" {
		desc = fmt.Sprintf("SQLite backup %s -> %s (verifyIntegrity=%v)", source, dest, verify)
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
			Env:            map[string]string{},
			TimeoutSeconds: intValue(values, "timeoutSeconds", 1800),
		},
	}, nil
}
