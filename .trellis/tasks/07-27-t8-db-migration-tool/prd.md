# T8 SQLite 与 PostgreSQL 迁移工具

> **轨**：Track T ｜ **依赖**：**C-1 冻结**（T1a 配置写入契约）
> **性质**：纯后端 CLI 任务，无前端工作。

## Goal

提供在 SQLite 与 PostgreSQL 之间迁移 taskdaemon 数据的 CLI 能力，让用户可以从单机 SQLite 起步、按需升级到 PostgreSQL，或反向降级。

## Context

一期 Out of Scope 列了「SQLite ↔ PostgreSQL 一键迁移」，来源：`.trellis/tasks/archive/2026-05/05-14-cross-platform-scheduler-daemon/prd.md`。

- 数据层：`internal/data/`（Ent，自动 migration）
- 数据库规范：`.trellis/spec/backend/database-guidelines.md`
- 已有 PostgreSQL 集成测试：`internal/data/postgres_integration_test.go`
- 错误码前缀：`DBXFER_*`（路线图 §9.1 已分配）
- 独占路径：`internal/cli/db_commands.go`（新建）、`internal/data/transfer/**`（新建）

## Requirements

### R1 CLI 子命令

- `export`：从当前配置的数据库导出到中间格式文件
- `import`：从中间格式文件导入到当前配置的数据库
- `transfer`：源库直接到目标库（等价于 export + import，但不落中间文件）
- 沿用现有 CLI 分发模式，不另起一套

### R2 迁移正确性（核心）

- 覆盖全部业务 entity，不遗漏任何表
- **类型映射明确**：时间戳、布尔、JSON、大文本、自增主键在两种数据库间的映射规则必须逐项文档化
- 外键关系与引用完整性在迁移后保持正确
- 自增序列在 PostgreSQL 侧迁移后正确复位（否则后续插入会主键冲突）
- 时区处理明确：存储时区、转换规则不能靠默认行为

### R3 前置检查与安全

- 迁移前检查目标库 schema 版本与源库一致，不一致返回 `DBXFER_SCHEMA_MISMATCH`
- 目标库非空时**必须显式确认**才继续，默认拒绝（防止误覆盖生产数据）
- 迁移前检查目标库连通性与权限
- 建议用户先备份，并在输出中显著提示
- 数据库凭据不进日志，与 `internal/httpapi/redaction.go` 脱敏边界一致

### R4 事务与失败处理

- 导入在事务中进行，失败可回滚，不留半迁移状态
- 若数据量导致单事务不可行，必须分批且**明确记录断点**，支持从断点继续
- 迁移中断后目标库状态可判定（完成/未开始/中断于某表某批次）

### R5 一致性校验

- 迁移后按表校验记录数一致
- 关键表提供抽样内容校验（不只是数量对得上）
- 校验失败返回明确的差异报告，指出哪张表、差多少

### R6 可用性

- `--dry-run`：打印将要迁移的表与预估记录数，零副作用
- 进度输出：当前表、已处理记录数、总数
- 大数据量分批处理，不整体读入内存

### R7 文档

- 迁移操作步骤、前置条件、回滚方式、类型映射表写入文档
- 明确说明迁移期间服务应停止（或说明可在线迁移的条件）

## Acceptance Criteria

- [ ] `export` / `import` / `transfer` 三个子命令可用
- [ ] 全部业务 entity 被覆盖，无遗漏（用 schema 对照测试证明）
- [ ] 类型映射表完整并写入文档
- [ ] 时间戳在两个方向迁移后值正确（含时区）
- [ ] 布尔、JSON、大文本字段迁移后值正确
- [ ] 外键与引用完整性在迁移后保持
- [ ] PostgreSQL 自增序列迁移后正确复位，后续插入无主键冲突（用测试证明）
- [ ] schema 版本不一致时返回 `DBXFER_SCHEMA_MISMATCH`
- [ ] 目标库非空时默认拒绝，需显式确认
- [ ] 目标库连通性与权限在迁移前被检查
- [ ] 输出中有显著的备份提示
- [ ] 数据库凭据不进日志
- [ ] 导入失败可回滚，不留半迁移状态
- [ ] 分批场景下断点可记录且可继续
- [ ] 中断后目标库状态可判定
- [ ] 迁移后按表记录数校验通过
- [ ] 关键表抽样内容校验通过
- [ ] 校验失败输出明确差异报告
- [ ] `--dry-run` 打印表与预估记录数，零副作用
- [ ] 进度输出可见
- [ ] 大数据量不整体读入内存（用大数据集验证）
- [ ] SQLite → PostgreSQL 双向迁移各有端到端测试
- [ ] 迁移文档完成，含类型映射表与回滚方式
- [ ] `pnpm test:go`、`pnpm vet:go`、`pnpm test:go:integration` 全绿

## Out of Scope

- 迁移的 Web UI
- 在线零停机迁移
- 增量/持续同步
- 迁移到 MySQL 或其他数据库
- 跨版本 schema 升级（那是 migration 的职责，不是本工具）

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**消费** | **C-1**（数据库配置的读取方式与敏感值处理约定） |
| 本任务**产出** | 无契约冻结职责 |
| 共享文件 | `services/taskdaemon-go/package.json`（追加 script） |

## Design / Implement 补齐条件

`design.md` 与 `implement.md` **待 C-1 冻结后补**。原因：源库/目标库连接配置的读取与凭据处理方式必须与 C-1 一致。

## Definition of Done

- Acceptance Criteria 全部勾选
- 双向端到端迁移测试通过
- 路线图 §7.2 状态已回写

## Notes

- **自增序列复位是最容易漏的一项**，PostgreSQL 侧迁移后不复位会导致后续插入全部主键冲突，测试必须覆盖
- 类型映射表是本任务最有长期价值的产物，写详细
- 「目标库非空默认拒绝」是防误删的关键闸门，不要为了方便去掉
