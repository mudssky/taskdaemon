# T8 Implement · 执行清单

## 前置

- [x] C-1 已冻结（T1a HANDOFF）
- [x] `prd.md` / `design.md` 就绪

## 实现顺序

1. [ ] `internal/data/store.go`：保留 `*sql.DB` + `driver`，暴露 `SQL()` / `Driver()`
2. [ ] `internal/data/transfer/`：
   - `errors.go` / `types.go` / `catalog.go`
   - `fingerprint.go` / `codec.go`
   - `export.go` / `import.go` / `transfer.go`
   - `sequence.go` / `verify.go` / `checkpoint.go`
   - 单元测试（双 SQLite E2E + schema/non-empty/dry-run/sequence）
3. [ ] `internal/cli/db_commands.go`：export/import/transfer；`command.go` 挂载
4. [ ] `docs/db-migration.md` + `error-handling.md` 增补 `DBXFER_*`
5. [ ] 路线图 §7.2 T8 状态回写（完成后）
6. [ ] `HANDOFF.md`

## 验证命令

```bash
cd services/taskdaemon-go && go test ./internal/data/transfer/ ./internal/data/ ./internal/cli/ -count=1
pnpm typecheck
pnpm vet:go
# 可选：pnpm test:go:integration
```

## 回滚

- 仅新增 transfer + CLI 命令；回滚删除对应文件并还原 `store.go` / `command.go` 小改动。
- 不改 Ent schema / 生成物。

## 不做

- Web UI、在线零停机、增量同步、MySQL、跨版本 schema 升级
- merge / archive（协调者）
