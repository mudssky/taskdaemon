# Error Handling

> How errors are handled in this project.

---

## Overview

taskdaemon 后端用 Go 标准错误模型作为基础：底层返回 `error`，调用方用 `errors.Is` / `errors.As` 判断可预期错误，并在 API/CLI 边界转换为用户可理解的响应。错误处理需要区分业务状态、用户输入错误和系统故障，尤其是 cron 校验、runner 执行、timeout/cancel、认证失败和数据库连接失败。

当前真实代码示例位于 `services/taskdaemon-go/internal/scheduler/gocron_spike_test.go`：spike 使用 sentinel error `errTaskAlreadyRunning` 表达同任务已经运行，并通过 gocron 的 skip hook 让业务层记录 `skipped`。

---

## Error Categories

* 输入错误：cron 语法非法、字段数量非法、timezone 不存在、runner 必填字段缺失。这类错误阻止保存。
* 软警告：秒级 cron、高频表达式等可保存但需要显式确认的风险。这类结果不应伪装成硬错误。
* 认证/授权错误：未登录、session 失效、CSRF/Host 校验失败。受保护 API 必须拒绝创建、编辑、删除、启停或触发任务。
* 执行错误：runner 非零退出、进程启动失败、timeout、cancelled、输出截断等。需要写入执行历史。
* 系统错误：数据库不可用、migration 失败、配置文件不可读、不可恢复的依赖初始化失败。需要记录日志并返回稳定错误摘要。

---

## Error Types

* 业务可判定错误优先定义 sentinel error 或小型 typed error，放在拥有该业务概念的 package 内。
* sentinel error 用于稳定分支，例如 `errTaskAlreadyRunning` 这类状态判断。
* typed error 用于携带字段级校验信息、runner 退出码、stderr 摘要等结构化数据。
* 不为只在一处处理的普通错误创建过度抽象；直接 wrap 原始错误即可。
* error message 用英文或稳定技术短语便于日志检索；面向用户的中文文案在 UI/API 展示层映射。

---

## Error Handling Patterns

* Go 代码返回错误时使用 `%w` 包装底层错误，保留 `errors.Is` / `errors.As` 能力。
* 每个接受 `context.Context` 的路径都要识别 `context.Canceled` 和 `context.DeadlineExceeded`，并分别映射为 `cancelled` 或 `timeout`。
* runner 执行完成后，把进程退出错误转换为执行状态与错误摘要，不把原始错误字符串直接暴露给前端。
* 调度 overlap 路径必须先写业务可见的 `skipped` 记录，再返回 sentinel error 跳过本次运行。
* 不在低层重复记录同一个错误；低层返回错误，边界层负责日志和响应。

---

## API Error Responses

第一版 API 错误响应保持稳定结构，便于前端表单和 CLI 复用：

```json
{
  "error": {
    "code": "cron_invalid",
    "message": "Cron expression is invalid",
    "details": {
      "field": "cron"
    }
  }
}
```

* `code` 是稳定机器码，前端根据它决定字段错误、toast 或确认对话框。
* `message` 是简短默认说明，不承载敏感信息。
* `details` 只放可安全展示的结构化信息，例如字段名、警告类型、允许范围。
* 软警告使用不同响应或字段表达 `warnings`，不能和硬错误混在同一个阻断结果里。

### Auth API Error Contracts

认证 API 必须把认证服务 sentinel error 映射为稳定错误码：

| Condition | Service error | HTTP | API code |
|---|---|---:|---|
| 未提供 session cookie、session 过期或 token 无效 | `auth.ErrInvalidSession` | 401 | `unauthorized` |
| 用户名或密码错误 | `auth.ErrInvalidCredentials` | 401 | `invalid_credentials` |
| 已存在管理员时再次初始化 | `auth.ErrAdminAlreadyInitialized` | 409 | `admin_already_initialized` |
| 认证服务未注入 | n/a | 503 | `auth_unavailable` |

未认证响应固定为：

```json
{
  "error": {
    "code": "unauthorized",
    "message": "Authentication required",
    "details": null
  }
}
```

错误响应和日志不得包含密码、密码哈希、session token、CSRF token 或完整 Cookie。

---

## CLI Error Handling

* CLI 子命令失败时返回非零 exit code。
* 输入错误输出可操作的提示，例如指出非法 flag、配置路径或任务 ID。
* daemon/desktop 入口初始化失败时输出摘要，并把详细错误交给日志系统。
* CLI-only 模式不能因为 Desktop 初始化失败而失败，除非该子命令确实需要 Desktop 能力。

---

## Common Mistakes

* 不要用字符串包含判断来识别业务错误；使用 sentinel error、typed error 或稳定错误码。
* 不要把 timeout 和用户取消都记录成 generic failed。
* 不要把数据库连接串、session token、环境变量值、runner 环境变量写入错误响应。
* 不要让 gocron 或其他第三方库的内部错误直接决定 taskdaemon 的业务执行状态。
