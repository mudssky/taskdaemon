# Go 后端 app lifecycle 大文件拆分

## Goal

拆分 `services/taskdaemon-go/internal/app/app.go`，把应用装配、HTTP server 生命周期、配置重载、daemon HTTP client、migration 和地址工具分离，降低 app 边界继续增长时的维护风险。

## Requirements

* 保持 `app` package 不变，不创建子 package。
* 只做纯移动和必要 import 整理，不修改 serve、desktop、reload、migration 或 daemon API client 行为。
* 拆分目标：
  * `app.go`：`App` 类型、构造、依赖注入。
  * `serve.go`：HTTP server 生命周期。
  * `reload.go`：运行时配置重载和 daemon reload 调用。
  * `daemon_client.go`：CLI 到 daemon HTTP API 的请求封装。
  * `migration.go`：schema migration。
  * `address.go`：监听地址和端口冲突判断。
* 保留 app 测试中的 server cancel、daemon API、reload 和 migration 相关断言。

## Acceptance Criteria

* [ ] `internal/app/app.go` 收敛为 App 类型和构造入口。
* [ ] serve/reload/daemon client/migration/address 职责可分别定位。
* [ ] `go test ./internal/app` 通过。
* [ ] `pnpm test:go` 通过。
* [ ] 提交说明标明这是纯拆分。

## Out of Scope

* 不改变 HTTP API 地址、端口冲突提示或 reload 结果。
* 不调整 Desktop/Wails 行为。
* 不改变 migration 策略。

## Technical Notes

* 来源规范：`.trellis/spec/backend/file-organization.md`。
* 相关规范：`.trellis/spec/backend/configuration-runtime-guidelines.md`、`.trellis/spec/backend/api-contracts.md`、`.trellis/spec/backend/database-guidelines.md`。
