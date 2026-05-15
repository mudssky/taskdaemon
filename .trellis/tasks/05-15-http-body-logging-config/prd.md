# HTTP 请求响应体日志配置

## Goal

为 Go HTTP API 增加可配置的请求体与响应体日志能力，方便开发和排查线上问题，同时默认保持安全克制，避免敏感信息泄露和大 body 刷爆日志。

## Requirements

* 请求摘要日志继续默认记录 `method`、`path`、`status`、`duration_ms`、`trace_id`。
* 新增日志配置项控制是否记录请求体、响应体，默认都关闭。
* 支持 body 最大记录字节数，超出后截断并在日志中标记。
* 支持敏感字段脱敏，默认覆盖 `password`、`token`、`cookie`、`authorization` 等常见字段。
* 仅对 JSON body 做字段级脱敏；非 JSON body 只按大小限制记录原始文本片段。
* body 采集不得影响 Gin handler 正常读取请求体或写出响应体。
* body 字段沿用日志生态推荐的 snake_case 命名。
* 在开发工作区未显式传 `--config` 时，支持自动叠加本地配置文件：`taskdaemon.local.yaml`、`taskdaemon.local.yml`、`config.local.yaml`、`config.local.yml`。
* 本地配置加载顺序为 `defaults < base config < local config < env < overrides`；显式 `--config` 时不自动叠加 local 文件。

## Acceptance Criteria

* [ ] 默认配置下不会输出请求体或响应体。
* [ ] 开启请求体配置后，请求日志包含脱敏后的 `request_body`。
* [ ] 开启响应体配置后，请求日志包含脱敏后的 `response_body`。
* [ ] body 超过配置限制时会截断，并输出对应 truncated 标记。
* [ ] 请求体被日志读取后，业务 handler 仍能正常读取原请求体。
* [ ] 项目内 `.local.yaml/.local.yml` 能覆盖基础项目配置，且环境变量仍然优先。
* [ ] 显式 `--config` 时不会额外加载项目 local 配置。
* [ ] 现有 Go 测试、vet 通过。

## Definition of Done

* Tests added/updated for middleware and config behavior.
* Lint / typecheck / tests green for touched backend scope.
* Specs updated if this task introduces reusable logging conventions.
* Commit uses Conventional Commits with Chinese subject.

## Technical Approach

在现有 `requestLoggerMiddleware` 基础上扩展：

* `config.LoggingConfig` 下增加 `HTTP` 子配置，包含 `includeRequestBody`、`includeResponseBody`、`maxBodyBytes`、`redactFields`。
* Router 初始化时把日志 HTTP 配置传给 request logger middleware。
* 请求体：在进入后续 handler 前读取最多 N+1 字节并恢复 `ctx.Request.Body`。
* 响应体：包装 `gin.ResponseWriter`，透传写入同时缓存最多 N+1 字节。
* 日志输出前对 body 进行截断和 JSON 字段脱敏。
* 配置加载层在发现项目基础配置后，按约定查找同目录 local 配置并叠加。

## Out of Scope

* 不实现自定义按路由开关。
* 不记录 multipart/form-data 文件内容。
* 不改变现有响应 envelope 结构。
* 不调整日志轮转策略。
* 不让显式 `--config` 自动发现旁边的 `.local.yaml`。

## Technical Notes

* 现有相关文件：
  * `services/taskdaemon-go/internal/httpapi/middleware.go`
  * `services/taskdaemon-go/internal/httpapi/router.go`
  * `services/taskdaemon-go/internal/config/config.go`
  * `services/taskdaemon-go/internal/logging/logger.go`
* 用户偏好：默认文件日志 JSON、console 可读格式；body 日志默认关闭，开发/排障时按配置打开。
