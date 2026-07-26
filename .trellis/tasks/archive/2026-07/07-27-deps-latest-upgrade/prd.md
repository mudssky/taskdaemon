# 依赖全量升级到最新（pnpm + Go modules）

> **轨**：Chore（跨轨基础设施）｜ **依赖**：与 T2a/T6 的 Ent 生成、功能分支 lockfile 变更错开  
> **性质**：专门任务。独占所有 lockfile / 清单文件，修复合规与测试破口。  
> **目标分支**：`feat/deps-latest-upgrade`（从 `dev` 切出合回 `dev`）

## Goal

把 monorepo 内 **JS/TS（pnpm）** 与 **Go modules** 的直接依赖升级到**当前可解析的最新稳定版本**（在工具链约束内），并保证 §11 验收基线全绿（或对预存失败有明确归因与 ticket）。

## Context

- 协调者请求（2026-07-27）：「依赖可以升级一下，专门派任务，最好都升级到最新」
- 当前工具链锚点：`pnpm@11.1.2`、`typescript ^6`、`go 1.26.3`、Biome / Vitest / Vite 8 / React 19 / Wails v3 alpha
- `pnpm outdated -r` 显示多包 wanted 落后于 latest（如 biome 2.4→2.5、router/query 小版本、radix 补丁等）
- Go 直接依赖中 atlas / 若干 indirect 有 major 候选；**直接 require 以 `go list -m -u` 对 `go.mod` require 块为准**
- 并行安全带：本任务独占 lockfile；**不得与其他任务同时改** `pnpm-lock.yaml` / `go.sum`

## Requirements

### R1 范围

- 根 `package.json`、`apps/*/package.json`、`services/*/package.json`、未来 `packages/*`（若已存在）
- `services/taskdaemon-go/go.mod` + `go.sum` 的**直接依赖**优先升到最新兼容版
- lockfile 必须一并提交：`pnpm-lock.yaml`、`go.sum`
- **不**擅自升 Node/pnpm major 除非现有脚本已证明需要且写明理由
- **不**引入与业务无关的新依赖

### R2 升级策略

- 默认：**全部直接依赖 → latest**（semver 允许范围内用 `pnpm up -r --latest` / `go get -u` 类流程）
- 若 latest 为 **breaking major** 且修复成本 > 半日：在 HANDOFF 记录「停在 last-compatible + 原因」，不得静默跳过不写
- Wails v3 仍为 alpha：只升到最新 alpha/可编译标签，不做 API 大重构（大重构另开任务）
- Ent / atlas 等代码生成链：升级后必须重跑 `pnpm generate:go`，**禁止手工改生成物**

### R3 破口修复

- typecheck / lint / 单测因升级失败 → **在本任务修**（适配 API、类型、配置）
- 预存失败（如已知 `internal/config` 路径断言）→ 若本任务未触及该包可标注预存；若升级触发则必须修或明确回滚该依赖
- 禁止用 skip/删除测试「假绿」

### R4 文档与清单

- HANDOFF 含：升级前后版本对照表、breaking 项、未升到 latest 的例外与原因
- 若影响下游契约（极少）：通知协调者；默认不改业务契约

### R5 并行与 Git

- 分支：`feat/deps-latest-upgrade`
- 每日 rebase `dev`
- 若 T2a/其他分支也改了 lockfile：以本任务为 lockfile 权威，合并时重跑 upgrade/generate，不手工揉 lock
- **不要 merge**；完成后 `worker_done` 等协调者验收

## Acceptance Criteria

- [ ] JS/TS 直接依赖已尽可能升到 latest；例外表完整
- [ ] Go 直接依赖已尽可能升到最新兼容版；例外表完整
- [ ] `pnpm-lock.yaml` 与 `go.sum` 与清单一致并已提交到功能分支
- [ ] 升级后 `pnpm install` 干净可复现
- [ ] `pnpm typecheck` 绿
- [ ] `pnpm lint` 绿
- [ ] `pnpm --filter @taskdaemon/web test` 绿
- [ ] `pnpm test:go` 绿或仅含**已文档化**的预存失败
- [ ] `pnpm vet:go` 绿
- [ ] 若动 Ent/atlas：`pnpm generate:go` 后生成物一致
- [ ] `HANDOFF.md` 含版本对照与例外
- [ ] 未改无关业务功能（diff 以依赖与必要适配为限）

## Out of Scope

- 新功能、重构业务模块
- 升级操作系统 / 全局 Go toolchain 安装方式（可建议，不强制改 CI 镜像除非已有文件）
- Track G 的 `packages/` 新包（G1 尚未落地则跳过）
- 依赖安全审计平台化（可顺带记 `pnpm audit` 高危，不阻塞）

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务产出 | 最新 lockfile + 适配补丁 + HANDOFF |
| 本任务消费 | 无功能契约 |
| 冲突方 | 任何改 `package.json` / `go.mod` / lock 的任务；**T2a Ent 生成期间优先等 T2a 合入或错开 go.mod** |

## Definition of Done

- AC 全勾；协调者复跑验收命令通过；分支待 merge

## Notes

- 「都升到最新」是默认目标，不是无脑 major 硬上；**可编译 + 测试绿**优先于版本号虚荣
- 协调者调度：尽量在功能任务不碰 lockfile 的窗口执行
