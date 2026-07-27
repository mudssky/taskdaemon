# 后端 API 与 DTO 契约

> HTTP API 是 Go 后端、Web 管理台、CLI daemon client 和 Desktop shell 之间的长期边界。

---

## 概览

`services/taskdaemon-go/internal/httpapi` 是唯一 HTTP API 边界。handler 负责请求绑定、认证校验、DTO 转换、稳定错误码和统一 envelope；业务判断留在 `scheduler`、`auth`、`config`、`app` 等 service 层。

新增或修改 API 时，不只改后端 route。必须同时检查前端 API client、DTO 类型、query/mutation hook、CLI daemon client 和测试，避免跨层契约漂移。

---

## 包边界

* `internal/httpapi/router.go` 负责 router 装配、middleware 顺序、前端静态资源和 SPA fallback。
* `internal/httpapi/*_routes.go` 负责路由注册、请求绑定、权限校验、调用 service 和错误映射。
* `internal/httpapi/*_dto.go` 负责 request/response DTO，以及 Ent/service 类型到 API 响应的转换。
* `internal/httpapi/responses.go` 是 API envelope、trace id 和错误对象的单一写出边界。
* handler 依赖接口，例如 `AuthService`、`TaskService`，不直接依赖具体 service 构造细节。
* handler 不直接拼 SQL，不直接操作 Ent query，不绕过 service 判断业务状态。
* Desktop 和 CLI 如需操作运行中的 daemon，优先走同一 HTTP API 契约，不另起一套隐藏业务路径。

---

## Envelope 契约

所有 HTTP JSON 响应使用统一 envelope。HTTP 状态码表达真实 2xx/4xx/5xx，顶层 `code` 只表达成败。

成功响应：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {},
  "traceId": "trace-id"
}
```

失败响应：

```json
{
  "code": 1,
  "msg": "Task definition is invalid",
  "data": null,
  "traceId": "trace-id",
  "error": {
    "code": "task_invalid",
    "message": "Task definition is invalid",
    "details": {
      "field": "cronExpression"
    }
  }
}
```

* 成功响应使用 `writeAPISuccess` / `writeAPIOK`。
* 失败响应使用 `writeAPIError` 或更具体的封装，例如 `writeUnauthorized`。
* `data` 的形状必须稳定；无内容成功响应也返回 envelope，`data` 可为 `null`。
* `traceId` 默认写入响应体；配置关闭时仍必须写入 `X-Trace-Id` header。
* 不新增绕过 envelope 的 JSON 响应，除非是静态资源、Swagger 或明确的非 JSON 协议。

---

## DTO 命名与转换

* 请求 DTO 使用小写私有类型，例如 `createTaskRequest`、`loginRequest`。
* 响应 DTO 使用小写私有类型，例如 `taskResponse`、`runResponse`。
* DTO 字段使用前端契约需要的 camelCase JSON tag，例如 `cronExpression`、`traceId`、`restartRequired`。
* DTO 不直接暴露 Ent entity、bcrypt hash、session token hash、CSRF token hash、数据库 DSN 或内部错误对象。
* Ent/service 类型转换集中在 `*_dto.go` 或同 package 的小型转换函数中，例如 `taskResponseFromEnt`。
* request -> service input 的转换应靠近 route 或 DTO 文件，字段默认值和业务校验仍由 service 层兜底。
* 列表响应优先使用命名对象包裹，例如 `{"tasks": [...]}`、`{"runs": [...]}`，避免顶层数组影响后续扩展。

---

## Handler 职责

handler 可以做：

* 绑定 JSON、path param、query param 和 Cookie。
* 调用 session middleware 或认证 service。
* 把 request DTO 转成 service input。
* 把 service 结果转成 response DTO。
* 把 sentinel/typed error 映射到 HTTP 状态码和稳定 `error.code`。

handler 不应做：

* 直接实现调度、runner、认证、数据库事务或配置 reload 的业务流程。
* 用字符串包含判断识别业务错误。
* 将第三方库错误原样作为前端稳定契约。
* 在单个 handler 中重复实现全局 middleware 已处理的认证、trace、recovery 或日志逻辑。

---

## 错误码

* `error.code` 是前端、CLI 和测试做分支判断的稳定契约。
* `msg` 和 `error.message` 只是默认说明，不作为前端业务分支依据。
* 新增错误码使用小写 snake_case，表达业务语义，例如 `task_not_running`、`config_reload_failed`。
* 同一类错误在不同 endpoint 中尽量复用稳定码，例如认证缺失统一为 `unauthorized`。
* 字段级输入错误用 `bad_request`，并在安全的 `details.field` 中指出字段。
* 未分类系统错误先映射为 `internal_error` 或具体 `*_failed`，不要暴露底层连接串、文件路径凭证或 token。

新增错误码时必须检查：

* 后端 route 测试是否断言 HTTP 状态码和 `error.code`。
* 前端 API client 或组件是否需要新增错误码映射。
* 文案是否在 UI 展示层处理，而不是依赖后端 message。

---

## 分页与限制

* 列表、执行历史和日志类接口必须设置默认 limit 或分页。
* handler 可传入 endpoint 级默认值，例如任务列表 100、执行历史 50。
* service 层必须二次兜底，避免未来调用者绕过 handler 传入无限 limit。
* 返回大文本时优先返回已截断的业务字段和截断标记，不通过 API 临时返回无限 stdout/stderr。

---

## 执行契约速查

### 1. Scope / Trigger

触发条件：新增或修改 HTTP route、request/response DTO、API envelope、错误码、traceId、CLI daemon API client 或前端 API client 类型。

### 2. Signatures

* 后端 route 注册：`registerXRoutes(router *gin.Engine, authService AuthService, service XService)`。
* 成功响应：`writeAPISuccess(ctx *gin.Context, status int, data any)` 或 `writeAPIOK(ctx *gin.Context, data any)`。
* 错误响应：`writeAPIError(ctx *gin.Context, status int, code string, message string, details any)`。
* DTO 转换：`xResponseFromEnt(record *ent.X) xResponse` 或 `xInputFromRequest(req xRequest) service.Input`。

### 3. Contracts

* 请求字段：camelCase JSON tag；必填字段用 Gin binding 或 handler 显式校验。
* 响应字段：统一 envelope；`data` 内使用命名对象或私有 response DTO。
* 错误字段：`error.code` 为稳定 snake_case；`error.details` 只放安全结构化信息。
* trace：响应 header 始终包含 `X-Trace-Id`；响应 body 的 `traceId` 受运行时配置控制。

### 4. Validation & Error Matrix

| 条件 | HTTP | error.code | 说明 |
|---|---:|---|---|
| 未登录或 session 无效 | 401 | `unauthorized` | 受保护 API 统一返回 |
| JSON 非法或必填字段缺失 | 400 | `bad_request` | `details.field` 指向字段 |
| 业务输入非法 | 400 | `task_invalid` 或具体码 | 例如 cron/runner 校验 |
| 业务状态冲突 | 409 | `task_running` / `task_not_running` | 删除运行中任务、取消未运行任务 |
| 依赖服务未注入 | 503 | `*_unavailable` | 测试或装配错误可诊断 |
| 未分类系统错误 | 500 | `internal_error` 或 `*_failed` | 不暴露敏感细节 |

### 5. Good / Base / Bad Cases

* Good：新增任务字段时，同步 request DTO、service input、response DTO、前端类型、API client 测试和 handler 测试。
* Base：只新增只读字段时，至少同步 response DTO、前端类型和关键展示测试。
* Bad：后端直接返回 Ent entity，前端靠 `msg` 做分支，或 route 返回裸 JSON 错误。

### 6. Tests Required

* 成功路径断言 HTTP 状态码、envelope `code=0`、`data` 形状和 trace id。
* 输入错误断言 HTTP 400、`error.code`、`details.field`。
* 业务错误断言 sentinel/typed error 到 HTTP/error code 的映射。
* 跨层字段变更断言前端 API client 类型/解析和关键 query/component 行为。

### 7. Wrong vs Correct

#### Wrong

```go
ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
```

#### Correct

```go
writeAPIError(ctx, http.StatusBadRequest, "task_invalid", "Task definition is invalid", gin.H{"field": "cronExpression"})
```


---

## 通知 API（C-2 / T2a）

全部端点走管理员 session 认证。实现：`notification_routes.go` + `notification_dto.go`。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/notifications` | 分页列表；query: `page`/`pageSize`/`read`/`severity` |
| GET | `/api/notifications/unread-count` | 未读计数 `{ count }` |
| POST | `/api/notifications/{id}/read` | 单条已读 |
| POST | `/api/notifications/read` | 批量已读；body `{ ids: number[] }` |
| POST | `/api/notifications/read-all` | 全部已读 |
| DELETE | `/api/notifications/{id}` | 删除单条 |
| DELETE | `/api/notifications/read` | 清空已读 |
| GET | `/api/notifications/sinks` | sink 状态列表 |

列表响应：

```json
{
  "notifications": [/* notificationResponse */],
  "total": 0,
  "page": 1,
  "pageSize": 20
}
```

`notificationResponse` 字段：`id`、`eventId`、`name`、`severity`、`subjectKind`、`subjectId`、`title`、`body`、`detail`、`readAt`、`occurredAt`、`createdAt`。

错误码见 [错误处理](./error-handling.md) 中 `NOTIFY_*` 表。事件模型与 Sink 契约见 [通知事件总线契约 C-2](./notification-event-contract.md)。

## 配置写入 API（C-1 / T1a）

全部端点走管理员 session 认证（`requireSession`）。实现：`config_routes.go` + `config_dto.go` + `app/config_write.go`。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/config/audio` | 安全只读；敏感字段仅 `*Configured` |
| PUT | `/api/config/:section` | 部分更新；首版 section=`audio` |
| POST | `/api/config/reload` | 从文件重载热配置（既有） |

`PUT` 成功 `data`：`config` + `applied` + `restartRequired` + `reload.subsystems[]`。

字段级错误 `error.details.fields[]`：`{ path, reason, code }`。

错误码见 [错误处理](./error-handling.md) 中 `CONFIG_*` 表。分级与落盘规则见 [配置运行时规范](./configuration-runtime-guidelines.md) C-1 节。

## 备份模板 API（T7a）

全部端点走管理员 session 认证（`requireSession`）。实现：`template_routes.go` + `template_dto.go` + `internal/template`。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/templates` | 列表（含完整参数定义）`{ "templates": [...] }` |
| GET | `/api/templates/:id` | 模板详情 |
| POST | `/api/templates/:id/render` | body `{ "params": { ... } }` → 任务草稿（不落库） |

渲染成功 `data` 与 `createTaskRequest` 字段兼容，并额外包含：

* `commandPreview`：可读命令预览（敏感值掩码）
* `templateId`：来源模板

字段级错误 `error.details.fields[]`：`{ path, reason, code }`（与 C-1 同形）。

错误码见 [错误处理](./error-handling.md) 中 `TEMPLATE_*` 表。领域实现见 `internal/template`。

---

## 前后端联动清单

修改 API contract 时，至少搜索并检查：

* `services/taskdaemon-go/internal/httpapi/*_routes.go`
* `services/taskdaemon-go/internal/httpapi/*_dto.go`
* `services/taskdaemon-go/internal/httpapi/*_test.go`
* `apps/web/src/lib/api/types.ts`
* `apps/web/src/lib/api/client.ts`
* `apps/web/src/features/**`
* `services/taskdaemon-go/internal/app/app.go` 中的 daemon API client
* `services/taskdaemon-go/internal/cli/command.go` 中的 CLI 输出摘要

跨层改动应明确数据流：

```text
HTTP request -> request DTO -> service input -> data/service state -> response DTO -> API client type -> query/component
```

---

## 测试要求

* Route 测试使用 fake service 断言 handler 是否传递正确 service input。
* API 错误测试断言 HTTP 状态码、envelope `code`、稳定 `error.code` 和安全 `details`。
* DTO 转换涉及时间、空值、运行态、退出码和截断输出时必须有测试覆盖。
* 前后端 contract 改动需要同步前端 API client 测试或关键 query/component 测试。
* CLI daemon client 改动需要覆盖 Cookie/session token、路径、成功状态码和 envelope data 解码。

---

## 禁止模式

* 不在 handler 中直接返回裸 `gin.H` 错误结构绕过 envelope。
* 不让前端依赖 `msg` 或 `error.message` 做业务分支。
* 不把 Ent entity 直接 JSON 序列化给前端。
* 不在 API response、错误 details 或测试快照中包含密码、token、Cookie、数据库密码或 runner env 值。
* 不把 `/api`、`/swagger` 等保留路径交给 SPA fallback。
