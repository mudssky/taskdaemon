# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

taskdaemon 第一版把 SQLite 和 PostgreSQL 都作为一等目标设计。默认运行时使用 SQLite，用户可通过配置切换到 PostgreSQL。数据建模与访问层使用 Ent，Ent 生成代码和数据库方言细节必须封装在 `internal/data` 边界内，业务层、HTTP handler、CLI 和 Wails 绑定不直接散落 SQL 方言判断。

当前仓库的正式 Ent schema 位于 `internal/data/ent/schema/`，生成入口为 `internal/data/ent/generate.go`。修改 schema 后使用 `go generate ./internal/data/ent` 重新生成代码。

---

## Data Boundary

* `internal/data` 负责 Ent client、schema、repository、migration 和方言隔离。
* `internal/app` 只负责组装数据层依赖，不承载具体查询逻辑。
* `internal/httpapi`、`internal/auth`、`internal/scheduler`、`internal/runner` 通过 service/repository 接口访问持久化能力。
* 不在 handler 或 runner 中直接拼接 SQL；需要数据库特定能力时，把差异集中在 `internal/data`。
* 生成的 Ent 代码保留在 `internal/data/ent/`，schema 变更随任务提交，并在 PR/任务说明中说明 `go generate ./internal/data/ent`。

---

## Core Entities

第一版至少需要覆盖这些实体或等价模型：

* 任务定义：cron 表达式、timezone、启停状态、runner 类型、结构化 runner 配置、timeout、overlap 策略。
* 执行记录：任务 ID、触发来源、状态、退出码、开始/结束时间、耗时、错误摘要、截断 stdout/stderr。
* 管理员账号：单管理员初始化、密码哈希、账号状态。
* 登录态/session：Cookie/session、过期时间、CSRF 关联信息。
* 配置或运行状态：仅保存确实需要持久化的业务配置，避免把静态配置文件内容复制进数据库。

---

## Scenario: Data/Auth Foundation

### 1. Scope / Trigger

* Trigger: 数据层 schema、migration、认证 API 和配置数据库方言形成跨层合约。
* Scope: `internal/data` 封装 Ent client 与方言映射；`internal/auth` 负责单管理员与 session；`internal/httpapi` 只依赖认证服务接口。

### 2. Signatures

* `data.Open(ctx context.Context, cfg config.DatabaseConfig) (*data.Store, error)`
* `(*data.Store).Migrate(ctx context.Context) error`
* `(*data.Store).Client() *ent.Client`
* `(*data.Store).Close() error`
* `auth.New(store *data.Store, opts auth.Options) *auth.Service`
* `(*auth.Service).InitializeAdmin(ctx context.Context, username string, password string) (auth.AdminAccount, error)`
* `(*auth.Service).Login(ctx context.Context, username string, password string, metadata auth.LoginMetadata) (auth.LoginResult, error)`
* `(*auth.Service).AuthenticateSession(ctx context.Context, token string) (auth.Principal, error)`
* `POST /api/auth/init`
* `POST /api/auth/login`
* `GET /api/auth/me`

### 3. Contracts

* 配置方言只接受 `sqlite` 与 `postgres`。`sqlite` 映射到 database/sql driver `sqlite` 和 Ent 方言 `sqlite3`；`postgres` 映射到 driver/Ent 方言 `postgres`。
* 默认 SQLite DSN 使用 `file:<path>?_fk=1`；数据层打开文件型 SQLite 前应确保父目录存在，并启用 foreign keys。
* 单管理员通过 `Admin.singleton_key` 唯一约束保证数据库层只有一个管理员；服务层创建时固定写入 `primary`。
* 密码只保存 bcrypt hash；session token 和 CSRF token 只保存 SHA-256 十六进制哈希，原始 token 只返回给客户端。
* HTTP session cookie 名称为 `taskdaemon_session`，必须设置 `HttpOnly` 和 `SameSite=Lax`。

### 4. Validation & Error Matrix

* 不支持的数据库方言 -> `data.ErrUnsupportedDriver`。
* 重复初始化管理员 -> `auth.ErrAdminAlreadyInitialized`，API 映射为 `409 admin_already_initialized`。
* 用户名或密码错误 -> `auth.ErrInvalidCredentials`，API 映射为 `401 invalid_credentials`。
* 缺失、过期或无效 session -> `auth.ErrInvalidSession`，API 映射为 `401 unauthorized`。
* API 错误响应必须保持 `{"error":{"code": "...", "message": "...", "details": null|object}}`。

### 5. Good/Base/Bad Cases

* Good: `taskdaemon db migrate` 对当前配置数据库执行 Ent migration，不初始化 Desktop。
* Base: SQLite 临时库完成 migration 后，`Admin.Query().Count(ctx)` 可返回 `0`。
* Bad: handler 直接调用 Ent client 或判断 SQL 方言；应通过 `auth.AuthService` 或后续 service/repository 边界。

### 6. Tests Required

* 数据层测试断言 SQLite open + migration 成功，并断言不支持方言返回 `ErrUnsupportedDriver`。
* 认证测试断言初始化、重复初始化、登录、session 认证和错误凭证。
* HTTP 测试断言 `/api/auth/me` 未登录返回稳定 401 JSON，`/api/auth/login` 写入 HttpOnly cookie，`/api/auth/init` 可初始化并拒绝重复初始化。
* PostgreSQL 集成测试使用 `go test -tags=integration ./internal/data`，本机 Docker 不可用时允许跳过。

### 7. Wrong vs Correct

#### Wrong

```go
client, _ := ent.Open("sqlite3", cfg.Database.DSN)
```

#### Correct

```go
store, err := data.Open(ctx, cfg.Database)
```

---

## Query Patterns

* 查询入口优先放在 repository/service 方法中，方法名表达业务意图，例如 `ListEnabledTasks`、`CreateRunRecord`、`MarkRunCancelled`。
* 任务调度相关查询需要显式区分“任务定义”和“执行记录”，避免用执行记录反推当前任务配置。
* 写入执行历史的路径必须能表达 `success`、`failed`、`timeout`、`cancelled`、`skipped` 等业务状态。
* 同一任务 overlap 检查与执行记录写入需要在业务层保持一致性；不能只依赖 gocron singleton 行为。
* 列表接口默认分页或限制数量，执行历史和日志片段不能无上限返回。

---

## Transactions

* 涉及多个表或多个状态转换的操作使用事务，例如创建任务及其初始状态、记录执行开始并更新任务运行态、取消运行并写执行结果。
* 事务函数接受 `context.Context`，使 API cancel、CLI cancel、daemon shutdown 可以向下传播。
* 事务内只做数据库状态变更，不在事务中长时间执行外部进程。
* 外部 runner 执行完成后再开启短事务写入最终状态与截断输出。

---

## Migrations

* 第一版支持对当前选定数据库执行 schema migration。
* CLI 需要提供 migration 入口，例如 `taskdaemon db migrate` 或等价子命令。
* migration 只作用于当前配置的数据库；第一版不实现 SQLite 与 PostgreSQL 之间的一键数据迁移。
* migration 行为需要有测试覆盖：默认 SQLite 可迁移，PostgreSQL 通过 `testcontainers-go` 覆盖集成路径。
* schema 变更应随任务提交，不提交只在本地生成但没有复现命令的状态。

---

## Naming Conventions

* Go package 使用 `internal/data` 作为数据层根目录。
* Ent schema 名称使用业务名词单数形式，字段名保持 Go 风格；数据库列名由 Ent 约定生成，确需自定义时集中在 schema 中说明。
* 表、索引和唯一约束命名要表达业务含义，例如任务唯一约束、session token 索引、执行记录按任务和开始时间查询索引。
* 配置中数据库方言名称使用稳定字符串，例如 `sqlite`、`postgres`，不要混用 `postgresql`、`pg` 等多个别名，除非配置层明确做兼容映射。

---

## Testing Requirements

* 数据层、migration、配置覆盖顺序、认证状态查询采用 TDD。
* SQLite 使用临时目录或内存数据库测试，避免污染用户配置目录。
* PostgreSQL 集成测试使用 `testcontainers-go`，并允许在本机缺少 Docker 时按项目约定跳过。
* 数据库测试断言业务结果，不只断言 Ent 方法没有报错。

---

## Common Mistakes

* 不要把 Ent client 泄漏到 HTTP handler、CLI command 或前端绑定层。
* 不要在保存任务时接受未分类的任意命令字符串；runner 配置必须先通过结构化校验。
* 不要把完整 stdout/stderr 当作无限长文本保存；第一版只保存截断内容。
* 不要把跨数据库迁移当成第一版 migration 范围；当前只迁移“当前选定数据库”的 schema。
