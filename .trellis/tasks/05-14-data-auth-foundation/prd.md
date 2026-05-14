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

* [ ] 默认 SQLite 可启动。
* [ ] PostgreSQL 可通过配置连接。
* [ ] Ent schema 生成流程可运行。
* [ ] 当前数据库 schema migration 可运行。
* [ ] 单管理员初始化/登录可用。
* [ ] 受保护 API 未登录时拒绝访问。
* [ ] 配置加载覆盖顺序明确且可测试。
* [ ] PostgreSQL 相关测试具备 testcontainers-go 集成测试入口。

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
