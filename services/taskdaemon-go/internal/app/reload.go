package app

import (
	"context"
	"net/http"
	"time"

	"taskdaemon/internal/config"
)

// ReloadRuntimeConfig 重新加载配置文件并应用可热更新配置。
//
// 参数:
//   - ctx: 控制配置加载生命周期的 context。
//
// 返回值:
//   - config.ReloadResult: 已应用和需要重启的配置分组。
//   - error: 配置加载失败时返回错误。
func (app *App) ReloadRuntimeConfig(ctx context.Context) (config.ReloadResult, error) {
	if err := ctx.Err(); err != nil {
		return config.ReloadResult{}, err
	}
	loader := app.loadConfig
	if loader == nil {
		loader = config.Load
	}
	cfg, err := loader(app.loadOptions)
	if err != nil {
		return config.ReloadResult{}, err
	}
	result := app.runtimeConfig.Apply(cfg)
	if app.audioService != nil {
		app.audioService.UpdateConfig(cfg.Audio)
	}
	if app.audioQueue != nil {
		app.audioQueue.UpdateConfig(cfg.Audio)
	}
	app.logger.Info("runtime config reloaded", "applied", result.Applied, "restart_required", result.RestartRequired)
	return result, nil
}

// ReloadDaemonConfig 调用正在运行的 daemon API 重载配置。
//
// 参数:
//   - ctx: 控制 HTTP 请求生命周期的 context。
//   - sessionToken: 管理员 session token，会作为 session cookie 发送。
//
// 返回值:
//   - config.ReloadResult: daemon 返回的重载结果。
//   - error: token 缺失、请求失败或 daemon 返回非成功状态时返回错误。
func (app *App) ReloadDaemonConfig(ctx context.Context, sessionToken string) (config.ReloadResult, error) {
	var result config.ReloadResult
	if err := app.callDaemonAPI(ctx, http.MethodPost, "config/reload", sessionToken, nil, &result, 10*time.Second, http.StatusOK); err != nil {
		return config.ReloadResult{}, err
	}
	app.logger.Info("config reload requested")
	return result, nil
}
