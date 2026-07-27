package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"taskdaemon/internal/config"
)

// TestServeCommandLoadsConfigAndRunsServe 验证 serve 子命令会加载 --config 并进入后端 API 模式。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestServeCommandLoadsConfigAndRunsServe(t *testing.T) {
	var loadedPath string
	var hookLoadPath string
	var served bool
	hooks := Hooks{
		Serve: func(_ context.Context, cfg config.Config, opts config.LoadOptions) error {
			served = true
			hookLoadPath = opts.ConfigPath
			if cfg.Server.Port != 9090 {
				t.Fatalf("serve config port = %d, want 9090", cfg.Server.Port)
			}
			return nil
		},
	}

	err := Execute(context.Background(), Options{
		Args:   []string{"serve", "--config", "custom.yaml"},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			loadedPath = opts.ConfigPath
			cfg := config.Default()
			cfg.Server.Port = 9090
			return cfg, nil
		},
		Hooks: hooks,
	})
	if err != nil {
		t.Fatalf("execute serve: %v", err)
	}
	if loadedPath != "custom.yaml" {
		t.Fatalf("loaded path = %s, want custom.yaml", loadedPath)
	}
	if !served {
		t.Fatal("serve hook was not called")
	}
	if hookLoadPath != "custom.yaml" {
		t.Fatalf("serve load path = %s, want custom.yaml", hookLoadPath)
	}
}

// TestCLIOnlyCommandDoesNotStartDesktop 验证 CLI-only 子命令不会初始化 Desktop。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestCLIOnlyCommandDoesNotStartDesktop(t *testing.T) {
	var migrated bool
	var desktopStarted bool
	hooks := Hooks{
		DBMigrate: func(context.Context, config.Config) error {
			migrated = true
			return nil
		},
		Desktop: func(context.Context, config.Config) error {
			desktopStarted = true
			return nil
		},
	}

	err := Execute(context.Background(), Options{
		Args:       []string{"db", "migrate"},
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		LoadConfig: staticConfigLoader(),
		Hooks:      hooks,
	})
	if err != nil {
		t.Fatalf("execute db migrate: %v", err)
	}
	if !migrated {
		t.Fatal("db migrate hook was not called")
	}
	if desktopStarted {
		t.Fatal("desktop hook should not be called for db migrate")
	}
}

// TestDesktopCommandUsesDesktopHook 验证 desktop 子命令只进入桌面入口。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestDesktopCommandUsesDesktopHook(t *testing.T) {
	var desktopStarted bool
	var served bool
	hooks := Hooks{
		Desktop: func(context.Context, config.Config) error {
			desktopStarted = true
			return nil
		},
		Serve: func(context.Context, config.Config, config.LoadOptions) error {
			served = true
			return nil
		},
	}

	err := Execute(context.Background(), Options{
		Args:       []string{"desktop"},
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		LoadConfig: staticConfigLoader(),
		Hooks:      hooks,
	})
	if err != nil {
		t.Fatalf("execute desktop: %v", err)
	}
	if !desktopStarted {
		t.Fatal("desktop hook was not called")
	}
	if served {
		t.Fatal("serve hook should not be called for desktop")
	}
}

// TestConfigShowPrintsMergedConfig 验证 config show 会输出脱敏后的合并配置。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestConfigShowPrintsMergedConfig(t *testing.T) {
	var stdout bytes.Buffer
	err := Execute(context.Background(), Options{
		Args:   []string{"config", "show"},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			cfg := config.Default()
			cfg.Database.DSN = "postgres://taskdaemon:password@127.0.0.1/taskdaemon"
			cfg.Logging.HTTP.IncludeRequestBody = true
			return cfg, nil
		},
	})
	if err != nil {
		t.Fatalf("execute config show: %v", err)
	}
	text := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte("includeRequestBody: true")) {
		t.Fatalf("config show output = %q, want http logging config", text)
	}
	if bytes.Contains(stdout.Bytes(), []byte("postgres://taskdaemon:password")) {
		t.Fatalf("config show leaked dsn: %q", text)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("[REDACTED]")) {
		t.Fatalf("config show output = %q, want redacted marker", text)
	}
}

// TestConfigValidateLoadsConfig 验证 config validate 只要配置可加载就输出成功。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestConfigValidateLoadsConfig(t *testing.T) {
	var stdout bytes.Buffer
	var loaded bool
	err := Execute(context.Background(), Options{
		Args:   []string{"config", "validate"},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			loaded = true
			return config.Default(), nil
		},
	})
	if err != nil {
		t.Fatalf("execute config validate: %v", err)
	}
	if !loaded {
		t.Fatal("config loader should be called")
	}
	if stdout.String() != "config ok\n" {
		t.Fatalf("stdout = %q, want config ok", stdout.String())
	}
}

// TestConfigReloadCallsHookWithSession 验证 config reload 会调用 hook 并传递 session token。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestConfigReloadCallsHookWithSession(t *testing.T) {
	var stdout bytes.Buffer
	var sessionToken string
	err := Execute(context.Background(), Options{
		Args:       []string{"--session-token", "session-token", "config", "reload"},
		Stdout:     &stdout,
		Stderr:     &bytes.Buffer{},
		LoadConfig: staticConfigLoader(),
		Hooks: Hooks{
			ConfigReload: func(ctx context.Context, _ config.Config) (config.ReloadResult, error) {
				sessionToken = SessionTokenFromContext(ctx)
				return config.ReloadResult{
					Applied:         []string{"logging.http"},
					RestartRequired: []string{"server"},
				}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("execute config reload: %v", err)
	}
	if sessionToken != "session-token" {
		t.Fatalf("session token = %s, want session-token", sessionToken)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("logging.http")) || !bytes.Contains(stdout.Bytes(), []byte("server")) {
		t.Fatalf("reload output = %q, want result fields", stdout.String())
	}
}

// TestTaskTriggerCommandLoadsConfigAndRunsHook 验证 task trigger 会加载配置并调用手动触发 hook。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestTaskTriggerCommandLoadsConfigAndRunsHook(t *testing.T) {
	var triggeredID int
	var sessionToken string
	err := Execute(context.Background(), Options{
		Args:       []string{"--session-token", "session-token", "task", "trigger", "42"},
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		LoadConfig: staticConfigLoader(),
		Hooks: Hooks{
			TaskTrigger: func(ctx context.Context, _ config.Config, taskID int) error {
				triggeredID = taskID
				sessionToken = SessionTokenFromContext(ctx)
				return nil
			},
		},
	})

	if err != nil {
		t.Fatalf("execute task trigger: %v", err)
	}
	if triggeredID != 42 {
		t.Fatalf("triggered task ID = %d, want 42", triggeredID)
	}
	if sessionToken != "session-token" {
		t.Fatalf("session token = %s, want session-token", sessionToken)
	}
}

// TestTaskCancelCommandLoadsConfigAndRunsHook 验证 task cancel 会加载配置并调用取消 hook。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestTaskCancelCommandLoadsConfigAndRunsHook(t *testing.T) {
	var cancelledID int
	var sessionToken string
	err := Execute(context.Background(), Options{
		Args:       []string{"--session-token", "session-token", "task", "cancel", "42"},
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		LoadConfig: staticConfigLoader(),
		Hooks: Hooks{
			TaskCancel: func(ctx context.Context, _ config.Config, taskID int) error {
				cancelledID = taskID
				sessionToken = SessionTokenFromContext(ctx)
				return nil
			},
		},
	})

	if err != nil {
		t.Fatalf("execute task cancel: %v", err)
	}
	if cancelledID != 42 {
		t.Fatalf("cancelled task ID = %d, want 42", cancelledID)
	}
	if sessionToken != "session-token" {
		t.Fatalf("session token = %s, want session-token", sessionToken)
	}
}

// TestTaskCancelCommandReadsSessionTokenFromEnv 验证 task cancel 可从环境变量读取 session token。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestTaskCancelCommandReadsSessionTokenFromEnv(t *testing.T) {
	t.Setenv("TASKDAEMON_SESSION_TOKEN", "env-session-token")
	var sessionToken string
	err := Execute(context.Background(), Options{
		Args:       []string{"task", "cancel", "42"},
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		LoadConfig: staticConfigLoader(),
		Hooks: Hooks{
			TaskCancel: func(ctx context.Context, _ config.Config, _ int) error {
				sessionToken = SessionTokenFromContext(ctx)
				return nil
			},
		},
	})

	if err != nil {
		t.Fatalf("execute task cancel: %v", err)
	}
	if sessionToken != "env-session-token" {
		t.Fatalf("session token = %s, want env-session-token", sessionToken)
	}
}

// TestTaskRunsCommandLoadsConfigAndPrintsHistory 验证 task runs 会加载配置、传递 session 并输出执行历史。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestTaskRunsCommandLoadsConfigAndPrintsHistory(t *testing.T) {
	var stdout bytes.Buffer
	var runsTaskID int
	var sessionToken string
	err := Execute(context.Background(), Options{
		Args:       []string{"--session-token", "session-token", "task", "runs", "42"},
		Stdout:     &stdout,
		Stderr:     &bytes.Buffer{},
		LoadConfig: staticConfigLoader(),
		Hooks: Hooks{
			TaskRuns: func(ctx context.Context, _ config.Config, taskID int) ([]TaskRunSummary, error) {
				runsTaskID = taskID
				sessionToken = SessionTokenFromContext(ctx)
				return []TaskRunSummary{
					{ID: 11, Trigger: "manual", Status: "success", ExitCode: intPtr(0), DurationMs: 1200, Stdout: "ok"},
				}, nil
			},
		},
	})

	if err != nil {
		t.Fatalf("execute task runs: %v", err)
	}
	if runsTaskID != 42 {
		t.Fatalf("runs task ID = %d, want 42", runsTaskID)
	}
	if sessionToken != "session-token" {
		t.Fatalf("session token = %s, want session-token", sessionToken)
	}
	output := stdout.String()
	for _, want := range []string{"id: 11", "trigger: manual", "status: success", "exitCode: 0", "stdout: ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("task runs output = %q, want %q", output, want)
		}
	}
}

// TestSessionTokenFromContextPrefersFlag 验证 flag token 优先于环境变量。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestSessionTokenFromContextPrefersFlag(t *testing.T) {
	t.Setenv("TASKDAEMON_SESSION_TOKEN", "env-session-token")
	ctx := withSessionToken(context.Background(), "flag-session-token")

	if got := SessionTokenFromContext(ctx); got != "flag-session-token" {
		t.Fatalf("session token = %s, want flag-session-token", got)
	}
}

// TestSessionTokenFromContextReturnsEmptyWhenUnset 验证未配置时不伪造 session token。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestSessionTokenFromContextReturnsEmptyWhenUnset(t *testing.T) {
	t.Setenv("TASKDAEMON_SESSION_TOKEN", "")

	if got := SessionTokenFromContext(context.Background()); got != "" {
		t.Fatalf("session token = %s, want empty", got)
	}
}

// TestServiceInstallDryRunHasNoSideEffects 验证 service install --dry-run 打印单元且不要求 hooks。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestServiceInstallDryRunHasNoSideEffects(t *testing.T) {
	stdout := &bytes.Buffer{}
	err := Execute(context.Background(), Options{
		Args:       []string{"service", "install", "--dry-run", "--config", "/tmp/taskdaemon-dry-run-config.yaml"},
		Stdout:     stdout,
		Stderr:     &bytes.Buffer{},
		LoadConfig: staticConfigLoader(),
	})
	if err != nil {
		t.Fatalf("dry-run install: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "将写入:") {
		t.Fatalf("missing unit path header: %s", out)
	}
	if !strings.Contains(out, "/tmp/taskdaemon-dry-run-config.yaml") {
		t.Fatalf("missing config path: %s", out)
	}
	if !strings.Contains(out, "--- 单元内容 ---") {
		t.Fatalf("missing unit body: %s", out)
	}
}

// TestServiceStatusCommandRuns 验证 service status 子命令可执行。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestServiceStatusCommandRuns(t *testing.T) {
	stdout := &bytes.Buffer{}
	err := Execute(context.Background(), Options{
		Args:       []string{"service", "status"},
		Stdout:     stdout,
		Stderr:     &bytes.Buffer{},
		LoadConfig: staticConfigLoader(),
	})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(stdout.String(), "platform:") && !strings.Contains(stdout.String(), "installed:") {
		t.Fatalf("unexpected status output: %s", stdout.String())
	}
}

// staticConfigLoader 返回测试使用的静态配置加载函数。
//
// 参数:
//   - 无。
//
// 返回值:
//   - func(config.LoadOptions) (config.Config, error): 忽略输入并返回默认配置的加载函数。
func staticConfigLoader() func(config.LoadOptions) (config.Config, error) {
	return func(config.LoadOptions) (config.Config, error) {
		return config.Default(), nil
	}
}

// intPtr 返回整数指针，便于测试构造可选退出码。
//
// 参数:
//   - value: 整数值。
//
// 返回值:
//   - *int: 指向 value 的指针。
func intPtr(value int) *int {
	return &value
}
