# T8 Design · SQLite ↔ PostgreSQL 迁移 CLI

## 1. 边界

| 层 | 职责 |
|---|---|
| `internal/data` | 暴露 `Store.SQL()` / `Store.Driver()`，供 transfer 做方言隔离 SQL |
| `internal/data/transfer` | 业务表目录、schema fingerprint、export/import/transfer、校验、序列复位、断点 |
| `internal/cli` | `db export|import|transfer` 子命令、flag、备份提示、进度输出 |
| `cmd/taskdaemon` | 装配 Hooks（可选；核心逻辑走 data/transfer，CLI 可直接调用） |
| `docs/db-migration.md` | 操作步骤、类型映射表、回滚、停机要求 |

**不**走 HTTP；**不**改 Ent schema。业务层不拼 SQL：SQL 仅出现在 `data/transfer`。

独占路径：`internal/cli/db_commands.go`、`internal/data/transfer/**`。  
共享 append-only：`command.go` 挂载子命令、`store.go` 暴露 DB 句柄、`error-handling.md` 增补 `DBXFER_*`、`package.json` 如需 script。

## 2. CLI 形状

沿用现有 `db` 命令组（已有 `migrate`）：

```text
taskdaemon db export  --output <path.tdxfer> [--batch-size N]
taskdaemon db import  --input  <path.tdxfer> [--force] [--dry-run] [--batch-size N] [--checkpoint <path>]
taskdaemon db transfer \
  --target-driver postgres|sqlite --target-dsn <dsn> \
  [--source-driver ...] [--source-dsn ...] \
  [--force] [--dry-run] [--batch-size N] [--checkpoint <path>]
```

- **export**：源 = 当前配置 `database`；写出中间文件。
- **import**：目标 = 当前配置 `database`；读中间文件。
- **transfer**：默认源 = 当前配置；目标由 flag 指定。等价 export+import 流式，不落盘。
- `--force`：目标库任一业务表非空时的显式确认闸门。
- `--dry-run`：只打印表与记录数，零写入。
- 凭据：输出/日志只显示 driver + 脱敏 DSN（复用 `cli.redactConfigValue` 语义）。

## 3. 中间格式（`.tdxfer`）

NDJSON 单文件（UTF-8），逐行一条 JSON，支持流式与分批：

```json
{"type":"meta","formatVersion":1,"schemaFingerprint":"<sha256>","exportedAt":"...","sourceDriver":"sqlite","tables":[{"name":"admins","count":1},...]}
{"type":"row","table":"admins","data":{"id":1,"singleton_key":"...","created_at":"2026-07-27T00:00:00Z",...}}
{"type":"eof","rowCount":N}
```

- 时间：RFC3339Nano，**一律 UTC**（`time.Time.UTC()`）。
- JSON 字段（如 `runner_config`、`detail`）：JSON 对象/数组原样编码；DB 侧 text/jsonb 由 dialect codec 互转。
- 布尔：JSON `true/false`；SQLite 读出 0/1 时规范化。
- 主键 ID **原样保留**，保证外键引用。

## 4. Schema 一致性

无独立 schema_version 表。用 Ent `migrate.Tables` 生成**应用期望 fingerprint**（有序：表名 + 列名 + 类型标签）。

流程：

1. Open 源/目标并 `Ping`。
2. 目标执行 `Store.Migrate`（确保表存在）。
3. 对源与目标分别计算 **live fingerprint**（查询 `sqlite_master` / `information_schema` 业务表列集合）。
4. 与应用期望 fingerprint 比对：缺表/缺列/多余业务表列 → `DBXFER_SCHEMA_MISMATCH`。
5. export meta 写入 fingerprint；import 时与目标再比一次。

跨版本 schema 升级 **不在本工具范围**（PRD Out of Scope）。

## 5. 表目录与顺序

业务表（与 `migrate.Tables` 对齐）：

| 顺序 | 表 | FK |
|---:|---|---|
| 1 | `admins` | — |
| 2 | `sessions` | `admin_sessions` → `admins.id` |
| 3 | `tasks` | — |
| 4 | `runs` | `task_runs` → `tasks.id` |
| 5 | `notifications` | — |
| 6 | `audio_records` | — |

导出/导入按此顺序；导入时在事务内先写父表。

## 6. 类型映射（摘要）

| 概念 | SQLite | PostgreSQL | 中间格式 |
|---|---|---|---|
| 自增 PK | INTEGER AUTOINCREMENT | SERIAL/IDENTITY | JSON number |
| bool | INTEGER 0/1 或 BOOL | boolean | JSON bool |
| time | TEXT/NUMERIC → driver time | timestamptz | RFC3339Nano UTC |
| string/text | TEXT | text | JSON string |
| JSON | TEXT | jsonb/json | JSON value |
| enum | TEXT | text/enum-as-string | JSON string |
| nullable | NULL | NULL | JSON null / 字段省略 |

导入后：

- PostgreSQL：对每张有 serial 的表 `setval(pg_get_serial_sequence(...), max(id), true)`。
- SQLite：更新 `sqlite_sequence`（若存在）使后续插入不冲突。

## 7. 安全与非空闸门

1. 启动时显著打印：**请先备份源库与目标库；迁移期间应停止 taskdaemon 服务。**
2. 目标业务表 `SUM(count)>0` 且未 `--force` → `DBXFER_TARGET_NOT_EMPTY`。
3. 连通性/权限失败 → `DBXFER_CONNECT_FAILED` / `DBXFER_PERMISSION`。
4. 错误与进度日志 **永不**打印明文 DSN/密码。

## 8. 事务、分批与断点

- 默认 `batch-size=500`。
- **小数据**（总行数 ≤ `batch-size * 20` 或未指定 checkpoint）：单事务导入；失败 Rollback，目标保持导入前状态。
- **大数据 / 指定 checkpoint**：按表分批提交；checkpoint JSON：

```json
{"status":"in_progress|completed|failed","table":"runs","offset":1000,"completedTables":["admins","sessions","tasks"],"error":"...","updatedAt":"..."}
```

- `--checkpoint` 存在且 `status=in_progress` 时可继续；`completed` 拒绝重复导入除非 `--force`。
- 中断后目标状态可判定：读 checkpoint + 各表 count。

## 9. 一致性校验

导入/transfer 成功后：

1. 按表 `COUNT(*)` 与源（或 export meta）比对。
2. 关键表（`tasks`、`admins`、`notifications`）按 `id` 排序抽样最多 20 行，比较关键列哈希（id + 主业务字段序列化）。
3. 失败 → `DBXFER_VERIFY_FAILED`，details 含表名、expected/actual count 或 sample mismatch id。

## 10. 错误码 `DBXFER_*`

| Code | 场景 |
|---|---|
| `DBXFER_SCHEMA_MISMATCH` | 源/目标/文件 fingerprint 不一致 |
| `DBXFER_TARGET_NOT_EMPTY` | 目标非空且无 `--force` |
| `DBXFER_CONNECT_FAILED` | 打开/ping 失败 |
| `DBXFER_PERMISSION` | 缺权限（无法 create/insert/setval） |
| `DBXFER_INVALID_INPUT` | flag/文件格式非法 |
| `DBXFER_IMPORT_FAILED` | 导入事务失败 |
| `DBXFER_VERIFY_FAILED` | 校验差异 |
| `DBXFER_CHECKPOINT_INVALID` | 断点损坏或不可续 |

CLI 错误信息前缀带 code，便于检索；底层 `%w` 保留。

## 11. 测试矩阵

| 场景 | 期望 |
|---|---|
| catalog 覆盖 migrate.Tables | 无遗漏 |
| SQLite → 文件 → SQLite | 全表 count + 抽样一致 |
| SQLite → 文件 → Postgres（integration tag） | 同上 + sequence 后续 insert 不冲突 |
| SQLite ↔ SQLite transfer | 双向 |
| schema 故意缺列 | `DBXFER_SCHEMA_MISMATCH` |
| 目标非空无 force | 拒绝 |
| dry-run | 零副作用 |
| 导入中途失败 | 单事务回滚为空（小数据） |
| 分批 checkpoint 续跑 | 可完成且 verify 通过 |
| 时间戳 UTC 往返 | 值相等（截断到存储精度） |
| bool / JSON / text | 值相等 |
| 日志/错误无 DSN 明文 | 断言 redacted |

单元测试优先双 SQLite 临时文件（无需 Docker）；Postgres 路径 `//go:build integration`。

## 12. 文档

`docs/db-migration.md`：前置条件、备份、停服、export/import/transfer 示例、类型映射全文、回滚（换回旧 DSN 或从备份恢复）、限制。
