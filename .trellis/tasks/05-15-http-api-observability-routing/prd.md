# brainstorm: HTTP API 可观测性与路由整理

## Goal

为 taskdaemon Go HTTP API 补齐请求日志、应用日志配置、全局响应 envelope，并整理 `internal/httpapi` 的路由文件结构，解决接口报错时没有请求日志、响应契约不够集中、日志无法配置输出目标、`router.go` 单文件过大的维护问题。

## What I Already Know

* 用户反馈 `/api/auth/init` 报错时没有请求日志，排障困难。
* 当前 `services/taskdaemon-go/internal/httpapi/router.go` 使用 `gin.New()` + `gin.Recovery()`，没有请求日志 middleware。
* `internal/app` 已经持有 `*slog.Logger`，但 `httpapi.Options` 还没有 logger 字段，`NewRouter` 没有日志依赖注入。
* `.trellis/spec/backend/logging-guidelines.md` 要求使用 `slog`，request 日志保留 `method`、`path`、`status`、`duration_ms`，默认不记录 body。
* `.trellis/spec/backend/error-handling.md` 已定义错误响应为 `{"error":{"code","message","details"}}`，认证接口刚补充了 `username/password/body` 字段错误矩阵。
* `router.go` 当前同时包含 router 装配、auth routes、task routes、middleware、DTO、cookie、错误响应和实体转换函数。
* `cmd/taskdaemon/main.go` 当前直接使用 `slog.NewTextHandler(os.Stderr, nil)` 构造 logger；如果日志输出要由配置控制，需要在加载配置后构造应用 logger，或者把 logger 工厂下沉到 hooks。
* `config.Config` 当前只有 `Server` 和 `Database`，配置加载已支持 defaults < file < env < overrides，可自然扩展 `logging.*` 键。
* `taskdaemon.example.yaml` 目前没有 logging 配置示例。
* `testify` 已经在项目中使用：`services/taskdaemon-go/go.mod` 依赖 `github.com/stretchr/testify v1.11.1`，`auth/httpapi/runner/scheduler/data` 等测试中大量使用 `require`。
* Context7 `/stretchr/testify` 文档显示 Testify 提供 `assert`、`require`、`mock`、`suite`；`require` 适合关键前置条件失败时立即终止测试。

## Assumptions (Temporary)

* 请求日志 middleware 应注入 `slog.Logger`，而不是使用 Gin 默认 logger 输出到 stdout。
* 全局响应格式采用统一 envelope，并同步前端 API client；这是破坏性 API 契约迁移，应一次性覆盖现有接口和测试。
* 路由拆分应保持 package `httpapi` 内部结构调整，不改变 API URL。
* Testify 不需要移除；反而可以继续作为 Go 测试断言标准。
* 日志输出目标建议支持 console、file、both；默认 console，保持当前开发体验。
* `log/slog` 本身不提供文件轮转；它通过 handler 写入 `io.Writer`。文件轮转使用 `gopkg.in/natefinch/lumberjack.v2` 作为 writer 接入 slog。
* 文件日志本轮纳入轮转能力，只使用 lumberjack 原生能力：基于文件大小的轮转、备份数量、保留天数和 gzip 压缩；不做按日期切分等二次开发。
* 文件日志默认 JSON，便于后续机器采集和检索；console 日志默认使用可读性更高的 text 格式。

## Open Questions

* 无。

## Requirements (Evolving)

* 添加 HTTP 请求日志 middleware。
* 添加 trace id middleware：优先复用 `X-Trace-Id`，缺失时生成；响应头回写 `X-Trace-Id`；请求日志包含 `trace_id`。
* 日志字段至少包含 method、path、status、duration_ms。
* 请求日志默认记录所有非健康检查 API；成功的 `/api/health` 不记录，异常状态仍记录。
* 日志不得包含 request/response body、Cookie、session token、CSRF token、密码。
* 添加日志配置：level、output、console format、file path、file rotation，必要时支持 console+file。
* 更新配置加载、示例配置与环境变量说明。
* 将所有 HTTP API 成功/失败响应迁移到统一 envelope：`code` 使用数字成败位，`msg` 是默认说明，`data` 是业务数据，失败时通过 `error` 保存稳定错误对象；成功响应不携带 `error` 字段。
* 原本 `204 No Content` 的成功接口统一改为 `200 OK` + envelope，例如 `{"code":0,"msg":"ok","data":null}`。
* 同步前端 API client 以解析新 envelope。
* 统一并记录 API 响应格式约定，包含成功、错误、列表与 204/no-content 边界。
* 拆分 `internal/httpapi/router.go`，降低单文件复杂度。
* 保持现有 API URL 不变；响应 shape 将发生迁移，前后端同任务内同步。
* 继续使用 Testify 的 `require` 作为 Go 测试断言工具。

## Acceptance Criteria (Evolving)

* [ ] 请求经过 HTTP API 后会产生结构化 slog 记录。
* [ ] 请求无 `X-Trace-Id` 时服务端生成并回写；请求已有 `X-Trace-Id` 时复用并回写。
* [ ] 默认响应 body 带 `traceId`，可通过配置关闭；请求日志始终带 `trace_id`。
* [ ] 4xx/5xx 请求日志级别或字段能支持快速筛选失败请求。
* [ ] `/api/auth/init` 这类错误请求能通过日志看到 method/path/status/duration，不泄漏敏感信息。
* [ ] 成功的 `/api/health` 不产生日志；失败的 `/api/health` 仍产生日志。
* [ ] 配置文件和环境变量可控制日志级别、格式和输出目标。
* [ ] console 日志和 file 日志可分别/同时启用。
* [ ] API 响应统一为 `code/msg/data/error` envelope，前端 API client 可正确解析成功和错误响应。
* [ ] logout/cancel 等原 204 成功接口返回 200 envelope。
* [ ] API 响应格式有集中 helper/类型和文档说明。
* [ ] `router.go` 拆分后职责清晰，测试仍通过。
* [ ] `go test ./internal/httpapi` 通过。
* [ ] `pnpm test:go` 通过。
* [ ] `pnpm --filter @taskdaemon/web test -- src/lib/api/client.test.ts` 通过。

## Definition of Done

* Tests added/updated (unit/integration where appropriate)
* Lint / typecheck / CI green, or known unrelated failure documented
* Docs/spec updated for response/logging contract
* Rollout/rollback considered if response shape changes

## Out of Scope (Explicit)

* 远程日志采集。
* 按日期/时间切分日志文件。
* 记录请求/响应 body。
* 引入第三方 logging middleware，除非后续研究确认明显优于本地实现。
* 移除 Testify。

## Technical Notes

* Likely files:
  * `services/taskdaemon-go/internal/httpapi/router.go`
  * new `services/taskdaemon-go/internal/httpapi/middleware.go`
  * new `services/taskdaemon-go/internal/httpapi/auth_routes.go`
  * new `services/taskdaemon-go/internal/httpapi/task_routes.go`
  * new `services/taskdaemon-go/internal/httpapi/responses.go`
  * `services/taskdaemon-go/internal/app/app.go`
  * `services/taskdaemon-go/cmd/taskdaemon/main.go`
  * `services/taskdaemon-go/internal/config/config.go`
  * `services/taskdaemon-go/internal/config/config_test.go`
  * `services/taskdaemon-go/taskdaemon.example.yaml`
  * `apps/web/src/lib/api/client.ts`
  * `apps/web/src/lib/api/client.test.ts`
  * `.trellis/spec/backend/logging-guidelines.md`
  * `.trellis/spec/backend/error-handling.md`
* Existing tests:
  * `services/taskdaemon-go/internal/httpapi/auth_test.go`
  * `services/taskdaemon-go/internal/httpapi/task_test.go`
  * `services/taskdaemon-go/internal/httpapi/router_test.go`
* Testify note:
  * `require` is already used and is appropriate for HTTP response assertions where continuing after failure would cascade.

## Expansion Sweep

### Future Evolution

* 后续可把 trace id 继续传递到任务执行链路，方便把一次 API 请求和任务执行日志串起来。
* 后续可增加配置项控制 debug 日志、慢请求阈值和访问日志开关。
* 后续可增加远程 JSON 日志采集；不做按日期切分，除非后续明确替换或扩展 lumberjack 策略。

### Related Scenarios

* HTTP API、CLI 调 daemon API、未来 Desktop/Wails 绑定应共享稳定错误码概念。
* 创建/更新任务、认证、手动触发/取消都应通过同一错误响应 helper 输出。

### Failure & Edge Cases

* panic 后仍需要 Recovery 兜底并记录 500；若使用 Gin RecoveryWithWriter，需要确认日志不重复且不泄漏 body。
* 404/405、OPTIONS 应正常记录；成功的 `/api/health` 默认不记录，异常状态仍记录。
* 日志 middleware 不能在未注入 logger 时 panic，应回退 `slog.Default()`。
* 文件日志路径不可写时应用应 fail fast，避免用户以为日志已落盘。

## Decision (ADR-lite)

### Global API Envelope

**Context**: 用户明确选择全局 envelope，期望统一成功与失败响应，而不是只统一错误响应。

**Decision**: 本任务迁移所有 HTTP API JSON 响应到统一 envelope，并同步前端 API client 与测试。API URL 保持不变。推荐响应形态：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {}
}
```

```json
{
  "code": 1,
  "msg": "Invalid request body",
  "data": null,
  "error": {
    "code": "bad_request",
    "message": "Invalid request body",
    "details": { "field": "body" }
  }
}
```

`code` 的数字值只表达 envelope 层面的成败：`0` 成功，`1` 失败。具体错误类型继续由 `error.code` 承载，例如 `bad_request`、`unauthorized`、`task_invalid`。HTTP 状态码仍保持 2xx/4xx/5xx，不把所有响应都改成 200。

成功响应不带 `error` 字段；失败响应必须带 `error` 字段。

原本返回 `204 No Content` 的成功接口也统一改为 `200 OK` + envelope：

```json
{
  "code": 0,
  "msg": "ok",
  "data": null
}
```

**Consequences**: 这是破坏性响应格式调整；需要覆盖后端 HTTP 测试、前端 API client 测试和关键组件测试。CLI 调 daemon API 的错误展示也需要兼容新格式。

### Logging Configuration

**Context**: 当前日志只能写 stderr，无法配置控制台/文件输出，接口报错时也缺 request log。

**Decision**: 增加 `logging` 配置段，第一版至少覆盖 `level`、`output`、`console.format`、`file.path`、`file.maxSizeMB`、`file.maxBackups`、`file.maxAgeDays`、`file.compress`。请求日志通过 `httpapi` middleware 使用注入的 `*slog.Logger`。

默认策略：

```yaml
logging:
  level: info
  output: console
  console:
    format: text
  file:
    path: ./logs/taskdaemon.log
    format: json
    maxSizeMB: 100
    maxBackups: 7
    maxAgeDays: 30
    compress: true
```

`log/slog` 继续作为日志 facade；文件轮转由 `gopkg.in/natefinch/lumberjack.v2` 承担。`lumberjack.Logger` 实现 `Write(p []byte)`，可直接作为 `slog.NewTextHandler` 或 `slog.NewJSONHandler` 的 writer。根据 Context7 `/natefinch/lumberjack` 文档，其配置支持 `Filename`、`MaxSize`、`MaxBackups`、`MaxAge`、`LocalTime`、`Compress`。

**Consequences**: `cmd/taskdaemon/main.go` / CLI hook 需要在加载配置后构造 logger；配置文件路径错误或日志文件不可写应返回启动错误。

### Trace ID

**Context**: 请求日志需要能和浏览器/CLI 看到的单次请求关联。用户倾向排障时直接在响应 JSON 中看到关联 ID，同时保留日志检索友好的字段命名。

**Decision**: 本轮加入 trace id 支持。请求进入时如果 `X-Trace-Id` header 已存在非空值则复用，否则生成新的 trace id；所有响应回写 `X-Trace-Id`；请求日志字段使用日志生态更常见的 snake_case `trace_id`。响应 JSON 默认包含 camelCase `traceId`，并通过配置控制是否写入响应体。

配置建议：

```yaml
observability:
  traceId:
    includeInResponse: true
```

成功响应：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {},
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736"
}
```

失败响应：

```json
{
  "code": 1,
  "msg": "Invalid request body",
  "data": null,
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
  "error": {
    "code": "bad_request",
    "message": "Invalid request body",
    "details": { "field": "body" }
  }
}
```

**Consequences**: 日志排障可以通过 body、header 和日志字段关联请求。响应体会包含一个观测字段，但默认排障体验更好；如调用方追求纯业务响应，可关闭 `observability.traceId.includeInResponse`。

## Technical Approach

* API envelope:
  * 新增集中响应 helper，例如 `writeAPISuccess(ctx, data)`、`writeAPISuccessStatus(ctx, status, data)`、`writeAPIError(...)`。
  * 成功响应：`{"code":0,"msg":"ok","data":...}`，成功不带 `error`。
  * 失败响应：`{"code":1,"msg":"...","data":null,"error":{"code":"...","message":"...","details":...}}`。
  * 原 204 成功接口改为 `200` + `data:null`。
* HTTP middleware:
  * `traceIDMiddleware` 负责 `X-Trace-Id`。
  * `requestLoggerMiddleware` 记录 method、path、status、duration_ms、trace_id；成功 `/api/health` 跳过。
  * panic/recovery 继续兜底，必要时接入同一个 logger。
* Logging config:
  * `config.Config` 增加 `Logging LoggingConfig`。
  * 新增 logger 构造边界，按配置创建 console/file/both handler。
  * file writer 使用 `lumberjack`，文件默认 JSON；console 默认 text。
* Router split:
  * `router.go` 保留 `Options`、service interfaces、`NewRouter` 和 health/swagger 装配。
  * `auth_routes.go` 放 auth route 与 auth DTO/cookie helper。
  * `task_routes.go` 放 task route、task DTO、Ent response mapping。
  * `middleware.go` 放 trace id、request logger、session middleware。
  * `responses.go` 放 envelope/error response 类型与 helper。
* Frontend:
  * 更新 `apps/web/src/lib/api/client.ts` 解析 envelope。
  * 更新前端 API client/auth setup 测试期望。

## Implementation Plan

* PR1: 配置与 logger 工厂
  * 增加 logging config、示例配置、配置测试。
  * 引入 lumberjack 并封装 console/file/both slog handler。
* PR2: HTTP middleware 与 router 拆分
  * 增加 trace id、request logger。
  * 拆分 `router.go`。
  * 补 httpapi middleware 测试。
* PR3: API envelope 迁移
  * 后端所有 JSON 响应改 envelope。
  * 204 改 200 envelope。
  * 前端 API client 同步。
  * 更新 HTTP/API client/组件测试。
