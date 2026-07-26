# T5 HANDOFF — 系统服务安装器

## Summary

实现 `taskdaemon service install|uninstall|status`，将 taskdaemon 注册为 OS 服务：

- macOS：用户级 LaunchAgent（默认）/ 系统 LaunchDaemon
- Linux：用户级 systemd（默认）/ 系统 unit
- Windows：SCM 系统服务（唯一作用域，需管理员）

平台差异收敛在 `internal/service/`；CLI 层无 `runtime.GOOS`。

## Files

### 独占新建

- `services/taskdaemon-go/internal/cli/service_commands.go`
- `services/taskdaemon-go/internal/service/**`
  - `types.go` / `errors.go` / `resolve.go` / `manager.go`
  - `render_launchd.go` / `render_systemd.go` / `render_scm.go`
  - `driver_launchd.go` / `driver_systemd.go` / `driver_scm.go` / `platform_unsupported.go`
  - `manager_test.go`

> 注意：文件名**避免** `*_darwin.go` / `*_linux.go` / `*_windows.go` 后缀（Go 会按 GOOS 过滤），否则交叉平台 Render 测试无法在单机跑全。

### 共享 append-only

- `services/taskdaemon-go/internal/cli/command.go` — 追加 `service` 子命令注册
- `services/taskdaemon-go/internal/cli/command_test.go` — 追加 dry-run / status 测试
- `services/taskdaemon-go/package.json` — 追加 `service:install|status|uninstall`
- `README.md` — 追加系统服务文档
- `.trellis/tasks/07-27-t5-service-installer/research/manual-verification.md`
- `services/taskdaemon-go/internal/config/config_test.go` — 修 macOS `/var` vs `/private/var` 预存 flake（与 T5 无关，但挡住 `pnpm test:go`）

## Behavior

| 命令 | 行为 |
|---|---|
| `service install` | 解析绝对 config/exe → 写单元 → 注册 → 启动；失败逆序回滚 |
| `service install --dry-run` | 只 Render 打印，零副作用 |
| `service install --force` | 覆盖已有单元 |
| `service uninstall` | 未安装安全 no-op；已安装 stop+unregister+删文件 |
| `service status` | installed + running + unitPath |

错误码：`SERVICE_INSTALL_PERMISSION_DENIED`、`SERVICE_ALREADY_INSTALLED`、`SERVICE_UNSUPPORTED_PLATFORM`、`SERVICE_INVALID_SCOPE` 等。

配置路径：复用 `config.ResolvePaths`；显式 `--config` 转绝对路径写入单元 `serve --config <abs>`。

## Verification

```bash
pnpm typecheck && pnpm lint && pnpm test:go && pnpm vet:go
cd services/taskdaemon-go && go test ./internal/service/... ./internal/cli/...
go run ./cmd/taskdaemon service install --dry-run --config /tmp/x.yaml
go run ./cmd/taskdaemon service status
```

macOS dry-run / status 已在本机执行；真实 launchctl 注册未跑（避免污染用户环境）。Linux/Windows 真实注册待补。

## Residual

1. 三平台真实 install/uninstall 需验收人在目标机补跑并更新 `research/manual-verification.md`
2. Windows SCM：当前用 `sc.exe` 注册 `serve`；完整 SCM 控制信号（ServiceMain）若生产需要，可后续加 `service run` 包装，不在本任务阻塞
3. 未 merge；未改 notify/desktop/audio/Ent/pnpm-lock

## Do not merge

协调者验收后再合。
