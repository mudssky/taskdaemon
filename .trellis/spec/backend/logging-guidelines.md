# Logging Guidelines

> How logging is done in this project.

---

## Overview

taskdaemon 后端第一版使用 Go 标准库 `log/slog` 作为日志基础。日志服务于 daemon 排障、任务执行追踪和部署问题定位；用户可见的执行历史仍写入数据库，不能只存在日志里。

当前仓库还没有正式 logging package。实现时应把 logger 作为应用依赖注入到 `internal/app`、`internal/httpapi`、`internal/scheduler`、`internal/runner` 等边界，避免在业务代码中散落全局 logger。

---

## Log Levels

* `debug`：开发期排查细节，例如配置合并来源、调度器内部决策、runner 命令构造的非敏感摘要。
* `info`：正常生命周期事件，例如 daemon 启动/停止、scheduler 启动、任务注册、migration 完成、登录成功。
* `warn`：可恢复但需要注意的情况，例如高频 cron 警告、任务 overlap 被跳过、swagger route 被关闭、配置使用默认值兜底。
* `error`：操作失败且需要排查，例如数据库连接失败、migration 失败、runner 启动失败、API handler 未能完成请求。

---

## Structured Logging

* 使用 `slog` 结构化字段，不拼接大段不可解析字符串。
* 常用字段名保持稳定：`task_id`、`run_id`、`trigger`、`status`、`duration_ms`、`exit_code`、`component`、`error`。
* request 相关日志保留 `method`、`path`、`status`、`duration_ms`，不要默认记录 body。
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

* 日志输出默认面向本地运行和守护进程部署，优先写 stdout/stderr；文件日志和轮转不是第一版范围。
* 如果未来加入系统服务安装器，再补充 Windows Service、systemd、launchd 的日志落点规范。
* Swagger UI/docs route 通过配置开关启用；关闭时可以记录一次 info，不要每个请求重复告警。

---

## Common Mistakes

* 不要只写日志而不写执行历史；UI/CLI 查询历史依赖数据库。
* 不要在循环或高频 cron 路径输出大量 info 日志；高频任务需要控制日志量。
* 不要在库函数里直接退出进程或 `panic`；返回错误给入口层记录。
