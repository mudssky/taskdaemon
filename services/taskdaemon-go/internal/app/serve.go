package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"taskdaemon/internal/agentbridge"
	"taskdaemon/internal/audio"
	"taskdaemon/internal/auth"
	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/httpapi"
	"taskdaemon/internal/notify"
	"taskdaemon/internal/runlog"
	"taskdaemon/internal/runner"
	"taskdaemon/internal/scheduler"
	"taskdaemon/web/embedded"
)

// Serve 启动 HTTP API server，并在 context 取消时尝试优雅关闭。
//
// 参数:
//   - ctx: 控制 server 生命周期的 context。
//
// 返回值:
//   - error: server 启动或关闭失败时返回错误。
func (app *App) Serve(ctx context.Context) error {
	store, err := data.Open(ctx, app.cfg.Database)
	if err != nil {
		return fmt.Errorf("open data store: %w", err)
	}
	defer func() {
		_ = store.Close()
	}()

	if err := app.migrateStore(ctx, store); err != nil {
		return err
	}

	// T2a: 通知总线与站内 sink 装配（与 audio service 同层）
	notifyBus := notify.NewBus(app.cfg.Notify, notify.BusOptions{Logger: app.logger})
	storeSink := notify.NewStoreSink(store, app.cfg.Notify.Store, notify.StoreSinkOptions{Logger: app.logger})
	notifyBus.Register(storeSink)
	// T4: Webhook / 邮件出站 sink
	webhookSink := notify.NewWebhookSink(app.cfg.Notify.Webhook, notify.WebhookSinkOptions{Logger: app.logger})
	emailSink := notify.NewEmailSink(app.cfg.Notify.Email, notify.EmailSinkOptions{Logger: app.logger})
	notifyBus.Register(webhookSink)
	notifyBus.Register(emailSink)
	// D2 / T3: Desktop 原生通知 sink（纯 server 无 Bind 时 Enabled=false）
	desktopSink := notify.NewDesktopSink(app.cfg.Notify.Desktop, notify.DesktopSinkOptions{Logger: app.logger})
	notifyBus.Register(desktopSink)
	notifyBus.Start()
	app.notifyBus = notifyBus
	app.storeSink = storeSink
	defer notifyBus.Shutdown()
	runLogService, err := runlog.New(store, app.cfg.RunLog, runlog.Options{Logger: app.logger})
	if err != nil {
		return fmt.Errorf("init runlog service: %w", err)
	}
	agentClient := agentbridge.NewClient(app.cfg.AgentBridge)
	exec := runner.NewExecutor().WithAgentRunner(agentbridge.NewRunnerAdapter(agentClient))
	taskService := scheduler.NewService(store, scheduler.Options{
		Runner:    exec,
		Publisher: notifyBus,
		RunLog:    runLogService,
	})
	if err := taskService.RegisterEnabledTasks(ctx); err != nil {
		return fmt.Errorf("register enabled tasks: %w", err)
	}
	if err := taskService.Start(); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	defer func() {
		_ = taskService.Shutdown()
	}()
	audioQueue := audio.NewQueue(store, app.cfg.Audio, audio.QueueOptions{Logger: app.logger})
	app.audioQueue = audioQueue
	defer audioQueue.Shutdown()
	app.audioService = audio.New(store, app.cfg.Audio, audio.Options{Queue: audioQueue})

	bridgeCfg := app.cfg.AgentBridge
	writeConfig := httpapi.DefaultConfigSectionWriter(func(ctx context.Context, patch config.AudioSectionPatch) (httpapi.ConfigSectionWriteResponse, error) {
		result, err := app.WriteAudioConfig(ctx, patch)
		if err != nil {
			return httpapi.ConfigSectionWriteResponse{}, err
		}
		subsystems := make([]httpapi.ConfigSubsystemStatusResponse, 0, len(result.Subsystems))
		for _, item := range result.Subsystems {
			subsystems = append(subsystems, httpapi.ConfigSubsystemStatusResponse{
				Name:   item.Name,
				Status: item.Status,
				Error:  item.Error,
			})
		}
		return httpapi.ConfigSectionWriteResponse{
			Config:          httpapi.AudioConfigResponseFromConfig(result.Config.Audio),
			Applied:         result.Applied,
			RestartRequired: result.RestartRequired,
			Reload: httpapi.ConfigSectionReloadResponse{
				Applied:         result.Reload.Applied,
				RestartRequired: result.Reload.RestartRequired,
				Subsystems:      subsystems,
			},
		}, nil
	})
	server := &http.Server{
		Addr: app.cfg.Server.Address(),
		Handler: httpapi.NewRouter(httpapi.Options{
			EnableSwagger: app.cfg.Server.Swagger.Enabled,
			Auth:          auth.New(store, auth.Options{}),
			Tasks:         taskService,
			Audio:         app.audioService,
			// T2a
			Notifications:          storeSink,
			NotifyBus:              notifyBus,
			Logger:                 app.logger,
			IncludeTraceInResponse: &app.cfg.Observability.TraceID.IncludeInResponse,
			HTTPLog:                app.cfg.Logging.HTTP,
			RuntimeConfig:          app.runtimeConfig,
			ReloadConfig:           app.ReloadRuntimeConfig,
			WriteConfig:            writeConfig,
			FrontendFS:             mustFrontendFS(),
			// T6
			RunLog: runLogService,
			// G6
			AgentBridgeConfig: func() config.AgentBridgeConfig { return bridgeCfg },
		}),
	}

	errCh := make(chan error, 1)
	go func() {
		app.logger.Info("starting http api", "addr", server.Addr, "swagger", app.cfg.Server.Swagger.Enabled)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		if err := server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		if isAddressInUse(err) {
			return fmt.Errorf("http api address %s is already in use; stop the process using it or set TASKDAEMON_SERVER_PORT / server.port to an explicit alternate port: %w", server.Addr, err)
		}
		return err
	}
}

// mustFrontendFS 返回嵌入的前端 dist 文件系统。
//
// 参数:
//   - 无。
//
// 返回值:
//   - fs.FS: dist 根目录文件系统。
func mustFrontendFS() fs.FS {
	frontendFS, err := fs.Sub(embedded.Assets, "dist")
	if err != nil {
		panic(fmt.Errorf("frontend assets unavailable: %w", err))
	}
	return frontendFS
}
