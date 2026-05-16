# 后端质量规范

> 后端代码应让业务状态可测试、可追踪，并避免把 taskdaemon 的业务语义藏进第三方库行为里。

---

## 概览

Go 后端的质量重点是：调度行为、runner 生命周期、持久化状态、认证边界、HTTP API 和 CLI/daemon 交互。规范只描述长期维护必须遵守的工程约定；单个功能的接口全集和验收矩阵留在对应任务 PRD。

---

## 必须遵守

* `services/taskdaemon-go/cmd/taskdaemon` 只做入口分发，不承载业务逻辑。
* 新增或修改 Go 函数时，按项目要求写清参数和返回值说明；复杂业务逻辑用中文注释解释设计意图。
* 接受外部请求、进程执行、调度触发、daemon shutdown 的路径优先接受 `context.Context`。
* scheduler/runner/data/auth/httpapi/cli 的错误分支必须可测试，不能只依赖日志判断行为。
* CLI 中会影响运行中 daemon 状态的动作必须通过 daemon HTTP API，不在短生命周期 CLI 进程里启动临时 scheduler。
* runner 配置保持结构化字段，不提供裸任意命令入口作为长期抽象。
* 任务运行状态、执行历史、认证 session 等业务状态以服务层为边界，不由 UI 或 CLI 绕过服务层直接推断。

---

## 调度与 Runner

* 使用 `github.com/go-co-op/gocron/v2` 作为进程内调度器。
* 同一任务 overlap 是 taskdaemon 业务规则：业务层先记录 `skipped`，再让 gocron skip 当前 run。
* gocron singleton/limit 只能作为防御性保护，不能作为执行历史来源。
* `running` 是 daemon 进程内 scheduler 视图，不写入数据库，不从最新 run 记录反推。
* timeout、cancel、非零退出和进程启动失败必须映射成明确执行状态，并写入执行历史。
* cron 校验区分硬错误和软警告：非法表达式阻止保存，秒级/高频表达式需要显式确认。

---

## HTTP API 与 CLI

* HTTP handler 负责请求绑定、权限校验、错误映射和 DTO 转换；业务判断放在 service 层。
* 受保护 API 统一走 session middleware，不在单个 handler 中重复解析 Cookie。
* API 返回稳定错误码给前端分支，错误 message 只作为默认说明。
* CLI task 类命令携带 `taskdaemon_session` Cookie 调用 daemon API；token 来源可以是 flag 或环境变量。
* CLI 输出用于人读时可用 YAML/表格，机器契约变更必须先在 PRD 或 spec 中说明。

---

## 测试要求

* Go 业务逻辑优先写 package-level 单元测试或集成测试，断言业务结果而不是第三方库内部行为。
* scheduler 测试覆盖 cron 5/6 字段、timezone、动态注册/移除、overlap skip、timeout、cancel 和 run history 排序。
* runner 测试覆盖命令构造、不同 runner 类型、context cancel、timeout、非零退出、stdout/stderr 截断。
* HTTP 测试覆盖认证、请求体错误、稳定错误码、DTO 映射和关键状态转换。
* CLI/app 测试覆盖参数解析、session token 传递、daemon API 请求路径和错误提示。
* 配置文件、Wails 配置、nginx/Dockerfile 等静态配置不要求单元测试；只在行为有代码承载时测试行为。
* 使用真实 timer 的测试必须设置明确超时；能用 fake clock 时优先 fake clock。

---

## 禁止模式

* 不在 handler、CLI 或 Desktop binding 中直接拼接 SQL 或调用 Ent 生成 query 绕过服务边界。
* 不用字符串包含判断识别业务错误；使用 sentinel error、typed error 或稳定 API error code。
* 不把 gocron、Gin、Cobra、Wails 等第三方库的默认行为当成产品业务语义。
* 不把密码、session token、CSRF token、Cookie、数据库密码或 runner env 值写入日志、错误响应或测试快照。
* 不为单个任务的一次性验收标准新增长期规范条目。

---

## 质量命令

常用命令从仓库根目录运行：

* `pnpm test:go`：运行 Go 测试。
* `pnpm vet:go`：运行 Go vet。
* `pnpm generate:go`：Ent schema 变更后重新生成代码。
* `pnpm test`：运行前端 typecheck/lint 和 Go test。

---

## 评审清单

* 业务逻辑是否在 `internal/*` 合适包内，而不是入口层。
* 错误分支是否有稳定错误类型或错误码。
* 执行历史是否覆盖 `success`、`failed`、`timeout`、`cancelled`、`skipped` 等业务状态。
* 新增文件是否遵守 [Go 文件组织与拆分](./file-organization.md)。
* 测试是否断言业务结果和跨层契约，而不是只验证无错误返回。
