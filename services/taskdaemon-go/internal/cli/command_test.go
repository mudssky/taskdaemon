package cli

import (
	"bytes"
	"context"
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
	var served bool
	hooks := Hooks{
		Serve: func(_ context.Context, cfg config.Config) error {
			served = true
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
		Serve: func(context.Context, config.Config) error {
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
