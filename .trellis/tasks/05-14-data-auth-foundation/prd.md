# 数据层与认证基础

## Goal

建立第一版数据层和单管理员认证基础，为任务定义、执行历史、登录态和多数据库支持提供稳定底座。

## Requirements

* 使用 Ent 定义核心 schema。
* 使用 Koanf 读取配置文件、环境变量和 CLI flag。
* 默认 SQLite，PostgreSQL 可通过配置启用。
* 支持对当前选定数据库执行 schema migration。
* 不实现 SQLite 与 PostgreSQL 之间的数据自动迁移。
* 实现单管理员初始化/登录。
* 未登录请求不能创建、编辑、删除、启停或触发任务。
* 为公网可演进场景保留登录态、CSRF/Host 校验和部署边界设计。
* 后端测试使用 `testing` + `testify`；PostgreSQL 集成测试使用 `testcontainers-go`。

## Acceptance Criteria

* [x] 默认 SQLite 可启动。
* [x] PostgreSQL 可通过配置连接。
* [x] Ent schema 生成流程可运行。
* [x] 当前数据库 schema migration 可运行。
* [x] 单管理员初始化/登录可用。
* [x] 受保护 API 未登录时拒绝访问。
* [x] 配置加载覆盖顺序明确且可测试。
* [x] PostgreSQL 相关测试具备 testcontainers-go 集成测试入口。

## Implementation Notes

* Ent 生成入口为 `internal/data/ent/generate.go`，复现命令为 `go generate ./internal/data/ent`。
* SQLite 使用 `modernc.org/sqlite` 的 `sqlite` driver，数据层在 `internal/data` 内映射为 Ent `sqlite3` 方言。
* PostgreSQL 使用 `github.com/lib/pq` 的 `postgres` driver。
* 兼容 Go `1.22.6` 的依赖版本为：`entgo.io/ent v0.13.1`、`modernc.org/sqlite v1.34.5`、`github.com/testcontainers/testcontainers-go v0.35.0`。
* PostgreSQL 集成测试入口为 `go test -tags=integration ./internal/data`。

## Testing Constraints

* 数据层、认证、配置加载、受保护 API 采用 TDD。
* PostgreSQL 集成行为使用 testcontainers-go。
* 静态配置文件和文档不要求单元测试。

## Out of Scope

* 不实现多用户、RBAC 或多租户。
* 不实现跨数据库数据迁移。
* 不实现完整设置页。

## Parent

* `.trellis/tasks/05-14-cross-platform-scheduler-daemon/prd.md`
