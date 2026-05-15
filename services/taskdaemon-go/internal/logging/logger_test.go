package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskdaemon/internal/config"
)

// TestNewWritesTextConsoleAndJSONFile 验证 console 和 file 可以使用不同格式输出同一条日志。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestNewWritesTextConsoleAndJSONFile(t *testing.T) {
	var console bytes.Buffer
	logPath := filepath.Join(t.TempDir(), "taskdaemon.log")
	cfg := config.Default().Logging
	cfg.Output = "both"
	cfg.Console.Format = "text"
	cfg.File.Path = logPath
	cfg.File.Format = "json"

	logger, err := New(cfg, Options{ConsoleWriter: &console})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()
	logger.Info("hello", "trace_id", "trace-1")

	if !strings.Contains(console.String(), "trace_id=trace-1") {
		t.Fatalf("console log = %q, want text trace_id field", console.String())
	}
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	var row map[string]any
	if err := json.Unmarshal(content, &row); err != nil {
		t.Fatalf("decode json log: %v", err)
	}
	if row["trace_id"] != "trace-1" {
		t.Fatalf("trace_id = %v, want trace-1", row["trace_id"])
	}
}

// TestNewHonorsLogLevel 验证日志级别会过滤低于配置的日志。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestNewHonorsLogLevel(t *testing.T) {
	var console bytes.Buffer
	cfg := config.Default().Logging
	cfg.Level = "warn"
	cfg.Output = "console"

	logger, err := New(cfg, Options{ConsoleWriter: &console})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()
	logger.Info("hidden")
	logger.Warn("shown")

	text := console.String()
	if strings.Contains(text, "hidden") {
		t.Fatalf("console log = %q, should filter info", text)
	}
	if !strings.Contains(text, "shown") {
		t.Fatalf("console log = %q, want warn message", text)
	}
}

// TestNewRejectsUnknownLevel 验证非法日志级别会 fail fast。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestNewRejectsUnknownLevel(t *testing.T) {
	cfg := config.Default().Logging
	cfg.Level = "verbose"

	if _, err := New(cfg, Options{}); err == nil {
		t.Fatal("unknown logging level should fail")
	}
}

// TestFanoutHandlerPreservesAttrs 验证 fan-out handler 会保留 WithAttrs 附加字段。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestFanoutHandlerPreservesAttrs(t *testing.T) {
	var console bytes.Buffer
	logger := slog.New(newFanoutHandler(slog.NewTextHandler(&console, nil))).With("component", "test")

	logger.Info("hello")

	if !strings.Contains(console.String(), "component=test") {
		t.Fatalf("console log = %q, want component attr", console.String())
	}
}
