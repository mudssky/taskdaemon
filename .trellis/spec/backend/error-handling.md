# 错误处理

> 后端错误处理基于 Go 标准错误模型，并在 API/CLI 边界转换为稳定、可理解的响应。

---

## 概览

低层返回 `error`，调用方用 `errors.Is` / `errors.As` 判断可预期错误。边界层负责记录日志、映射 HTTP 状态码、输出 CLI 提示和隐藏敏感信息。

错误处理必须区分业务状态、用户输入错误和系统故障，尤其是 cron 校验、runner 执行、timeout/cancel、认证失败、配置加载和数据库连接失败。

---

## 错误分类

* 输入错误：cron 非法、timezone 不存在、runner 必填字段缺失、任务 ID 非正数。
* 软警告：秒级 cron、高频 cron 等可保存但需要显式确认的风险。
* 认证/授权错误：缺失 session、session 失效、无效凭证、重复初始化管理员。
* 执行错误：runner 非零退出、进程启动失败、timeout、cancelled、输出截断。
* 系统错误：数据库不可用、migration 失败、配置文件不可读、日志文件不可写。

软警告不能伪装成硬错误；用户取消和 timeout 也不能折叠成 generic failed。

---

## Go 错误类型

* 稳定业务分支使用 sentinel error，例如任务运行中、任务未运行、无效 session。
* 需要携带字段、退出码、stderr 摘要等结构化信息时使用 typed error。
* 返回错误时用 `%w` 包装底层错误，保留 `errors.Is` / `errors.As` 能力。
* 不为只在一处处理的普通错误创建过度抽象。
* error message 使用英文或稳定技术短语，便于日志检索；用户可见中文文案在 UI 展示层映射。

---

## API Envelope

HTTP API 使用统一 envelope。HTTP 状态码表达真实 2xx/4xx/5xx；顶层 `code` 只表达成败，`0` 成功、`1` 失败。响应头始终带 `X-Trace-Id`，响应体默认带 `traceId`。

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

* `error.code` 是前端稳定分支依据。
* `error.message` 是简短默认说明，不放敏感信息。
* `error.details` 只放可安全展示的结构化信息。
* 无内容成功响应也返回 envelope，`data` 可为 `null`。
* API 错误响应和日志不得包含密码、密码哈希、session token、CSRF token、完整 Cookie 或数据库密码。

---

## 常用映射

* 未登录、session 缺失或失效 -> HTTP 401，`unauthorized`。
* 用户名或密码错误 -> HTTP 401，`invalid_credentials`。
* 重复初始化管理员 -> HTTP 409，`admin_already_initialized`。
* 请求体为空、JSON 非法、字段缺失 -> HTTP 400，`bad_request`，`details.field` 指向具体字段。
* 任务定义校验失败 -> HTTP 400，`task_invalid`。
* 删除运行中任务 -> HTTP 409，`task_running`。
* 取消未运行任务 -> HTTP 409，`task_not_running`。
* 依赖服务未注入或不可用 -> HTTP 503，`*_unavailable`。
* 未分类系统故障 -> HTTP 500，`internal_error` 或更具体的稳定错误码。

### 通知错误码（`NOTIFY_*`，C-2 / T2a / T4）

| 码 | HTTP | 场景 |
|---|---:|---|
| `NOTIFY_NOT_FOUND` | 404 | 通知不存在 |
| `NOTIFY_INVALID_SEVERITY` | 400 | 筛选 severity 非法 |
| `NOTIFY_INVALID_PAGE` | 400 | 分页参数非法 |
| `NOTIFY_SINK_UNAVAILABLE` | 503 | 通知服务/总线未装配 |
| `NOTIFY_LIST_FAILED` | 500 | 列表查询失败 |
| `NOTIFY_UNREAD_COUNT_FAILED` | 500 | 未读计数失败 |
| `NOTIFY_MARK_READ_FAILED` | 500 | 标记已读失败 |
| `NOTIFY_CLEAR_READ_FAILED` | 500 | 清空已读失败 |
| `NOTIFY_OPERATION_FAILED` | 500 | 其他通知操作失败 |
| `NOTIFY_WEBHOOK_URL_REJECTED` | — | Webhook URL 协议/私网/Host 策略拒绝（sink 层） |
| `NOTIFY_WEBHOOK_DELIVER_FAILED` | — | Webhook 投递失败（含重试耗尽） |
| `NOTIFY_EMAIL_DELIVER_FAILED` | — | 邮件 SMTP 投递失败（含重试耗尽） |

### 配置写入错误码（`CONFIG_*`，C-1 / T1a）

| 码 | HTTP | 场景 |
|---|---:|---|
| `CONFIG_SECTION_UNKNOWN` | 404 | 未知或未开放写入的 section |
| `CONFIG_INVALID_JSON` | 400 | body 非法或缺失 |
| `CONFIG_FIELD_INVALID` | 400 | 字段格式/类型非法 |
| `CONFIG_FIELD_OUT_OF_RANGE` | 400 | 取值越界 |
| `CONFIG_FIELD_CONFLICT` | 400 | 字段与其他配置冲突 |
| `CONFIG_FIELD_NOT_WRITABLE` | 400 | file_only 或未知字段 |
| `CONFIG_VALIDATION_FAILED` | 400 | 多类字段错误聚合 |
| `CONFIG_PATH_UNAVAILABLE` | 503 | 无法解析可写路径或 writer 未装配 |
| `CONFIG_WRITE_FAILED` | 500 | 落盘失败 |
| `CONFIG_RELOAD_FAILED` | 500 | 写入后重载/Apply 失败（已回滚） |

约束：

* 校验失败**零写入**；`details.fields` 指明路径与原因。
* 敏感值永不进入响应、错误 details 或写入日志原文。
* 既有 `config_reload_failed` / `config_reload_unavailable` 保留兼容 `POST /reload`。

### 备份模板错误码（`TEMPLATE_*`，T7a）

| 码 | HTTP | 场景 |
|---|---:|---|
| `TEMPLATE_NOT_FOUND` | 404 | 未知模板 id |
| `TEMPLATE_INVALID_JSON` | 400 | body 非法 |
| `TEMPLATE_FIELD_INVALID` | 400 | 参数类型/格式/路径非法 |
| `TEMPLATE_FIELD_REQUIRED` | 400 | 缺必填参数 |
| `TEMPLATE_FIELD_OUT_OF_RANGE` | 400 | 数值越界 |
| `TEMPLATE_VALIDATION_FAILED` | 400 | 多类字段错误聚合 |
| `TEMPLATE_RUNNER_UNSUPPORTED` | 400 | runner 越界白名单（注册或渲染后） |
| `TEMPLATE_RENDER_FAILED` | 500 | 未预期渲染失败 |
| `TEMPLATE_UNAVAILABLE` | 503 | 模板服务未装配 |
| `TEMPLATE_REGISTER_REJECTED` | — | 注册期定义非法（进程内/测试） |

约束：

* 字段级错误 `error.details.fields[]` 形状与 C-1 一致：`{ path, reason, code }`，`path` 使用 `params.<name>`。
* 渲染**不落库**；敏感参数（`secret_ref`）不明文进入任务草稿或日志。
* 用户参数进入 shell 时必须经单引号字面量转义；注入载荷不得构造额外命令。

### Desktop capability 错误码（`DESKTOP_*`，D1/C-5）

Desktop Wails binding / capability 调用使用以下稳定码（非 HTTP envelope 的 `error.code` 同名字段，也可出现在 bridge `InvokeResult.code`）：

| 码 | 场景 |
|---|---|
| `DESKTOP_CAPABILITY_UNAVAILABLE` | 能力在当前平台不可用（`Available() == false` 且非权限） |
| `DESKTOP_CAPABILITY_NOT_FOUND` | 请求了未注册的 capability 名 |
| `DESKTOP_PERMISSION_DENIED` | 能力存在但权限未授予 |
| `DESKTOP_INVOKE_FAILED` | 调用本体失败，或 capability panic 被 Registry recover |

约束：

* Wails binding 路径**不得 panic**；`Registry.Invoke` 外层统一 `recover` 并映射为 `DESKTOP_INVOKE_FAILED`。
* 前端 `useDesktopCapability` 的 `invoke` 将上述码放入结构化结果，不抛异常。

---

## CLI 错误

* CLI 子命令失败返回非零 exit code。
* 输入错误输出可操作提示，例如非法 flag、配置路径、任务 ID 或缺失 session token。
* daemon/desktop 初始化失败时输出摘要，详细错误交给日志系统。
* CLI-only 模式不能因为 Desktop 初始化失败而失败，除非该命令确实需要 Desktop 能力。

---

## 禁止模式

* 不用字符串包含判断识别业务错误。
* 不把第三方库内部错误直接暴露给前端作为稳定契约。
* 不在低层重复记录同一个错误；低层返回，边界层记录。
* 不把数据库连接串、环境变量值、runner env、token、Cookie 写入错误响应。
* 不让 API `msg` 成为前端分支依据；前端只依赖稳定 `error.code`。
