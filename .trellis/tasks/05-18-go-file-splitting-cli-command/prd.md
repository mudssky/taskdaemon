# Go 后端 CLI command 大文件拆分

## Goal

拆分 `services/taskdaemon-go/internal/cli/command.go`，把 Cobra 根命令、配置命令、task daemon API 命令、输出格式化和 session token context 分离，提升 CLI 命令扩展时的可维护性。

## Requirements

* 保持 `cli` package 不变，不创建子 package。
* 只做纯移动和必要 import 整理，不修改 CLI 参数、输出结构、session token 传递或 hook 调用行为。
* 拆分目标：
  * `command.go`：根命令、全局 flag 和命令装配主线。
  * `config_commands.go`：config path/show/validate/reload。
  * `task_commands.go`：task trigger/cancel/runs。
  * `output.go`：YAML/JSON 输出和配置脱敏。
  * `session.go`：session token context 与解析。
* 保留 CLI 测试中的参数解析、输出和 hook 断言。

## Acceptance Criteria

* [ ] `internal/cli/command.go` 收敛为根命令装配入口。
* [ ] config/task/session/output 职责可分别定位。
* [ ] `go test ./internal/cli` 通过。
* [ ] `pnpm test:go` 通过。
* [ ] 提交说明标明这是纯拆分。

## Out of Scope

* 不新增 CLI 命令。
* 不改变 YAML/JSON 输出字段。
* 不改变 `TASKDAEMON_SESSION_TOKEN` 或 `--session-token` 优先级。

## Technical Notes

* 来源规范：`.trellis/spec/backend/file-organization.md`。
* 相关规范：`.trellis/spec/backend/configuration-runtime-guidelines.md`、`.trellis/spec/backend/api-contracts.md`、`.trellis/spec/backend/quality-guidelines.md`。
