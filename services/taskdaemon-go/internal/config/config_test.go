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
logging:
  level: debug
  output: file
  file:
    path: ./file.log
    maxSizeMB: 10
observability:
  traceId:
    includeInResponse: false
`)
	if err := os.WriteFile(configFile, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("TASKDAEMON_SERVER_PORT", "7070")
	t.Setenv("TASKDAEMON_DATABASE_DSN", "postgres://env")
	t.Setenv("TASKDAEMON_LOGGING_FILE_MAX_SIZE_MB", "20")
	t.Setenv("TASKDAEMON_OBSERVABILITY_TRACE_ID_INCLUDE_IN_RESPONSE", "true")

	cfg, err := Load(LoadOptions{
		ConfigPath: configFile,
		Overrides: map[string]any{
			"server.host":    "0.0.0.0",
			"logging.output": "both",
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
	if cfg.Logging.Level != "debug" {
		t.Fatalf("logging level = %s, want file config", cfg.Logging.Level)
	}
	if cfg.Logging.Output != "both" {
		t.Fatalf("logging output = %s, want override", cfg.Logging.Output)
	}
	if cfg.Logging.File.MaxSizeMB != 20 {
		t.Fatalf("logging file max size = %d, want env alias override", cfg.Logging.File.MaxSizeMB)
	}
	if cfg.Logging.HTTP.IncludeRequestBody {
		t.Fatal("request body logging should be disabled by default")
	}
	if cfg.Logging.HTTP.IncludeResponseBody {
		t.Fatal("response body logging should be disabled by default")
	}
	if cfg.Logging.HTTP.MaxBodyBytes != 4096 {
		t.Fatalf("http max body bytes = %d, want default 4096", cfg.Logging.HTTP.MaxBodyBytes)
	}
	if len(cfg.Logging.HTTP.RedactFields) == 0 {
		t.Fatal("http redact fields should have safe defaults")
	}
	if !cfg.Observability.TraceID.IncludeInResponse {
		t.Fatal("trace id response flag should be overridden by env alias")
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
	if cfg.Server.Port != 39245 {
		t.Fatalf("server port = %d, want default", cfg.Server.Port)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("database driver = %s, want sqlite", cfg.Database.Driver)
	}
}

// TestLoadUsesProjectConfigWhenNoExplicitPath 验证工作区内未显式指定配置时会自动读取项目内配置文件。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestLoadUsesProjectConfigWhenNoExplicitPath(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "pnpm-workspace.yaml"), []byte("packages:\n  - apps/*\n"), 0o600); err != nil {
		t.Fatalf("write workspace marker: %v", err)
	}
	configFile := filepath.Join(projectDir, "taskdaemon.yaml")
	content := []byte(`
server:
  port: 9191
database:
  driver: postgres
  dsn: postgres://project
`)
	if err := os.WriteFile(configFile, content, 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	cfg, err := Load(LoadOptions{Optional: true})
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}

	if cfg.Server.Port != 9191 {
		t.Fatalf("server port = %d, want project config", cfg.Server.Port)
	}
	if cfg.Database.Driver != "postgres" {
		t.Fatalf("database driver = %s, want postgres", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "postgres://project" {
		t.Fatalf("database dsn = %s, want project config", cfg.Database.DSN)
	}
}

// TestLoadMergesProjectLocalConfigBeforeEnv 验证开发工作区会在基础项目配置后叠加本地配置，且环境变量仍优先。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestLoadMergesProjectLocalConfigBeforeEnv(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "pnpm-workspace.yaml"), []byte("packages:\n  - apps/*\n"), 0o600); err != nil {
		t.Fatalf("write workspace marker: %v", err)
	}
	baseConfig := []byte(`
server:
  port: 9191
logging:
  level: info
  http:
    includeRequestBody: false
    maxBodyBytes: 128
`)
	if err := os.WriteFile(filepath.Join(projectDir, "taskdaemon.yaml"), baseConfig, 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	localConfig := []byte(`
server:
  port: 9292
logging:
  level: debug
  http:
    includeRequestBody: true
    includeResponseBody: true
    maxBodyBytes: 256
    redactFields:
      - password
      - apiKey
`)
	if err := os.WriteFile(filepath.Join(projectDir, "taskdaemon.local.yaml"), localConfig, 0o600); err != nil {
		t.Fatalf("write local config: %v", err)
	}
	t.Setenv("TASKDAEMON_SERVER_PORT", "9393")
	t.Setenv("TASKDAEMON_LOGGING_HTTP_MAX_BODY_BYTES", "512")

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	cfg, err := Load(LoadOptions{Optional: true})
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}

	if cfg.Server.Port != 9393 {
		t.Fatalf("server port = %d, want env override", cfg.Server.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("logging level = %s, want local config", cfg.Logging.Level)
	}
	if !cfg.Logging.HTTP.IncludeRequestBody {
		t.Fatal("request body logging should be enabled from local config")
	}
	if !cfg.Logging.HTTP.IncludeResponseBody {
		t.Fatal("response body logging should be enabled from local config")
	}
	if cfg.Logging.HTTP.MaxBodyBytes != 512 {
		t.Fatalf("http max body bytes = %d, want env override", cfg.Logging.HTTP.MaxBodyBytes)
	}
	if len(cfg.Logging.HTTP.RedactFields) != 2 || cfg.Logging.HTTP.RedactFields[1] != "apiKey" {
		t.Fatalf("redact fields = %#v, want local config list", cfg.Logging.HTTP.RedactFields)
	}
}

// TestLoadSkipsProjectConfigOutsideWorkspace 验证非工作区环境不会自动读取项目内配置文件。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestLoadSkipsProjectConfigOutsideWorkspace(t *testing.T) {
	projectDir := t.TempDir()
	configFile := filepath.Join(projectDir, "taskdaemon.yaml")
	content := []byte(`
server:
  port: 9494
database:
  driver: postgres
  dsn: postgres://project
`)
	if err := os.WriteFile(configFile, content, 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	t.Setenv("APPDATA", filepath.Join(t.TempDir(), "appdata"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	cfg, err := Load(LoadOptions{Optional: true})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Port != 39245 {
		t.Fatalf("server port = %d, want default", cfg.Server.Port)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("database driver = %s, want default", cfg.Database.Driver)
	}
}

// TestProjectPathFindsFirstProjectConfig 验证项目内配置文件会按约定顺序发现。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestProjectPathFindsFirstProjectConfig(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "pnpm-workspace.yaml"), []byte("packages:\n  - apps/*\n"), 0o600); err != nil {
		t.Fatalf("write workspace marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "config.yml"), []byte("server:\n  port: 9393\n"), 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	path, found, err := ProjectPath()
	if err != nil {
		t.Fatalf("project path: %v", err)
	}
	if !found {
		t.Fatal("project config should be found")
	}
	if path != filepath.Join(projectDir, "config.yml") {
		t.Fatalf("project path = %s, want config.yml", path)
	}
}

// TestProjectLocalPathFindsFirstLocalConfig 验证项目本地配置文件会按约定顺序发现。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestProjectLocalPathFindsFirstLocalConfig(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "pnpm-workspace.yaml"), []byte("packages:\n  - apps/*\n"), 0o600); err != nil {
		t.Fatalf("write workspace marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "config.local.yml"), []byte("server:\n  port: 9393\n"), 0o600); err != nil {
		t.Fatalf("write local config: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	path, found, err := ProjectLocalPath()
	if err != nil {
		t.Fatalf("project local path: %v", err)
	}
	if !found {
		t.Fatal("project local config should be found")
	}
	if path != filepath.Join(projectDir, "config.local.yml") {
		t.Fatalf("project local path = %s, want config.local.yml", path)
	}
}

// TestProjectPathReturnsMissingWhenNoProjectConfig 验证当前目录没有项目内配置时返回未找到。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestProjectPathReturnsMissingWhenNoProjectConfig(t *testing.T) {
	projectDir := t.TempDir()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	path, found, err := ProjectPath()
	if err != nil {
		t.Fatalf("project path: %v", err)
	}
	if found {
		t.Fatalf("project config path = %s, want missing", path)
	}
}

// TestExplicitConfigPathIgnoresProjectConfig 验证 --config 显式路径优先，不会再叠加项目内配置。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestExplicitConfigPathIgnoresProjectConfig(t *testing.T) {
	projectDir := t.TempDir()
	projectConfig := []byte(`
server:
  port: 9191
database:
  dsn: postgres://project
`)
	if err := os.WriteFile(filepath.Join(projectDir, "taskdaemon.yaml"), projectConfig, 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	localConfig := []byte(`
server:
  port: 9393
database:
  dsn: postgres://local
`)
	if err := os.WriteFile(filepath.Join(projectDir, "taskdaemon.local.yaml"), localConfig, 0o600); err != nil {
		t.Fatalf("write local config: %v", err)
	}

	explicitConfig := filepath.Join(t.TempDir(), "custom.yaml")
	explicitContent := []byte(`
server:
  port: 9292
database:
  dsn: postgres://explicit
`)
	if err := os.WriteFile(explicitConfig, explicitContent, 0o600); err != nil {
		t.Fatalf("write explicit config: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	cfg, err := Load(LoadOptions{ConfigPath: explicitConfig})
	if err != nil {
		t.Fatalf("load explicit config: %v", err)
	}

	if cfg.Server.Port != 9292 {
		t.Fatalf("server port = %d, want explicit config", cfg.Server.Port)
	}
	if cfg.Database.DSN != "postgres://explicit" {
		t.Fatalf("database dsn = %s, want explicit config", cfg.Database.DSN)
	}
}
