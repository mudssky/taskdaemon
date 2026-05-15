# Logging Guidelines

> How logging is done in this project.

---

## Overview

taskdaemon 后端第一版使用 Go 标准库 `log/slog` 作为日志基础。日志服务于 daemon 排障、任务执行追踪和部署问题定位；用户可见的执行历史仍写入数据库，不能只存在日志里。

当前仓库的 logger 工厂位于 `services/taskdaemon-go/internal/logging`。实现时应把 logger 作为应用依赖注入到 `services/taskdaemon-go/internal/app`、`services/taskdaemon-go/internal/httpapi`、`services/taskdaemon-go/internal/scheduler`、`services/taskdaemon-go/internal/runner` 等边界，避免在业务代码中散落全局 logger。

---

## Log Levels

* `debug`：开发期排查细节，例如配置合并来源、调度器内部决策、runner 命令构造的非敏感摘要。
* `info`：正常生命周期事件，例如 daemon 启动/停止、scheduler 启动、任务注册、migration 完成、登录成功。
* `warn`：可恢复但需要注意的情况，例如高频 cron 警告、任务 overlap 被跳过、swagger route 被关闭、配置使用默认值兜底。
* `error`：操作失败且需要排查，例如数据库连接失败、migration 失败、runner 启动失败、API handler 未能完成请求。

---

## Structured Logging

* 使用 `slog` 结构化字段，不拼接大段不可解析字符串。
* 常用字段名保持稳定：`trace_id`、`task_id`、`run_id`、`trigger`、`status`、`duration_ms`、`exit_code`、`component`、`error`、`request_body`、`response_body`。
* request 相关日志保留 `method`、`path`、`status`、`duration_ms`、`trace_id`，不要默认记录 body。
* HTTP body 日志只能通过 `logging.http.includeRequestBody` / `logging.http.includeResponseBody` 显式开启；默认关闭。
* body 日志必须受 `logging.http.maxBodyBytes` 限制，超出时追加 `request_body_truncated=true` 或 `response_body_truncated=true`。
* JSON body 日志必须递归脱敏 `logging.http.redactFields` 中的字段；非 JSON body 至少要对常见 `field: value` / `field=value` 形式做保守脱敏。
* runner 日志记录脚本路径、runner 类型、工作目录、退出码和耗时；stdout/stderr 内容以数据库截断历史为准，日志里只放摘要。
* Desktop/Wails 专属日志加 `component=desktop`，不要和 HTTP API 生命周期混淆。

---

## What to Log

* 应用启动模式：`serve`、`desktop`、CLI 子命令。
* 配置文件路径、数据库方言、监听地址、swagger route 是否启用。
* schema migration 开始、成功和失败。
* 任务注册、更新、删除、启停和手动触发。
* 执行状态转换：started、success、failed、timeout、cancelled、skipped。
* 认证安全事件：登录成功/失败、session 失效、CSRF/Host 校验失败。记录账号标识时只记录非敏感 ID 或用户名。

---

## What NOT to Log

* 不记录密码、密码哈希、session token、CSRF token、Cookie、数据库密码、API key。
* 不记录完整环境变量 map；runner 环境变量只记录键名或数量。
* 不默认记录完整 stdout/stderr，避免日志膨胀和敏感信息泄漏。
* 不记录用户脚本完整内容，除非未来提供显式 debug 开关且已做敏感信息提示。
* 不在错误响应和日志中重复输出带凭证的连接串。

---

## Operational Notes

* 日志输出由 `logging` 配置段控制，支持 `console`、`file`、`both`。
* console 默认使用 text 格式，但由项目自定义 pretty handler 输出，方便本地阅读；file 默认使用 JSON，方便采集和检索。
* console 中 `http request` 使用访问日志风格单行输出：`WARN 15:04:05 POST /api/auth/init 400 3ms trace=<id> req=<body> res=<body>`。body map 必须格式化为紧凑 JSON，不能出现 Go 默认 `map[...]` 文本。
* 文件日志使用 `gopkg.in/natefinch/lumberjack.v2` 轮转，只使用其原生按大小轮转、备份数量、保留天数和压缩能力，不二次开发按日期切分。
* 文件日志路径不可写时应用应 fail fast，避免用户误以为日志已经落盘。
* 响应头始终写入 `X-Trace-Id`；响应 body 默认写入 `traceId`，可通过 `observability.traceId.includeInResponse=false` 关闭。
* 本地开发排查 body 时，优先把 `logging.http` 覆盖写入 `taskdaemon.local.yaml`，不要提交带 body 日志开关的共享配置。
* 运行中的 daemon 支持通过 `taskdaemon config reload` 热更新 `logging.http` 与 `observability.traceId`；`logging.level`、`logging.output`、文件路径和轮转配置仍需重启，避免运行中替换 handler/文件句柄导致日志丢失。
* 如果未来加入系统服务安装器，再补充 Windows Service、systemd、launchd 的日志落点规范。
* Swagger UI/docs route 通过配置开关启用；关闭时可以记录一次 info，不要每个请求重复告警。

## HTTP Body Logging Contract

### 1. Scope / Trigger

* Trigger: 新增或修改 HTTP 请求日志、`logging.http` 配置、body 捕获/脱敏逻辑时必须遵守。

### 2. Signatures

* `logging.http.includeRequestBody: bool`，默认 `false`。
* `logging.http.includeResponseBody: bool`，默认 `false`。
* `logging.http.maxBodyBytes: int`，默认 `4096`。
* `logging.http.redactFields: []string`，默认至少包含 `password`、`token`、`cookie`、`authorization`、`csrf` 相关字段。
* 环境变量别名：`TASKDAEMON_LOGGING_HTTP_INCLUDE_REQUEST_BODY`、`TASKDAEMON_LOGGING_HTTP_INCLUDE_RESPONSE_BODY`、`TASKDAEMON_LOGGING_HTTP_MAX_BODY_BYTES`、`TASKDAEMON_LOGGING_HTTP_REDACT_FIELDS`。

### 3. Contracts

* 默认请求日志只输出摘要字段，不输出 `request_body` 或 `response_body`。
* 开启请求体日志后，日志字段为 `request_body`；开启响应体日志后，日志字段为 `response_body`。
* 截断字段固定为 `request_body_truncated`、`response_body_truncated`。
* 捕获请求体后必须恢复 `ctx.Request.Body`，保证 handler 仍能读取完整 body。
* 捕获响应体时必须透传原始写出行为，不能改变状态码、响应头或响应内容。

### 4. Validation & Error Matrix

| Condition | Behavior |
|---|---|
| body 日志配置未开启 | 不记录 body 字段 |
| JSON body 可解析 | 递归脱敏后输出结构化字段 |
| body 非 JSON 或截断后不可解析 | 输出截断字符串并做保守文本脱敏 |
| body 超出 `maxBodyBytes` | 只记录前 N 字节并追加 `*_truncated=true` |
| `maxBodyBytes <= 0` | 使用默认值，不允许无限制记录 |

### 5. Good/Base/Bad Cases

* Good: 本地 `taskdaemon.local.yaml` 开启 body 日志，日志中密码字段显示 `[REDACTED]`。
* Base: 默认配置下只看到 `method/path/status/duration_ms/trace_id`。
* Bad: 在共享配置中默认开启 body 日志，或日志里出现真实密码、session token、Cookie。

### 6. Tests Required

* 默认配置不输出 body 字段。
* 开启请求/响应 body 后输出脱敏字段。
* 超长 body 输出截断标记。
* handler 在 request body 被日志中间件预读后仍能读取完整 body。

### 7. Wrong vs Correct

#### Wrong

```go
body, _ := io.ReadAll(ctx.Request.Body)
logger.Info("http request", "request_body", string(body))
ctx.Next()
```

#### Correct

```go
// 只预读有限字节用于日志，并把预读片段和剩余原始流组合回请求体。
captured := captureRequestBody(ctx, cfg.MaxBodyBytes)
ctx.Next()
logger.Info("http request", "request_body", sanitizedLogBody(captured.content, cfg.RedactFields))
```

---

## Common Mistakes

* 不要只写日志而不写执行历史；UI/CLI 查询历史依赖数据库。
* 不要在循环或高频 cron 路径输出大量 info 日志；高频任务需要控制日志量。
* 不要在库函数里直接退出进程或 `panic`；返回错误给入口层记录。
