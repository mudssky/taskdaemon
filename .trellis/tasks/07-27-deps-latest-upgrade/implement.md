# 依赖全量升级 — 执行计划

## 前置

- [ ] 读 `prd.md` / `design.md`
- [ ] `git fetch && git checkout -b feat/deps-latest-upgrade`（基于最新 `dev`）
- [ ] 向协调者确认：T2a 是否仍占用 Ent/go.mod（若是，Go 生成链延后或只升无关模块）
- [ ] 保存基线：`pnpm outdated -r`、`go list -m -u` 直接依赖 → `HANDOFF.md` 附录「升级前」

## 阶段 1 · JS/TS

- [ ] `pnpm up -r --latest`（失败则按 workspace 分包升级）
- [ ] `pnpm install`
- [ ] `pnpm typecheck`
- [ ] `pnpm lint`（必要且最小的 format 修复）
- [ ] `pnpm --filter @taskdaemon/web test`
- [ ] 记录无法到 latest 的包与原因

## 阶段 2 · Go modules

- [ ] 列出 `go.mod` require 直接依赖
- [ ] 逐个或批量 `go get <module>@latest`（注意 ent/wails/atlas）
- [ ] `go mod tidy`
- [ ] 若 ent/atlas 变：`pnpm generate:go`
- [ ] `cd services/taskdaemon-go && go test ./...`（或 `pnpm test:go`）
- [ ] `pnpm vet:go`
- [ ] 最小 API 适配

## 阶段 3 · 收敛

- [ ] 全量验收命令
- [ ] 写 `HANDOFF.md`：对照表、例外、预存失败、建议 merge 顺序
- [ ] `git status` 确认无无关脏文件
- [ ] `worker_done`（**不要 merge**）

## 验收命令

```bash
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/web test
pnpm test:go
pnpm vet:go
```

## 回滚

- 整分支 revert；或按 lockfile 回退单个 major 例外 pin
