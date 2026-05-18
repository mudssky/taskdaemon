# Go 后端 HTTP middleware 大文件拆分

## Goal

拆分 `services/taskdaemon-go/internal/httpapi/middleware.go`，把 trace/response options、请求日志、body capture、脱敏、recovery 和 session 校验分离，降低 HTTP API 横切逻辑的维护成本。

## Requirements

* 保持 `httpapi` package 不变，不创建子 package。
* 只做纯移动和必要 import 整理，不修改 middleware 顺序、响应 envelope、trace、日志脱敏或认证行为。
* 拆分目标：
  * `middleware.go`：通用 middleware 入口或轻量组合。
  * `trace_middleware.go`：trace id 注入与响应选项。
  * `request_logger.go`：访问日志主流程。
  * `body_capture.go`：有限 body 捕获与恢复。
  * `redaction.go`：JSON/text 脱敏。
  * `auth_middleware.go`：session 校验。
  * `recovery_middleware.go`：panic recovery。
* 保留当前 HTTP middleware 测试断言。

## Acceptance Criteria

* [x] `internal/httpapi/middleware.go` 明显收敛。
* [x] body capture、redaction、auth、recovery 逻辑可分别定位。
* [x] `go test ./internal/httpapi` 通过。
* [x] `pnpm test:go` 通过。
* [x] 提交说明标明这是纯拆分。

## Out of Scope

* 不改变 API envelope 或错误码。
* 不调整受保护路由范围。
* 不改变 HTTP body 日志默认关闭和 max bytes 行为。

## Technical Notes

* 来源规范：`.trellis/spec/backend/file-organization.md`。
* 相关规范：`.trellis/spec/backend/api-contracts.md`、`.trellis/spec/backend/logging-guidelines.md`、`.trellis/spec/backend/error-handling.md`。
* 实际拆分保持 `httpapi` package 不变，未创建子 package。
* `middleware.go` 保留全局 middleware 注册入口；注册顺序仍为 trace、response options、request logger、recovery。
* 已额外验证：`pnpm vet:go`、`git diff --check`。
