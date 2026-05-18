package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	"taskdaemon/internal/config"
)

// printConfigPaths 输出配置文件解析结果。
//
// 参数:
//   - writer: 输出 writer。
//   - paths: 配置路径解析结果。
//
// 返回值:
//   - 无。
func printConfigPaths(writer io.Writer, paths config.ResolvedPaths) {
	fmt.Fprintf(writer, "config: %s\n", paths.ConfigPath)
	fmt.Fprintf(writer, "configExists: %t\n", paths.ConfigExists)
	fmt.Fprintf(writer, "localConfig: %s\n", paths.LocalConfigPath)
	fmt.Fprintf(writer, "localConfigExists: %t\n", paths.LocalConfigExists)
	fmt.Fprintf(writer, "explicitConfig: %t\n", paths.ExplicitConfigPath)
}

// printYAML 将结构化值输出为 YAML。
//
// 参数:
//   - writer: 输出 writer。
//   - value: 待输出值。
//
// 返回值:
//   - error: YAML 编码失败或写入失败时返回错误。
func printYAML(writer io.Writer, value any) error {
	content, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	_, err = writer.Write(content)
	return err
}

// sanitizedConfig 返回适合 CLI 展示的脱敏配置。
//
// 参数:
//   - cfg: 应用配置。
//
// 返回值:
//   - map[string]any: 可序列化的脱敏配置。
func sanitizedConfig(cfg config.Config) map[string]any {
	return map[string]any{
		"server": map[string]any{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
			"swagger": map[string]any{
				"enabled": cfg.Server.Swagger.Enabled,
			},
		},
		"database": map[string]any{
			"driver": cfg.Database.Driver,
			"dsn":    redactConfigValue(cfg.Database.DSN),
		},
		"logging": map[string]any{
			"level":  cfg.Logging.Level,
			"output": cfg.Logging.Output,
			"console": map[string]any{
				"format": cfg.Logging.Console.Format,
			},
			"file": map[string]any{
				"path":       cfg.Logging.File.Path,
				"format":     cfg.Logging.File.Format,
				"maxSizeMB":  cfg.Logging.File.MaxSizeMB,
				"maxBackups": cfg.Logging.File.MaxBackups,
				"maxAgeDays": cfg.Logging.File.MaxAgeDays,
				"compress":   cfg.Logging.File.Compress,
			},
			"http": map[string]any{
				"includeRequestBody":  cfg.Logging.HTTP.IncludeRequestBody,
				"includeResponseBody": cfg.Logging.HTTP.IncludeResponseBody,
				"maxBodyBytes":        cfg.Logging.HTTP.MaxBodyBytes,
				"redactFields":        cfg.Logging.HTTP.RedactFields,
			},
		},
		"observability": map[string]any{
			"traceId": map[string]any{
				"includeInResponse": cfg.Observability.TraceID.IncludeInResponse,
			},
		},
	}
}

// redactConfigValue 脱敏配置展示中的敏感字符串。
//
// 参数:
//   - value: 原始配置值。
//
// 返回值:
//   - string: 脱敏后的配置值。
func redactConfigValue(value string) string {
	if value == "" {
		return value
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "password") || strings.Contains(lower, "://") && strings.Contains(value, "@") {
		return "[REDACTED]"
	}
	return value
}

// marshalConfigJSON 将配置转为 JSON 字符串，供测试断言使用。
//
// 参数:
//   - value: 待序列化值。
//
// 返回值:
//   - string: JSON 字符串；序列化失败时为空。
func marshalConfigJSON(value any) string {
	content, _ := json.Marshal(value)
	return string(content)
}

// writerOrDefault 返回 writer 或默认 writer。
//
// 参数:
//   - writer: 调用方提供的 writer。
//   - fallback: writer 为空时使用的默认 writer。
//
// 返回值:
//   - io.Writer: 可用于命令输出的 writer。
func writerOrDefault(writer io.Writer, fallback io.Writer) io.Writer {
	if writer != nil {
		return writer
	}
	return fallback
}
