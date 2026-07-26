package scheduler

import (
	"math"
	"strconv"
	"time"

	"taskdaemon/internal/data/ent"
	"taskdaemon/internal/runner"
)

// runnerConfigToJSON 将 runner.Config 转换为可写入 Ent JSON 字段的 map。
//
// 参数:
//   - cfg: runner 配置。
//
// 返回值:
//   - map[string]any: JSON 友好的 runner 配置。
func runnerConfigToJSON(cfg runner.Config) map[string]any {
	out := map[string]any{
		"type":             string(cfg.Type),
		"inline":           cfg.Inline,
		"scriptPath":       cfg.ScriptPath,
		"args":             cfg.Args,
		"workDir":          cfg.WorkDir,
		"env":              cfg.Env,
		"timeoutSeconds":   int(math.Ceil(cfg.Timeout.Seconds())),
		"outputLimitBytes": cfg.OutputLimitBytes,
	}
	if cfg.Type == runner.TypeAgent {
		out["runtimeId"] = cfg.AgentRuntimeID
		out["profile"] = cfg.AgentProfile
		out["inputText"] = cfg.AgentInputText
		out["threadId"] = cfg.AgentThreadID
	}
	return out
}

// withCronWarnings 在 runner JSON 中附加 cron 软警告，便于后续展示和排查。
//
// 参数:
//   - cfg: runner JSON 配置。
//   - warnings: cron 软警告列表。
//
// 返回值:
//   - map[string]any: 包含 warnings 的配置。
func withCronWarnings(cfg map[string]any, warnings []string) map[string]any {
	if len(warnings) > 0 {
		cfg["cronWarnings"] = warnings
	}
	return cfg
}

// runnerConfigFromTask 从任务定义恢复 runner.Config。
//
// 参数:
//   - taskRecord: Ent 任务记录。
//
// 返回值:
//   - runner.Config: runner 执行配置。
//   - error: 配置类型转换失败时返回错误。
func runnerConfigFromTask(taskRecord *ent.Task) (runner.Config, error) {
	cfg := runner.Config{
		Type:    runner.Type(taskRecord.RunnerType),
		Timeout: time.Duration(taskRecord.TimeoutSeconds) * time.Second,
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "inline"); ok {
		cfg.Inline = value
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "scriptPath"); ok {
		cfg.ScriptPath = value
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "workDir"); ok {
		cfg.WorkDir = value
	}
	if value, ok := intValue(taskRecord.RunnerConfig, "outputLimitBytes"); ok {
		cfg.OutputLimitBytes = value
	}
	if args, ok := stringSliceValue(taskRecord.RunnerConfig, "args"); ok {
		cfg.Args = args
	}
	if env, ok := stringMapValue(taskRecord.RunnerConfig, "env"); ok {
		cfg.Env = env
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "runtimeId"); ok {
		cfg.AgentRuntimeID = value
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "profile"); ok {
		cfg.AgentProfile = value
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "inputText"); ok {
		cfg.AgentInputText = value
	}
	if value, ok := stringValue(taskRecord.RunnerConfig, "threadId"); ok {
		cfg.AgentThreadID = value
	}
	// 环路深度可从 env 注入（agent 触发的任务）
	if cfg.Env != nil {
		if depthRaw, ok := cfg.Env["TASKDAEMON_AGENT_LOOP_DEPTH"]; ok {
			if n, err := strconv.Atoi(depthRaw); err == nil {
				cfg.AgentLoopDepth = n
			}
		}
		if tr, ok := cfg.Env["TASKDAEMON_TRACE_ID"]; ok {
			cfg.AgentTraceID = tr
		}
	}
	return cfg, nil
}

// stringValue 从 JSON map 中读取字符串。
//
// 参数:
//   - values: JSON map。
//   - key: 字段名。
//
// 返回值:
//   - string: 字段值。
//   - bool: true 表示字段存在且类型正确。
func stringValue(values map[string]any, key string) (string, bool) {
	value, ok := values[key].(string)
	return value, ok
}

// intValue 从 JSON map 中读取整数。
//
// 参数:
//   - values: JSON map。
//   - key: 字段名。
//
// 返回值:
//   - int: 字段值。
//   - bool: true 表示字段存在且类型可转换。
func intValue(values map[string]any, key string) (int, bool) {
	switch value := values[key].(type) {
	case int:
		return value, true
	case float64:
		return int(value), true
	default:
		return 0, false
	}
}

// stringSliceValue 从 JSON map 中读取字符串切片。
//
// 参数:
//   - values: JSON map。
//   - key: 字段名。
//
// 返回值:
//   - []string: 字段值。
//   - bool: true 表示字段存在且可转换。
func stringSliceValue(values map[string]any, key string) ([]string, bool) {
	raw, ok := values[key].([]any)
	if !ok {
		if typed, ok := values[key].([]string); ok {
			return typed, true
		}
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

// stringMapValue 从 JSON map 中读取字符串 map。
//
// 参数:
//   - values: JSON map。
//   - key: 字段名。
//
// 返回值:
//   - map[string]string: 字段值。
//   - bool: true 表示字段存在且可转换。
func stringMapValue(values map[string]any, key string) (map[string]string, bool) {
	if typed, ok := values[key].(map[string]string); ok {
		return typed, true
	}
	raw, ok := values[key].(map[string]any)
	if !ok {
		return nil, false
	}
	out := make(map[string]string, len(raw))
	for key, value := range raw {
		text, ok := value.(string)
		if !ok {
			return nil, false
		}
		out[key] = text
	}
	return out, true
}
