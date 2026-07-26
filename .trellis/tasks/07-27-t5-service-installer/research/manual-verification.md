# T5 手动验证记录

> 真实 OS 注册不进 CI。本文件归档手工验证观察。

## 环境

| 项 | 值 |
|---|---|
| 日期 | 2026-07-27 |
| 主机 | macmini (darwin arm64) |
| OS | macOS 26.x (Darwin 25.5.0) |
| 分支 | mudssky/t5-service-installer |
| 二进制 | `go run ./cmd/taskdaemon` |

## macOS

### dry-run（已执行）

```bash
cd services/taskdaemon-go
go run ./cmd/taskdaemon service install --dry-run --config /tmp/taskdaemon-t5-demo.yaml
go run ./cmd/taskdaemon service status
```

观察：

- 将写入路径：`~/Library/LaunchAgents/com.taskdaemon.daemon.plist`
- 配置路径为绝对路径 `/tmp/taskdaemon-t5-demo.yaml`
- 可执行文件为 `go run` 临时路径的绝对路径
- 作用域默认 `user`，平台 `darwin`
- plist 含 `RunAtLoad` + `KeepAlive`
- `status`：`installed: false`，`running: false`（本机未真正安装）

### 真实 install/uninstall

未在本 worker 会话对生产用户环境执行真实 `install`（避免污染 LaunchAgents 与占用端口）。建议验收人在隔离账号执行：

```bash
go install ./cmd/taskdaemon
taskdaemon service install --config "$HOME/.config/taskdaemon/config.yaml"
launchctl print gui/$(id -u)/com.taskdaemon.daemon
taskdaemon service status
# 可选：kill 进程后观察 KeepAlive 拉起
taskdaemon service uninstall
test ! -f ~/Library/LaunchAgents/com.taskdaemon.daemon.plist
```

## Linux

本会话无 Linux runner。单元生成由 `TestRenderSystemdContainsRestart` 锁定；真实 `systemctl --user` 流程待 Linux 主机补记。

预期命令：

```bash
taskdaemon service install --dry-run
taskdaemon service install
systemctl --user status taskdaemon.service
# lingering 提示应出现在 install 输出 messages 中
taskdaemon service uninstall
```

## Windows

本会话无 Windows runner。SCM 描述由 `TestRenderWindowsBinPath` / `TestWindowsUserScopeRejected` 锁定；真实 `sc create` 需管理员 shell 补记。

预期：

- 非管理员：`SERVICE_INSTALL_PERMISSION_DENIED` + 补救命令
- 管理员：`sc query TaskDaemon` 可见；`uninstall` 后删除

## 单元测试（CI 可跑）

```bash
cd services/taskdaemon-go
go test ./internal/service/... ./internal/cli/...
```

覆盖：三平台 Render、配置绝对路径、重复 install 拒绝、`--force`、注册/启动失败回滚、uninstall 幂等、权限错误码、unsupported 平台。
