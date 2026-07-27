# HANDOFF — 07-27-deps-latest-upgrade

**分支**: `mudssky/deps-latest-upgrade`  
**Worker**: deps-latest-upgrade (supervised)  
**完成时间**: 2026-07-27  
**状态**: 待协调者验收 / **不要 merge by worker**

## 摘要

1. JS/TS 直接依赖已 `pnpm up -r --latest` 升到当前 registry latest；含 TypeScript 7.0.2、Biome 2.5.5、jest-dom 7、Vite 8.1.5 等。
2. Go 直接依赖已 `go get …@latest` + `go mod tidy`；Wails `v3.0.0-alpha.91` → `v3.0.0-alpha2.118`，sqlite/crypto/gocron/koanf/testcontainers 同步升。
3. 验收：`pnpm typecheck` / `lint` / web test / `vet:go` 全绿；`pnpm test:go` 仅 `internal/config` 两条路径断言失败（**预存**，与依赖无关）。

## 验收结果

| 命令 | 结果 |
|---|---|
| `pnpm typecheck` | PASS |
| `pnpm lint` | PASS（已 `biome migrate` 到 2.5.5 schema） |
| `pnpm --filter @taskdaemon/web test` | PASS（6 files / 17 tests） |
| `pnpm vet:go` / `go vet ./...` | PASS |
| `go build ./...` | PASS |
| `pnpm test:go` / `go test ./...` | **FAIL 仅** `taskdaemon/internal/config` 2 例（见下） |
| `pnpm generate:go` | **未跑**（ent 仍为 `v0.14.6`，无生成链变更） |

### 预存失败（非本任务引入）

`internal/config`：

- `TestProjectPathFindsFirstProjectConfig`
- `TestProjectLocalPathFindsFirstLocalConfig`

现象：macOS 上 `t.TempDir()` 经 `Chdir` 后 `ProjectPath()` 返回 `/private/var/folders/...`，断言期望 `TempDir` 字面路径（常为 `/var/folders/...` 符号链接另一面）。  
证据：本任务 **未改** 任何 `*.go` 源码，仅 `go.mod` / `go.sum`；测试文件来自既有提交 `44ba6df`。  
建议：另开 fix 任务对路径做 `filepath.EvalSymlinks` 后再比，或与 `os.Getwd()` 对齐。

## 改动文件（本任务权威）

| 路径 | 说明 |
|---|---|
| `package.json` | 根 devDependencies latest |
| `apps/web/package.json` | web deps latest |
| `apps/web/biome.json` | schema/preset 迁移到 Biome 2.5.5 |
| `pnpm-lock.yaml` | 重生成 |
| `services/taskdaemon-go/go.mod` | 直接依赖 latest |
| `services/taskdaemon-go/go.sum` | tidy 后 |
| `.trellis/tasks/07-27-deps-latest-upgrade/*` | 任务文档 + 本 HANDOFF |

**未改**：`internal/notify/**`、desktop 业务源码、Ent 生成物、Node/pnpm major（`packageManager` 仍 `pnpm@11.1.2`）。

## JS/TS 版本对照

| 包 | 升级前 | 升级后 | 备注 |
|---|---|---|---|
| `@biomejs/biome` | 2.4.15 | **2.5.5** | 已 migrate config |
| `typescript` | ^6.0.3 | **^7.0.2** | major；typecheck 绿 |
| `@testing-library/jest-dom` | ^6.9.1 | **^7.0.0** | major；测试绿 |
| `vitest` | ^4.1.6 | ^4.1.10 | |
| `lint-staged` | ^17.0.4 | ^17.2.0 | |
| `@hookform/resolvers` | ^5.2.2 | ^5.4.3 | |
| `@radix-ui/react-alert-dialog` | ^1.1.15 | ^1.1.23 | |
| `@radix-ui/react-checkbox` | ^1.3.3 | ^1.3.11 | |
| `@radix-ui/react-label` | ^2.1.8 | ^2.1.15 | |
| `@radix-ui/react-select` | ^2.2.6 | ^2.3.7 | |
| `@radix-ui/react-slot` | ^1.2.4 | ^1.3.3 | |
| `@tailwindcss/vite` | ^4.3.0 | ^4.3.3 | |
| `tailwindcss` | ^4.3.0 | ^4.3.3 | |
| `@tanstack/react-query` | ^5.100.10 | ^5.101.4 | |
| `@tanstack/react-router` | ^1.169.2 | ^1.170.18 | |
| `@vitejs/plugin-react` | ^6.0.2 | ^6.0.4 | |
| `@types/react` | ^19.2.14 | ^19.2.17 | |
| `dayjs` | ^1.11.20 | ^1.11.21 | |
| `lucide-react` | ^1.16.0 | ^1.26.0 | |
| `react` / `react-dom` | ^19.2.6 | ^19.2.8 | |
| `react-hook-form` | ^7.75.0 | ^7.83.0 | |
| `vite` | ^8.0.13 | ^8.1.5 | |
| `zod` | ^4.4.3 | ^4.4.3 | 已是 latest |
| `husky` / `jsdom` / `@testing-library/react` 等 | — | 已是 latest 或无更新 | |

### JS 例外（未升到 latest 的 pin）

| 项 | 原因 |
|---|---|
| `packageManager: pnpm@11.1.2` | PRD：不擅自升 Node/pnpm major |
| （其余直接依赖） | `pnpm outdated -r` 升级后应为空或仅锁内解析差异 |

## Go 直接依赖对照

| 模块 | 升级前 | 升级后 | 备注 |
|---|---|---|---|
| `entgo.io/ent` | v0.14.6 | v0.14.6 | 已是 latest；**未** generate |
| `github.com/ebitengine/oto/v3` | v3.4.0 | v3.4.0 | latest |
| `github.com/gin-gonic/gin` | v1.12.0 | v1.12.0 | latest |
| `github.com/go-co-op/gocron/v2` | v2.21.2 | **v2.22.0** | |
| `github.com/google/uuid` | v1.6.0 | v1.6.0 | latest |
| `github.com/jonboulle/clockwork` | v0.5.0 | v0.5.0 | latest |
| `github.com/knadh/koanf/providers/confmap` | v1.0.0 | v1.0.0 | latest |
| `github.com/knadh/koanf/v2` | v2.3.4 | **v2.3.5** | |
| `github.com/lib/pq` | v1.12.3 | v1.12.3 | latest |
| `github.com/spf13/cobra` | v1.10.2 | v1.10.2 | latest |
| `github.com/stretchr/testify` | v1.11.1 | v1.11.1 | latest |
| `github.com/testcontainers/testcontainers-go` | v0.42.0 | **v0.43.0** | |
| `…/modules/postgres` | v0.42.0 | **v0.43.0** | |
| `github.com/wailsapp/wails/v3` | v3.0.0-alpha.91 | **v3.0.0-alpha2.118** | 最新 alpha；build/desktop 测试绿 |
| `golang.org/x/crypto` | v0.51.0 | **v0.54.0** | |
| `gopkg.in/natefinch/lumberjack.v2` | v2.2.1 | v2.2.1 | latest |
| `gopkg.in/yaml.v3` | v3.0.1 | v3.0.1 | latest |
| `modernc.org/sqlite` | v1.50.1 | **v1.54.0** | |

### Go 例外 / 间接说明

| 项 | 说明 |
|---|---|
| `ariga.io/atlas`（indirect） | 仍跟 ent 解析；`go list -m -u` 可见 major `v1.x` 候选，**未**强制升 major（避免生成链无谓动荡） |
| 源码 API 适配 | **无**；Wails alpha2 与现有 desktop 代码编译通过 |

## 与 T2a 并行说明

- 本 worktree 基于 `dev`，**无** T2a notify/schema 业务代码。
- 若合并时 T2a 也改了 `go.mod`/`go.sum`/Ent 生成物：以本分支 lock 为 JS 权威；Go 侧协调者 rebase 后 `go mod tidy`，保留双方 require，再视需要 `pnpm generate:go`。
- 建议 merge 顺序：**功能分支先合** 或 **本 deps 先合均可**；撞 lockfile 时 **重跑 upgrade/tidy，不手工揉**。

## 建议协调者复跑

```bash
pnpm typecheck && pnpm lint && pnpm --filter @taskdaemon/web test && pnpm test:go && pnpm vet:go
```

预期：前三项 + vet 绿；`test:go` 仅 config 两例红（预存）。

## 回滚

- 整分支不 merge；或 `git revert` 本 chore commit。
- 单包 pin：改 `package.json` / `go get module@旧版` + 重装/tidy。
