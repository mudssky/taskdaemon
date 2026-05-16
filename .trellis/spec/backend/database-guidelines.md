# 数据库规范

> 数据层负责持久化边界、schema、migration 和数据库方言隔离。

---

## 概览

taskdaemon 后端使用 Ent 建模与访问数据。默认运行时使用 SQLite，同时保留 PostgreSQL 运行时和集成测试路径。业务层、HTTP handler、CLI 和 Desktop binding 不直接散落 SQL 方言判断。

Ent schema 位于 `services/taskdaemon-go/internal/data/ent/schema/`，生成入口为 `services/taskdaemon-go/internal/data/ent/generate.go`。修改 schema 后在 `services/taskdaemon-go` 内运行 `go generate ./internal/data/ent`，或从仓库根运行 `pnpm generate:go`。

---

## 数据边界

* `internal/data` 负责 Ent client、schema、migration、repository 和方言隔离。
* `internal/app` 只装配数据层依赖，不承载具体查询逻辑。
* `internal/auth`、`internal/scheduler`、`internal/httpapi` 等包通过 service/repository 边界访问数据。
* handler、runner、CLI 不直接拼 SQL，也不直接依赖数据库方言。
* Ent 生成代码留在 `internal/data/ent/`，属于生成代码；评估文件体量时不纳入拆分候选。

---

## 实体与状态

长期业务实体至少分为这些概念：

* 任务定义：cron、timezone、启停状态、runner 类型、结构化 runner 配置、timeout、overlap 策略。
* 执行记录：任务 ID、触发来源、状态、退出码、开始/结束时间、耗时、错误摘要、截断 stdout/stderr。
* 管理员账号：单管理员初始化、密码哈希、账号状态。
* 登录态/session：Cookie/session、过期时间、CSRF 关联信息。
* 运行配置或状态：只保存确实需要持久化的业务配置，不把静态配置文件内容复制进数据库。

任务当前定义与执行历史要分开建模；不要用最新执行记录反推任务配置或运行态。

---

## 方言与迁移

* 配置中的数据库方言保持稳定字符串：`sqlite`、`postgres`。
* SQLite 使用 `modernc.org/sqlite`，database/sql driver 名为 `sqlite`；打开文件型 SQLite 前确保父目录存在，并启用 foreign keys。
* PostgreSQL runtime driver 使用 `github.com/lib/pq`，配置方言字符串保持为 `postgres`。
* migration 作用于当前配置的数据库；跨数据库数据迁移不是常规 schema migration 的职责。
* PostgreSQL 集成测试使用 `testcontainers-go`，本机 Docker 不可用时可按测试约定跳过。
* schema 变更必须提交 schema 和生成代码 diff，并在任务说明中写清生成命令。

---

## 查询与事务

* 查询入口优先放在 repository/service 方法中，方法名表达业务意图，例如 `ListEnabledTasks`、`CreateRunRecord`、`MarkRunCancelled`。
* 列表和历史查询必须设置默认 limit 或分页，避免无上限返回日志与执行历史。
* 涉及多个表或多个状态转换的操作使用事务。
* 事务内只做短时间数据库状态变更，不在事务中运行外部进程。
* 外部 runner 执行完成后再开启短事务写最终状态和截断输出。
* 所有数据库路径接受并传递 `context.Context`，让 API cancel、CLI cancel 和 daemon shutdown 能向下传播。

---

## 认证数据

* 单管理员约束由数据库唯一键和服务层逻辑共同保证。
* 密码只保存 bcrypt hash。
* session token 和 CSRF token 只保存不可逆哈希；原始 token 只返回给客户端。
* session Cookie 名称、HttpOnly、SameSite 等响应行为由 HTTP/auth 边界维护，不在数据层拼响应。

---

## 禁止模式

* 不把 Ent client 泄漏到 HTTP handler、CLI command 或 Desktop binding。
* 不在保存任务时接受未分类的任意命令字符串；runner 配置必须先通过结构化校验。
* 不保存无限长 stdout/stderr；执行历史只保存截断内容和摘要。
* 不把数据库连接串、密码或 token 写入错误响应、日志或测试快照。
* 不把某个任务的阶段性实体列表作为长期扩展边界；新增实体应由当前需求和持久化理由驱动。

---

## 测试要求

* SQLite 使用临时目录或内存数据库测试，避免污染用户配置目录。
* 数据库测试断言业务结果和状态转换，不只断言 Ent 方法没有报错。
* 不支持方言、migration、session 生命周期、执行历史排序和删除级联等边界需要测试覆盖。
* PostgreSQL 集成路径用 `pnpm test:go:integration` 或 `go test -tags=integration ./internal/data`。
