# Go 后端 logging 大文件拆分

## Goal

拆分 `services/taskdaemon-go/internal/logging/logger.go`，把 logger 工厂、文件 handler、fanout handler、pretty handler 和 HTTP 日志格式化分离，保持日志行为不变但提升可读性。

## Requirements

* 保持 `logging` package 不变，不创建子 package。
* 只做纯移动和必要 import 整理，不修改日志输出格式、脱敏、文件轮转或 level 解析行为。
* 拆分目标：
  * `logger.go`：对外构造、关闭、level 解析。
  * `file_handler.go`：日志文件可写性和 lumberjack handler。
  * `fanout_handler.go`：多 handler 分发。
  * `pretty_handler.go`：console pretty handler。
  * `format.go`：HTTP 单行日志和值格式化。
* 保留已有测试覆盖的行为契约。

## Acceptance Criteria

* [x] `internal/logging/logger.go` 不再承载所有 handler 和格式化逻辑。
* [x] 新文件职责清晰，私有 helper 放在最接近调用者的位置。
* [x] `go test ./internal/logging` 通过。
* [x] `pnpm test:go` 通过。
* [x] 提交说明标明这是纯拆分。

## Out of Scope

* 不改变日志字段名、日志格式、脱敏字段或文件轮转配置。
* 不新增日志依赖。
* 不调整 HTTP middleware 日志采集逻辑。

## Technical Notes

* 来源规范：`.trellis/spec/backend/file-organization.md`。
* 相关规范：`.trellis/spec/backend/logging-guidelines.md`、`.trellis/spec/backend/configuration-runtime-guidelines.md`。
* 实际拆分保持 `logging` package 不变，未创建子 package。
* 已额外验证：`pnpm vet:go`、`git diff --check`。
