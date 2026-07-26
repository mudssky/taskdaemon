package template_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/runner"
	"taskdaemon/internal/template"
)

// TestShellQuoteViaRender 验证注入载荷经渲染后以单引号字面量进入命令。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestShellQuoteViaRender(t *testing.T) {
	svc := &template.Service{Registry: template.DefaultRegistry()}
	payloads := []string{
		";dropdb",
		"`id`",
		"$(reboot)",
		"with'quote",
	}
	for _, payload := range payloads {
		draft, err := svc.Render("postgres-pg-dump", map[string]any{
			"name":           "t",
			"host":           payload,
			"user":           "u",
			"database":       "d",
			"outputPath":     "/tmp/out.dump",
			"cronExpression": "0 1 * * *",
		})
		require.NoError(t, err, payload)
		require.Contains(t, draft.Runner.Inline, "-h '")
		// 单引号在 payload 内会被 '\'' 转义，不再连续出现原文。
		if strings.Contains(payload, "'") {
			require.Contains(t, draft.Runner.Inline, `'\''`)
		} else {
			require.Contains(t, draft.Runner.Inline, payload)
		}
	}
}

// TestPgDumpRenderQuotesInjection 验证恶意 host 不能逃逸参数位置。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestPgDumpRenderQuotesInjection(t *testing.T) {
	svc := &template.Service{Registry: template.DefaultRegistry()}
	payloads := []string{"; dropdb evil", "`id`", "$(reboot)", "with'quote"}
	for _, payload := range payloads {
		draft, err := svc.Render("postgres-pg-dump", map[string]any{
			"name":           "t",
			"host":           payload,
			"user":           "u",
			"database":       "d",
			"outputPath":     "/tmp/out.dump",
			"cronExpression": "0 1 * * *",
		})
		require.NoError(t, err, payload)
		inline := draft.Runner.Inline
		// 命令必须以 pg_dump 开头，且 payload 只以 quote 形式出现。
		require.True(t, strings.HasPrefix(inline, "pg_dump "))
		require.Contains(t, inline, "'")
		// 未引用的分号注入形态不应出现：... -h ; dropdb
		require.NotContains(t, inline, "-h ;")
		require.NotContains(t, inline, "-h `")
		require.NotContains(t, inline, "-h $(")
		// 单引号包裹
		require.Contains(t, inline, "-h '")
		// 密码不明文
		require.NotContains(t, inline, "password")
		require.Empty(t, draft.Runner.Env)
		require.NotNil(t, draft.Enabled)
		require.False(t, *draft.Enabled)
		require.Equal(t, "postgres-pg-dump", draft.TemplateID)
		require.NotEmpty(t, draft.CommandPreview)
	}
}

// TestPathControlCharactersRejected 验证含换行的路径被拒绝。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestPathControlCharactersRejected(t *testing.T) {
	svc := &template.Service{Registry: template.DefaultRegistry()}
	_, err := svc.Render("postgres-pg-dump", map[string]any{
		"name":           "t",
		"host":           "h",
		"user":           "u",
		"database":       "d",
		"outputPath":     "/tmp/out\ndump",
		"cronExpression": "0 1 * * *",
	})
	require.Error(t, err)
	ve, ok := template.AsValidationError(err)
	require.True(t, ok)
	require.NotEmpty(t, ve.Fields)
	require.Equal(t, "params.outputPath", ve.Fields[0].Path)
}

// TestSecretRefInvalidRejected 验证非法 secret_ref。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestSecretRefInvalidRejected(t *testing.T) {
	svc := &template.Service{Registry: template.DefaultRegistry()}
	_, err := svc.Render("postgres-pg-dump", map[string]any{
		"name":           "t",
		"host":           "h",
		"user":           "u",
		"database":       "d",
		"outputPath":     "/tmp/out.dump",
		"passwordEnv":    "a;b",
		"cronExpression": "0 1 * * *",
	})
	require.Error(t, err)
	ve, ok := template.AsValidationError(err)
	require.True(t, ok)
	require.Equal(t, "params.passwordEnv", ve.Fields[0].Path)
}

// TestRegisterRejectsUnsupportedRunner 验证注册期 runner 白名单。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRegisterRejectsUnsupportedRunner(t *testing.T) {
	reg := template.NewRegistry()
	err := reg.Register(template.Definition{
		ID:         "evil",
		Name:       "evil",
		RunnerType: runner.Type("raw-command"),
		Render: func(map[string]any) (template.TaskDraft, error) {
			return template.TaskDraft{}, nil
		},
	})
	require.Error(t, err)
	de, ok := template.AsDomainError(err)
	require.True(t, ok)
	require.Equal(t, template.CodeRunnerUnsupported, de.Code)
}

// TestRegisterExtraTemplateWithoutFrameworkChange 证明新增模板只需 Register。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRegisterExtraTemplateWithoutFrameworkChange(t *testing.T) {
	reg := template.NewRegistry()
	require.NoError(t, reg.Register(template.Definition{
		ID:         "test-extra",
		Name:       "Extra test template",
		RunnerType: runner.TypeShell,
		Params: []template.ParamDef{
			{Name: "name", Type: template.ParamString, Required: true},
			{Name: "cronExpression", Type: template.ParamString, Required: true, Default: "0 0 * * *"},
		},
		Render: func(values map[string]any) (template.TaskDraft, error) {
			return template.TaskDraft{
				Name:           values["name"].(string),
				CronExpression: values["cronExpression"].(string),
				Runner: template.RunnerDraft{
					Type:           string(runner.TypeShell),
					Inline:         "echo ok",
					TimeoutSeconds: 60,
				},
			}, nil
		},
	}))
	list := reg.List()
	require.Len(t, list, 1)
	require.Equal(t, "test-extra", list[0].ID)

	svc := &template.Service{Registry: reg}
	draft, err := svc.Render("test-extra", map[string]any{"name": "n"})
	require.NoError(t, err)
	require.Equal(t, "echo ok", draft.Runner.Inline)
	require.Equal(t, "test-extra", draft.TemplateID)
}

// TestDefaultRegistryHasThreeBuiltins 验证首批三模板。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestDefaultRegistryHasThreeBuiltins(t *testing.T) {
	reg := template.DefaultRegistry()
	list := reg.List()
	require.Len(t, list, 3)
	ids := []string{list[0].ID, list[1].ID, list[2].ID}
	require.Contains(t, ids, "postgres-pg-dump")
	require.Contains(t, ids, "sqlite-file-backup")
	require.Contains(t, ids, "generic-script")
	for _, def := range list {
		require.NotEmpty(t, def.Params)
		require.Equal(t, runner.TypeShell, def.RunnerType)
	}
}

// TestSQLiteAndScriptRender 冒烟三模板渲染。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestSQLiteAndScriptRender(t *testing.T) {
	svc := &template.Service{Registry: template.DefaultRegistry()}

	sqliteDraft, err := svc.Render("sqlite-file-backup", map[string]any{
		"name":       "s",
		"sourcePath": "/data/app.db",
		"destPath":   "/backup/app.db",
	})
	require.NoError(t, err)
	require.Contains(t, sqliteDraft.Runner.Inline, "sqlite3")
	require.Contains(t, sqliteDraft.Runner.Inline, "'/data/app.db'")
	require.Contains(t, sqliteDraft.Runner.Inline, "cp")

	// 注入路径
	sqliteDraft2, err := svc.Render("sqlite-file-backup", map[string]any{
		"name":            "s",
		"sourcePath":      "/data/app.db;rm -rf /",
		"destPath":        "/backup/app.db",
		"verifyIntegrity": false,
		"cronExpression":  "0 3 * * *",
	})
	require.NoError(t, err)
	require.Contains(t, sqliteDraft2.Runner.Inline, "'/data/app.db;rm -rf /'")
	require.NotContains(t, sqliteDraft2.Runner.Inline, "cp /data/app.db;rm")

	scriptDraft, err := svc.Render("generic-script", map[string]any{
		"name":    "g",
		"command": "rsync -a /data/ /backup/",
	})
	require.NoError(t, err)
	require.Equal(t, "rsync -a /data/ /backup/", scriptDraft.Runner.Inline)
}

// TestRequiredAndRangeValidation 验证必填与数值范围。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestRequiredAndRangeValidation(t *testing.T) {
	svc := &template.Service{Registry: template.DefaultRegistry()}
	_, err := svc.Render("postgres-pg-dump", map[string]any{
		"host": "h",
	})
	require.Error(t, err)
	ve, ok := template.AsValidationError(err)
	require.True(t, ok)
	require.NotEmpty(t, ve.Fields)

	_, err = svc.Render("postgres-pg-dump", map[string]any{
		"name":           "t",
		"host":           "h",
		"user":           "u",
		"database":       "d",
		"outputPath":     "/tmp/o",
		"port":           float64(99999),
		"cronExpression": "0 1 * * *",
	})
	require.Error(t, err)
	ve, ok = template.AsValidationError(err)
	require.True(t, ok)
	require.Equal(t, template.CodeFieldOutOfRange, ve.Fields[0].Code)
}

// TestDraftShapeCompatibleWithCreateTaskRequest 字段形状兼容任务创建。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestDraftShapeCompatibleWithCreateTaskRequest(t *testing.T) {
	svc := &template.Service{Registry: template.DefaultRegistry()}
	draft, err := svc.Render("postgres-pg-dump", map[string]any{
		"name":           "backup-app",
		"host":           "db.internal",
		"user":           "backup",
		"database":       "app",
		"outputPath":     "/var/backups/app.dump",
		"cronExpression": "30 2 * * *",
		"timezone":       "Asia/Shanghai",
		"timeoutSeconds": float64(7200),
	})
	require.NoError(t, err)
	require.Equal(t, "backup-app", draft.Name)
	require.Equal(t, "30 2 * * *", draft.CronExpression)
	require.Equal(t, "Asia/Shanghai", draft.Timezone)
	require.Equal(t, "shell", draft.Runner.Type)
	require.NotEmpty(t, draft.Runner.Inline)
	require.Equal(t, 7200, draft.Runner.TimeoutSeconds)
	require.NotNil(t, draft.Enabled)
	require.False(t, *draft.Enabled)
	// createTaskRequest 需要的关键字段均存在
	require.NotEmpty(t, draft.Runner.Type)
}
