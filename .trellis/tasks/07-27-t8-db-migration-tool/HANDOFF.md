# HANDOFF · T8 SQLite ↔ PostgreSQL 迁移工具

> 任务：`07-27-t8-db-migration-tool`  
> 分支：`mudssky/t8-db-migration-tool`  
> 日期：2026-07-27  
> 状态：**实现完成** — 待协调者验收后 merge + archive（本 worker **不 merge**）

## 结论

- 提供 `taskdaemon db export|import|transfer` CLI，覆盖全部 Ent 业务表。
- 中间格式 `.tdxfer`（NDJSON）流式导出/导入；transfer 经临时文件复用同一路径。
- Schema fingerprint、目标非空闸门、`--force`、`--dry-run`、分批 checkpoint、序列复位、计数+抽样校验均已落地。
- 文档：`docs/db-migration.md`；错误码：`DBXFER_*` 写入 `.trellis/spec/backend/error-handling.md`。

## 交付物

| 路径 | 说明 |
|---|---|
| `internal/data/store.go` | 暴露 `SQL()` / `Driver()` |
| `internal/data/transfer/**` | catalog / export / import / transfer / verify / sequence / checkpoint |
| `internal/cli/db_commands.go` | `db export|import|transfer` + 既有 `migrate` |
| `internal/cli/command.go` | 挂载 `newDBCommand` |
| `docs/db-migration.md` | 操作步骤、类型映射、回滚 |
| `.trellis/spec/backend/error-handling.md` | `DBXFER_*` |
| `docs/roadmap-post-mvp-and-agent.md` | §7.2 T8 done（待 merge） |

## CLI 速查

```bash
taskdaemon db export --output ./backup.tdxfer
taskdaemon db import --input ./backup.tdxfer [--force] [--dry-run] [--checkpoint cp.json]
taskdaemon db transfer --target-driver postgres --target-dsn 'postgres://…' [--force] [--dry-run]
```

进度与错误中的 DSN 经 `redactConfigValue` 脱敏；启动打印备份/停服 WARNING。

## 验证（本 worker 已跑）

```bash
cd services/taskdaemon-go && go test ./internal/data/transfer/ ./internal/data/ ./internal/cli/ -count=1
pnpm typecheck   # 绿
pnpm vet:go      # 绿
# 可选：pnpm test:go:integration  # Postgres testcontainers
```

## 残留 / 协调者

| 项 | 说明 |
|---|---|
| merge + archive | **协调者**验收后执行 |
| Postgres 集成测 | 需 Docker；`//go:build integration` |
| 在线零停机 / Web UI / 增量 | Out of Scope |

## 给协调者

- 不要由本 worker merge。
- 合入注意：`command.go` / `store.go` 小改动 + 新包 `transfer` + docs。
- 建议合入后：`pnpm cli -- db export --dry-run` 冒烟。
