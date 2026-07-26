package app

import (
	"context"
	"fmt"

	"taskdaemon/internal/config"
)

// ConfigSubsystemStatus 描述写入后单个子系统应用结果。
type ConfigSubsystemStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ConfigSectionWriteResult 配置 section 写入编排结果。
type ConfigSectionWriteResult struct {
	Config          config.Config
	Applied         []string
	RestartRequired []string
	Reload          config.ReloadResult
	Subsystems      []ConfigSubsystemStatus
}

// WriteAudioConfig 校验、落盘并热应用 audio section 部分更新。
//
// 参数:
//   - ctx: 请求上下文。
//   - patch: audio 部分更新。
//
// 返回值:
//   - ConfigSectionWriteResult: 写入后的有效配置与 reload 观测。
//   - error: 校验/落盘/重载失败。
func (app *App) WriteAudioConfig(ctx context.Context, patch config.AudioSectionPatch) (ConfigSectionWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return ConfigSectionWriteResult{}, err
	}
	if app == nil {
		return ConfigSectionWriteResult{}, &config.WriteError{Code: config.CodeWriteFailed, Message: "app is nil"}
	}

	loader := app.loadConfig
	if loader == nil {
		loader = config.Load
	}
	current, err := loader(app.loadOptions)
	if err != nil {
		return ConfigSectionWriteResult{}, &config.WriteError{Code: config.CodeWriteFailed, Message: "load current config failed", Err: err}
	}

	plan, err := config.BuildAudioWritePlan(current.Audio, patch)
	if err != nil {
		return ConfigSectionWriteResult{}, err
	}
	if len(plan.Applied) == 0 && len(plan.RestartRequired) == 0 {
		return ConfigSectionWriteResult{
			Config: current,
			Subsystems: []ConfigSubsystemStatus{
				{Name: "runtime", Status: "skipped"},
				{Name: "audio", Status: "skipped"},
			},
		}, nil
	}

	writePath, err := config.ResolveWritePath(app.loadOptions)
	if err != nil {
		return ConfigSectionWriteResult{}, err
	}

	previous := current
	backup, err := config.PersistSectionMap(writePath, "audio", plan.FileAudioMap)
	if err != nil {
		return ConfigSectionWriteResult{}, err
	}

	reloaded, loadErr := loader(app.loadOptions)
	if loadErr != nil {
		_ = config.RestoreFileContent(writePath, backup)
		return ConfigSectionWriteResult{}, &config.WriteError{
			Code:    config.CodeReloadFailed,
			Message: "reload after write failed; file restored",
			Err:     loadErr,
		}
	}

	subsystems := app.applyConfigSnapshot(reloaded)
	for _, item := range subsystems {
		if item.Status == "failed" {
			_ = config.RestoreFileContent(writePath, backup)
			app.applyConfigSnapshot(previous)
			return ConfigSectionWriteResult{}, &config.WriteError{
				Code:    config.CodeReloadFailed,
				Message: "apply after write failed; runtime and file restored",
			}
		}
	}

	app.cfg = reloaded
	if app.logger != nil {
		app.logger.Info("config section written",
			"section", "audio",
			"path", writePath,
			"applied", plan.Applied,
			"restart_required", plan.RestartRequired,
		)
	}

	return ConfigSectionWriteResult{
		Config:          reloaded,
		Applied:         plan.Applied,
		RestartRequired: plan.RestartRequired,
		Reload: config.ReloadResult{
			Applied:         plan.Applied,
			RestartRequired: plan.RestartRequired,
		},
		Subsystems: subsystems,
	}, nil
}

// applyConfigSnapshot 将配置应用到 runtime 与已装配子系统。
//
// 参数:
//   - cfg: 完整配置。
//
// 返回值:
//   - []ConfigSubsystemStatus: 各子系统状态。
func (app *App) applyConfigSnapshot(cfg config.Config) []ConfigSubsystemStatus {
	statuses := make([]ConfigSubsystemStatus, 0, 3)
	if app.runtimeConfig != nil {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					statuses = append(statuses, ConfigSubsystemStatus{
						Name:   "runtime",
						Status: "failed",
						Error:  fmt.Sprint(recovered),
					})
				}
			}()
			_ = app.runtimeConfig.Apply(cfg)
			statuses = append(statuses, ConfigSubsystemStatus{Name: "runtime", Status: "ok"})
		}()
	} else {
		statuses = append(statuses, ConfigSubsystemStatus{Name: "runtime", Status: "skipped"})
	}

	if app.audioService != nil {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					statuses = append(statuses, ConfigSubsystemStatus{
						Name:   "audio",
						Status: "failed",
						Error:  fmt.Sprint(recovered),
					})
				}
			}()
			app.audioService.UpdateConfig(cfg.Audio)
			if app.audioQueue != nil {
				app.audioQueue.UpdateConfig(cfg.Audio)
			}
			statuses = append(statuses, ConfigSubsystemStatus{Name: "audio", Status: "ok"})
		}()
	} else {
		statuses = append(statuses, ConfigSubsystemStatus{Name: "audio", Status: "skipped"})
	}

	// T2a
	if app.notifyBus != nil {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					statuses = append(statuses, ConfigSubsystemStatus{
						Name:   "notify",
						Status: "failed",
						Error:  fmt.Sprint(recovered),
					})
				}
			}()
			app.notifyBus.UpdateConfig(cfg.Notify)
			statuses = append(statuses, ConfigSubsystemStatus{Name: "notify", Status: "ok"})
		}()
	}

	return statuses
}
