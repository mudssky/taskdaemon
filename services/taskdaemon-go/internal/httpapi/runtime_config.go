package httpapi

import (
	"sync"

	"taskdaemon/internal/config"
)

// RuntimeConfig 保存可在 daemon 运行时热更新的 HTTP API 配置。
type RuntimeConfig struct {
	mu                     sync.RWMutex
	loggingHTTP            config.LoggingHTTPConfig
	includeTraceInResponse bool
}

// NewRuntimeConfig 创建 HTTP API 运行时配置。
//
// 参数:
//   - cfg: 初始应用配置。
//
// 返回值:
//   - *RuntimeConfig: 可被 router 和 reload 逻辑共享的运行时配置。
func NewRuntimeConfig(cfg config.Config) *RuntimeConfig {
	return &RuntimeConfig{
		loggingHTTP:            cfg.Logging.HTTP,
		includeTraceInResponse: cfg.Observability.TraceID.IncludeInResponse,
	}
}

// HTTPLog 返回当前 HTTP 请求日志配置快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - config.LoggingHTTPConfig: 当前 HTTP 请求日志配置。
func (runtime *RuntimeConfig) HTTPLog() config.LoggingHTTPConfig {
	if runtime == nil {
		return config.Default().Logging.HTTP
	}
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.loggingHTTP
}

// IncludeTraceInResponse 返回当前响应体 traceId 开关。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: true 表示响应 body 包含 traceId。
func (runtime *RuntimeConfig) IncludeTraceInResponse() bool {
	if runtime == nil {
		return true
	}
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.includeTraceInResponse
}

// Apply 更新可热重载的运行时配置。
//
// 参数:
//   - cfg: 新加载的应用配置。
//
// 返回值:
//   - config.ReloadResult: 已应用和需要重启的配置分组。
func (runtime *RuntimeConfig) Apply(cfg config.Config) config.ReloadResult {
	if runtime == nil {
		return config.ReloadResult{}
	}
	runtime.mu.Lock()
	runtime.loggingHTTP = cfg.Logging.HTTP
	runtime.includeTraceInResponse = cfg.Observability.TraceID.IncludeInResponse
	runtime.mu.Unlock()
	return config.ReloadResult{
		Applied:         []string{"logging.http", "observability.traceId"},
		RestartRequired: []string{"server", "database"},
	}
}
