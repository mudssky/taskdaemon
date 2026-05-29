package app

import (
	"log/slog"

	"taskdaemon/internal/audio"
	"taskdaemon/internal/config"
	"taskdaemon/internal/httpapi"
)

// App 组合 taskdaemon 的共享后端能力。
type App struct {
	cfg           config.Config
	logger        *slog.Logger
	loadConfig    func(config.LoadOptions) (config.Config, error)
	loadOptions   config.LoadOptions
	runtimeConfig *httpapi.RuntimeConfig
	audioService  *audio.Service
	audioQueue    *audio.Queue
}

// New 创建应用装配实例。
//
// 参数:
//   - cfg: 已加载的应用配置。
//   - logger: 结构化 logger；为空时使用 slog.Default。
//
// 返回值:
//   - *App: 应用装配实例。
func New(cfg config.Config, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	return &App{
		cfg:           cfg,
		logger:        logger,
		loadConfig:    config.Load,
		runtimeConfig: httpapi.NewRuntimeConfig(cfg),
	}
}

// WithConfigReload 配置 daemon 运行时重载使用的配置加载器。
//
// 参数:
//   - loader: 配置加载函数；为空时使用 config.Load。
//   - opts: 配置加载选项。
//
// 返回值:
//   - *App: 当前应用实例，便于链式调用。
func (app *App) WithConfigReload(loader func(config.LoadOptions) (config.Config, error), opts config.LoadOptions) *App {
	if loader == nil {
		loader = config.Load
	}
	app.loadConfig = loader
	app.loadOptions = opts
	return app
}
