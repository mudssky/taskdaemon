# Go 多数据库数据层调研

## 背景

用户要求从第一阶段就设计多数据库能力，至少支持 SQLite 和 PostgreSQL。该项目需要兼顾：

* 本地轻量部署与单二进制体验。
* 远程/公网访问可演进。
* Fiber HTTP API、CLI、任务调度器共享同一套业务数据层。
* 任务、执行记录、通知、用户/登录态等模型会逐步增长。

## 资料来源

* Context7: `/websites/entgo_io`
* Context7: `/websites/sqlc_dev_en`
* Context7: `/websites/gorm_io`

## 候选方案

### Ent ORM / Entity Framework

优点：

* 官方文档示例覆盖 SQLite 与 PostgreSQL。
* Schema 以 Go 代码建模，能生成类型安全 API，适合早期模型快速演进。
* 支持按数据库方言定义字段类型，有助于隔离 SQLite/PostgreSQL 差异。
* 对后端 API、CLI、调度器共享数据层比较友好。

缺点：

* 引入代码生成和 ORM 抽象，需要维护生成代码流程。
* 对复杂 SQL、批量写入、数据库特有能力的控制不如 SQL-first 直接。

### sqlc SQL-first

优点：

* 从 SQL 生成类型安全 Go 代码，可控、透明、性能路径清晰。
* 官方文档覆盖 SQLite 配置和 PostgreSQL/pgx 使用。
* 复杂查询可直接写 SQL。

缺点：

* 同时支持 SQLite 与 PostgreSQL 时，schema、占位符、返回值、UPSERT、时间类型等差异会更显性。
* 早期业务模型频繁变化时，SQL 与生成代码维护成本更高。

### GORM

优点：

* 官方支持 SQLite、PostgreSQL、MySQL、SQL Server 等多种数据库。
* 上手成本低，生态成熟，Fiber + Go Web 项目中资料多。
* 支持 AutoMigrate、事务、关联、钩子、插件等完整 ORM 能力。
* 新版提供泛型 API，可获得比传统 GORM 更好的类型体验。
* 不需要 Ent 那样的代码生成流程，早期开发速度快。

缺点：

* 查询仍主要依赖运行时构造，强类型约束不如 Ent 代码生成彻底。
* AutoMigrate 适合早期迭代，长期生产迁移仍需要更严格的版本化 migration 策略。
* 如果大量使用 GORM magic、hook、关联自动加载，后续排查性能和 SQL 行为会更隐性。
* 对跨数据库方言差异的处理仍需要约束，不能假设 ORM 自动抹平所有差异。

### 仓储接口 + 方言适配

优点：

* 业务层完全不感知具体数据库。
* 可以先使用 Ent 或 sqlc 之一实现，再在仓储边界替换。

缺点：

* 过早抽象会增加样板代码。
* 如果接口粒度设计不好，会变成“ORM 外面再包一层薄皮”。

## 推荐

MVP 可在 **Ent** 和 **GORM** 之间二选一，二者都合理：

### 推荐 A：GORM + 服务/仓储边界

适合更看重成熟生态、上手速度、资料丰富和少代码生成流程的路线：

* GORM 负责模型、查询、事务和基础迁移。
* 服务层不直接暴露 GORM 细节，复杂查询集中封装。
* 早期可用 AutoMigrate，发布前再引入版本化 migration。

### 推荐 B：Ent + 服务/仓储边界

适合更看重强类型 schema、代码生成约束和模型一致性的路线：

* Ent 负责 schema 建模、迁移和类型安全查询。
* 服务层只依赖业务语义，不在业务逻辑里散落 Ent 查询细节。
* 对真正需要数据库特化的能力，集中在 data/storage 层处理。
* 测试至少覆盖 SQLite；关键数据层能力为 PostgreSQL 保留集成测试入口。

## 需要后续确认

* 是否更偏好 GORM 的成熟生态/少生成流程，还是 Ent 的强类型/代码生成约束。
* 迁移策略是使用 Ent auto migration 起步，还是从第一版就接入版本化 migration。
* SQLite 是默认本地数据库，PostgreSQL 是生产/远程部署数据库，还是两者完全等价。
