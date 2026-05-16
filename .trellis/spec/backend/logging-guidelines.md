# 日志规范

> 日志用于 daemon 排障、任务执行追踪和部署问题定位；用户可见执行历史仍写入数据库。

---

## 概览

taskdaemon 后端使用 Go 标准库 `log/slog`。logger 作为应用依赖注入到 `internal/app`、`internal/httpapi`、`internal/scheduler`、`internal/runner` 等边界，避免业务代码散落全局 logger。

日志规范描述长期行为；具体某次排障需要的临时字段不写入 spec，除非已经成为长期运维契约。

---

## 日志级别

* `debug`：开发期排查细节，例如配置合并来源、调度内部决策、runner 命令构造的非敏感摘要。
* `info`：正常生命周期事件，例如 daemon 启动/停止、scheduler 启动、任务注册、migration 完成、登录成功。
* `warn`：可恢复但需要注意的情况，例如高频 cron、任务 overlap 被跳过、配置使用默认值兜底。
* `error`：操作失败且需要排查，例如数据库连接失败、migration 失败、runner 启动失败、API handler 未能完成请求。

---

## 结构化字段

* 使用 `slog` 结构化字段，不拼接大段不可解析字符串。
* 常用字段名保持稳定：`trace_id`、`task_id`、`run_id`、`trigger`、`status`、`duration_ms`、`exit_code`、`component`、`error`、`request_body`、`response_body`。
* HTTP 请求日志保留 `method`、`path`、`status`、`duration_ms`、`trace_id`。
* runner 日志记录 runner 类型、脚本路径、工作目录、退出码和耗时；stdout/stderr 以数据库截断历史为准，日志里只放摘要。
* Desktop/Wails 专属日志加 `component=desktop`，不要和 HTTP API 生命周期混淆。

---

## HTTP Body 日志

* 默认不记录请求体或响应体。
* 只有通过 `logging.http.includeRequestBody` / `logging.http.includeResponseBody` 显式开启时才记录 body。
* body 捕获必须受 `logging.http.maxBodyBytes` 限制，超出时追加 `request_body_truncated=true` 或 `response_body_truncated=true`。
* 捕获请求体后必须恢复 `ctx.Request.Body`，保证 handler 仍能读取完整 body。
* 捕获响应体时必须透传原始写出行为，不能改变状态码、响应头或响应内容。
* JSON body 日志递归脱敏 `logging.http.redactFields` 中的字段；非 JSON body 做保守文本脱敏。
* `maxBodyBytes <= 0` 使用默认值，不允许无限制 body 日志。

---

## 不记录

* 密码、密码哈希、session token、CSRF token、Cookie、数据库密码、API key。
* 完整环境变量 map；runner env 只记录键名或数量。
* 默认不记录完整 stdout/stderr。
* 默认不记录用户脚本完整内容。
* 带凭证的连接串不进入错误响应或日志。

---

## 配置与运行时

* 日志输出由 `logging` 配置段控制，支持 console、file、both。
* console 默认面向本地阅读；file 默认 JSON，便于采集和检索。
* 文件日志路径不可写时应用 fail fast，避免用户误以为日志已经落盘。
* 响应头始终写入 `X-Trace-Id`；响应体 traceId 可由配置关闭。
* 运行中 daemon 可热更新 `logging.http` 与 `observability.traceId`。
* `logging.level`、`logging.output`、文件路径和轮转配置变更需要重启，避免运行中替换 handler 或文件句柄。
* 本地排查 body 时优先写入 `taskdaemon.local.yaml`，不要提交带 body 日志开关的共享配置。

---

## 测试要求

* 默认配置不输出 body 字段。
* 开启请求/响应 body 后输出脱敏字段。
* 超长 body 输出截断标记。
* handler 在 request body 被日志中间件预读后仍能读取完整 body。
* 文件日志路径不可写时能返回可诊断错误。

---

## 禁止模式

* 不只写日志而不写执行历史；UI/CLI 查询历史依赖数据库。
* 不在循环或高频 cron 路径输出大量 info 日志。
* 不在库函数里直接退出进程或 panic；返回错误给入口层记录。
* 不在共享配置中默认开启 HTTP body 日志。
