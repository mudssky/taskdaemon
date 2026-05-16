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
