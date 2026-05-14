package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultPathUsesUserConfigDir 验证默认配置路径位于用户配置目录下的 taskdaemon/config.yaml。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestDefaultPathUsesUserConfigDir(t *testing.T) {
	configPath, err := DefaultPath()
	if err != nil {
		t.Fatalf("default path: %v", err)
	}

	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("user config dir: %v", err)
	}
	want := filepath.Join(userConfigDir, "taskdaemon", "config.yaml")
	if configPath != want {
		t.Fatalf("default path = %s, want %s", configPath, want)
	}
}

// TestLoadMergesDefaultsFileEnvAndOverrides 验证配置覆盖顺序为 defaults < file < env < overrides。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestLoadMergesDefaultsFileEnvAndOverrides(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`
server:
  host: 127.0.0.1
  port: 9090
  swagger:
    enabled: true
database:
  driver: postgres
  dsn: postgres://example
`)
	if err := os.WriteFile(configFile, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("TASKDAEMON_SERVER_PORT", "7070")
	t.Setenv("TASKDAEMON_DATABASE_DSN", "postgres://env")

	cfg, err := Load(LoadOptions{
		ConfigPath: configFile,
		Overrides: map[string]any{
			"server.host": "0.0.0.0",
		},
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("server host = %s, want override", cfg.Server.Host)
	}
	if cfg.Server.Port != 7070 {
		t.Fatalf("server port = %d, want env override", cfg.Server.Port)
	}
	if !cfg.Server.Swagger.Enabled {
		t.Fatal("swagger should be enabled from config file")
	}
	if cfg.Database.Driver != "postgres" {
		t.Fatalf("database driver = %s, want postgres", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "postgres://env" {
		t.Fatalf("database dsn = %s, want env override", cfg.Database.DSN)
	}
}

// TestLoadDefaultsWhenConfigFileIsMissing 验证未显式指定配置文件且默认文件不存在时使用默认值。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestLoadDefaultsWhenConfigFileIsMissing(t *testing.T) {
	cfg, err := Load(LoadOptions{
		ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"),
		Optional:   true,
	})
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("server host = %s, want default", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("server port = %d, want default", cfg.Server.Port)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("database driver = %s, want sqlite", cfg.Database.Driver)
	}
}
