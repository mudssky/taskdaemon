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
* 需要讨论配置变更生效机制：支持实时生效或通过命令触发重载，避免每次改日志开关都重启 daemon。
* 需要完善 `config` 子命令，让用户能查看/定位/校验配置。
* 需要优化 console 日志可读性；当前 `slog.TextHandler` 会输出 `time=... level=... msg=... request_body="" response_body="map[...]"`，人工阅读体验较差。
* 需要增加开发期运行 CLI 的 package scripts，方便直接测试 `taskdaemon config ...` 等子命令。

## Acceptance Criteria

* [ ] 默认配置下不会输出请求体或响应体。
* [ ] 开启请求体配置后，请求日志包含脱敏后的 `request_body`。
* [ ] 开启响应体配置后，请求日志包含脱敏后的 `response_body`。
* [ ] body 超过配置限制时会截断，并输出对应 truncated 标记。
* [ ] 请求体被日志读取后，业务 handler 仍能正常读取原请求体。
* [ ] 项目内 `.local.yaml/.local.yml` 能覆盖基础项目配置，且环境变量仍然优先。
* [ ] 显式 `--config` 时不会额外加载项目 local 配置。
* [ ] 配置变更生效方式有明确 MVP 范围和不可热更新项说明。
* [ ] `config` 子命令覆盖查看配置路径、打印合并后配置、校验配置这类基础排障能力。
* [ ] console 日志对 HTTP 请求使用更适合人工阅读的格式，尤其是 body/envelope 不再打印成 Go map 字符串。
* [ ] package scripts 支持从 workspace 根目录直接透传 CLI 参数，并支持安装本地 CLI 二进制。
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
* 配置重载候选方向：
  * 推荐 MVP：`taskdaemon config reload` 通过 daemon HTTP 管理端点触发重载，仅应用运行时安全配置，例如 logging/observability；server/database 等需要重启。
  * 备选：文件监听自动热加载，开发体验更顺，但跨平台 watcher、错误回滚和日志噪音更复杂。
* `config` 子命令候选能力：`path`、`show`、`validate`、`reload`。
* console 日志候选方向：保留 file JSON；console 使用自定义 pretty handler 或 HTTP request 专用格式化输出，body JSON 以紧凑 JSON 字符串展示。
* package scripts 候选方向：
  * `pnpm cli -- <args>`：开发期直接 `go run ./cmd/taskdaemon <args>`，不需要安装。
  * `pnpm install:cli`：执行 `go install ./cmd/taskdaemon`，把当前版本安装到 `GOBIN` / `GOPATH/bin`。
  * `pnpm build:backend`：继续输出到仓库 `build/bin/taskdaemon`，适合固定路径调试。

## Out of Scope

* 不实现自定义按路由开关。
* 不记录 multipart/form-data 文件内容。
* 不改变现有响应 envelope 结构。
* 不调整日志轮转策略。
* 不让显式 `--config` 自动发现旁边的 `.local.yaml`。
* 不让 server host/port、database driver/dsn 在运行中无缝热切换；这些配置变更需要重启。
* 不在本任务里实现系统级 service 安装器；这里只处理开发期 CLI 运行/安装脚本。

## Technical Notes

* 现有相关文件：
  * `services/taskdaemon-go/internal/httpapi/middleware.go`
  * `services/taskdaemon-go/internal/httpapi/router.go`
  * `services/taskdaemon-go/internal/config/config.go`
  * `services/taskdaemon-go/internal/logging/logger.go`
* 用户偏好：默认文件日志 JSON、console 可读格式；body 日志默认关闭，开发/排障时按配置打开。
* 当前代码事实：
  * `internal/cli` 暂无 `config` 子命令。
  * `internal/config.Load` 只在命令启动时加载一次配置。
  * `internal/logging.New` 的 console text handler 会把结构化 map 以 Go 文本形式打印，`response_body=map[...]` 可读性差。
  * 根 `package.json` 已有 `dev:backend`/`build:backend`，service `package.json` 已有 `dev`/`build`，但缺少任意 CLI 参数透传和 `go install` 脚本。
