# SQLite ↔ PostgreSQL 数据迁移

> 对应任务 T8。本工具迁移 **业务数据**，不做跨版本 schema 升级（schema 变更请用 `taskdaemon db migrate`）。

## 前置条件

1. **先备份**源库与目标库（文件拷贝 / `pg_dump`）。
2. **停止** taskdaemon 服务（HTTP/Desktop/daemon），避免迁移中并发写入。
3. 源库与目标库应能通过当前二进制的 Ent schema（`db migrate`）对齐。
4. 配置中的 `database.driver` / `database.dsn` 与 C-1 一致：文件 + env；CLI 输出会对 DSN 脱敏。

## 命令

根命令：`taskdaemon db …`（或 `pnpm cli -- db …`）。

### export

从**当前配置数据库**导出中间文件 `.tdxfer`（NDJSON）：

```bash
taskdaemon db export --output /path/backup.tdxfer
taskdaemon db export --output /path/backup.tdxfer --dry-run   # 只打印表与行数
```

### import

将 `.tdxfer` 导入**当前配置数据库**：

```bash
taskdaemon db import --input /path/backup.tdxfer
taskdaemon db import --input /path/backup.tdxfer --force      # 目标非空时显式确认覆盖
taskdaemon db import --input /path/backup.tdxfer --dry-run
# 大数据分批 + 断点：
taskdaemon db import --input /path/backup.tdxfer --batch-size 500 --checkpoint /path/cp.json
```

目标库任一业务表非空且未加 `--force` 时返回 `DBXFER_TARGET_NOT_EMPTY`。

### transfer

源默认当前配置；目标由 flag 指定：

```bash
# SQLite → PostgreSQL
taskdaemon db transfer \
  --target-driver postgres \
  --target-dsn 'postgres://user:pass@127.0.0.1:5432/taskdaemon?sslmode=disable' \
  --force

# 显式源
taskdaemon db transfer \
  --source-driver sqlite --source-dsn 'file:/path/taskdaemon.db?_fk=1' \
  --target-driver postgres --target-dsn 'postgres://…' \
  --dry-run
```

CLI 日志只打印 `driver=` 与脱敏后的 `dsn=`。

## 业务表与顺序

| 顺序 | 表 | 外键 |
|---:|---|---|
| 1 | `admins` | — |
| 2 | `sessions` | `admin_sessions` → `admins.id` |
| 3 | `tasks` | — |
| 4 | `runs` | `task_runs` → `tasks.id` |
| 5 | `notifications` | — |
| 6 | `audio_records` | — |

主键 ID **原样保留**；导入后复位 PostgreSQL `setval` / SQLite `sqlite_sequence`。

## 类型映射

| 概念 | SQLite | PostgreSQL | 中间格式 |
|---|---|---|---|
| 自增 PK | INTEGER | serial/identity | JSON number |
| bool | 0/1 或 bool | boolean | JSON bool |
| time | driver time | timestamptz | RFC3339Nano **UTC** |
| string/text | TEXT | text | JSON string |
| JSON | TEXT | jsonb/json | JSON object/array |
| enum | TEXT | text | JSON string |
| NULL | NULL | NULL | JSON null |

## Schema 一致性

工具用 Ent `migrate.Tables` 生成 **schema fingerprint**。源/目标/文件 fingerprint 不一致 → `DBXFER_SCHEMA_MISMATCH`。  
跨版本列变更不在本工具范围：请先统一到同一版本二进制并 `db migrate`。

## 事务、断点与状态判定

- 小数据：默认单事务导入，失败 Rollback，目标保持导入前状态。
- 指定 `--checkpoint` 或数据量超过阈值：按表分批提交；断点 JSON 字段 `status=in_progress|completed|failed`、`table`、`offset`、`completedTables`。
- 中断后：读 checkpoint + 各表 `COUNT(*)` 可判定完成/未开始/中断于某表某批。

## 一致性校验

导入/transfer 成功后：

1. 按表记录数与源（export meta）比对。
2. 关键表 `admins` / `tasks` / `notifications` 抽样内容比对（`VerifyStores`）。
3. 失败 → `DBXFER_VERIFY_FAILED`，报告含表名与 expected/actual。

## 错误码 `DBXFER_*`

| Code | 含义 |
|---|---|
| `DBXFER_SCHEMA_MISMATCH` | schema fingerprint 不一致 |
| `DBXFER_TARGET_NOT_EMPTY` | 目标非空且无 `--force` |
| `DBXFER_CONNECT_FAILED` | 连接/ping 失败 |
| `DBXFER_PERMISSION` | 权限不足（migrate/insert/setval） |
| `DBXFER_INVALID_INPUT` | flag 或文件格式非法 |
| `DBXFER_IMPORT_FAILED` | 导入失败 |
| `DBXFER_VERIFY_FAILED` | 校验失败 |
| `DBXFER_CHECKPOINT_INVALID` | 断点损坏或不可续 |

## 回滚

1. **未切换配置**：目标库丢弃即可（重建空库 + migrate，或从备份恢复）。
2. **已切换 `database.dsn` 到新库但数据有问题**：把配置改回旧 DSN/文件，从备份恢复旧库。
3. **保留 export 文件**：可对干净目标重新 `import`。

## 推荐流程（SQLite → PostgreSQL）

```bash
# 1. 停服 + 备份 SQLite 文件与（如有）旧 PG
# 2. 准备空 PostgreSQL，确认网络/账号
# 3. dry-run
taskdaemon --config ./config.yaml db transfer \
  --target-driver postgres --target-dsn "$PG_DSN" --dry-run
# 4. 执行
taskdaemon --config ./config.yaml db transfer \
  --target-driver postgres --target-dsn "$PG_DSN"
# 5. 修改配置 database.driver/dsn 指向 PG，启动服务冒烟
```

或分步：`export` → 检查文件 → 改配置到目标 → `import`。

## 限制

- 无 Web UI、无在线零停机、无增量同步。
- 不迁移 MySQL 或其他方言。
- 不负责跨版本 schema 升级。
- YAML 注释与配置格式不在本工具范围（见 C-1）。
