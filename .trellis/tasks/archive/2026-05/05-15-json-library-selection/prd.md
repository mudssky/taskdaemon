# JSON 库选型与 Go 依赖升级

## Goal

补齐 taskdaemon 技术栈中 Go JSON 库选型的决策，并把 Go 后端基线从 `go 1.22.6` 升级到本机最新稳定工具链 `go 1.26.3`，同步评估后端与前端依赖升级范围。目标是在不牺牲跨平台单二进制和稳定性的前提下减少旧依赖约束。

## What I already know

* 用户指出此前技术栈讨论没有覆盖 JSON 库，现在需要补充讨论。
* 用户选择 JSON 方案 1：标准库优先。
* 用户确认本机 Go 已升级到最新 `go1.26.3`，并希望项目依赖也升级。
* Go 后端位于 `services/taskdaemon-go`；本任务已将 `go.mod` 从 `go 1.22.6` 升级到 `go 1.26.3`，本机 `go version` 是 `go1.26.3 windows/amd64`。
* 当前后端使用 Gin、Ent、Wails、Koanf；业务代码未直接选择第三方 JSON 库。
* `go.mod` 中已有 `sonic`、`goccy/go-json`、`json-iterator/go` 间接依赖，但不是项目显式标准。
* Ent 生成代码和现有测试仍使用 `encoding/json`。
* 核心后端依赖已升级到 Ent `v0.14.6`、Gin `v1.12.0`、Wails `v3.0.0-alpha.91`、modernc SQLite `v1.50.1`、testcontainers `v0.42.0`、Koanf `v2.3.4`。
* 前端/工作区依赖已升级到 Vite 8、TypeScript 6、React plugin 6、lucide-react 1.x、lint-staged 17。

## Assumptions (temporary)

* 第一版管理 API 的 JSON 吞吐不是主要瓶颈。
* 项目更看重跨平台稳定、低依赖复杂度和行为可预测。
* Go 基线升级后，可以移除此前因 Go 1.22.6 保守 pin 住的一部分依赖限制。
* 用户选择依赖升级范围 2：Go 后端与前端 workspace 一起升级。
* 依赖升级应分层推进和验证：Go directive + direct Go deps、Wails/SQLite/Ent 生成代码、前端 major 各自形成可回退提交，但同属本任务范围。

## Open Questions

* 无。

## Requirements (evolving)

* Go 后端业务代码默认使用 `encoding/json`。
* 明确 HTTP API、Ent JSON 字段、测试代码、未来 API client 生成链路的边界。
* 不新增直接第三方 JSON 依赖；只有 profiling 证明 JSON 是瓶颈时，才在 HTTP hot path 局部替换。
* 将 Go module 基线更新到 `go 1.26.3`。
* 升级项目依赖并通过现有质量门禁。
* 记录升级策略：直接依赖优先，major/alpha/Wails/SQLite/Ent 这类高风险依赖需要单独验证或分批提交。
* 前端 workspace 依赖升级纳入本任务范围，包括 Vite、TypeScript、React plugin、lint-staged、lucide-react 等可升级项。
* Wails v3 alpha 也纳入本任务范围，允许从 `v3.0.0-alpha.9` 升级到当前最新 alpha，并修复必要 API/构建差异。

## Acceptance Criteria (evolving)

* [x] PRD 记录 JSON 库候选、项目约束和推荐方案。
* [x] 后端 spec 或父 PRD 记录最终选型。
* [x] `services/taskdaemon-go/go.mod` 的 Go directive 更新到 `1.26.3`。
* [x] 后端 direct dependencies 升级到与 Go 1.26.3 兼容的合理版本。
* [x] 如升级 Ent 或 schema 相关依赖，运行并提交必要的 `go generate ./internal/data/ent` 结果。
* [x] `go mod tidy` 后 `go.mod` / `go.sum` 干净一致。
* [x] `pnpm test:go`、`pnpm vet:go` 通过。
* [x] 若包含前端依赖升级，`pnpm install`、`pnpm typecheck`、`pnpm lint` 通过。
* [x] 若 Wails 升级，`go test ./...` 和至少 `go build ./cmd/taskdaemon` 通过；Desktop runtime 若无法自动验证，需要在任务说明中记录残余风险。

## Definition of Done (team quality bar)

* Tests added/updated (unit/integration where appropriate)
* Lint / typecheck / CI green
* Docs/notes updated if behavior changes
* Rollout/rollback considered if risky

## Out of Scope (explicit)

* 不在没有 profiling 证据时做全项目 JSON 性能替换。
* 不修改 Gin、Ent 或 Wails 的内部 JSON 实现。
* 不引入多个 JSON 库分别用于不同业务层。
* 不在同一小步里强行修复依赖升级暴露出的无关业务问题；若发现大面积破坏，拆后续任务。

## Research References

* [`research/go-json-library-options.md`](research/go-json-library-options.md) — 标准库、goccy/go-json、json-iterator/go、sonic 的项目约束对比。

## Research Notes

### Feasible approaches here

**Approach A: 标准库优先** (Recommended)

* How it works: 业务代码、测试、配置、Ent 边界默认使用 `encoding/json`；不新增直接 JSON 依赖。
* Pros: 零新增依赖、跨平台稳定、行为可预测、Go 版本兼容风险最低。
* Cons: 极端吞吐场景不是最快。

**Approach B: HTTP 层保留可替换空间**

* How it works: 业务层仍用标准库；HTTP 层不主动绑定第三方库，但未来可基于 profiling 增加 adapter。
* Pros: 保持当前简单性，同时保留性能优化出口。
* Cons: 需要未来补压测和边界测试。

**Approach C: 直接采用高性能库**

* How it works: 选 `goccy/go-json`、`json-iterator/go` 或 `sonic` 作为项目标准。
* Pros: 提前获得性能余量。
* Cons: 增加兼容性、跨平台和维护风险；当前缺少瓶颈证据。

## Decision (ADR-lite)

### JSON 库标准库优先

**Context**: 后端 API 当前不是高吞吐 JSON 服务，项目目标更偏跨平台、低占用和单二进制稳定发布。Gin 间接依赖里已有多个高性能 JSON 库，但业务代码没有必要绑定其中一个。

**Decision**: Go 后端业务代码、配置、测试和 Ent JSON 边界默认使用 `encoding/json`。不新增直接第三方 JSON 依赖；未来只有 profiling 证明 JSON 是瓶颈时，才在 HTTP hot path 局部引入替换。

**Consequences**: 短期性能不是理论最高，但依赖面更小、行为更稳定，降低 Go 版本和跨平台构建风险。

## Upgrade Notes

### Go 依赖升级候选

* Go directive: `1.22.6` -> `1.26.3`。
* Ent: `v0.13.1` -> `v0.14.6`，需要运行 codegen 并检查生成代码 diff。
* Gin: `v1.10.1` -> `v1.12.0`，需要确认 binding/JSON 行为和测试。
* Wails: `v3.0.0-alpha.9` -> `v3.0.0-alpha.91`，alpha 跨度大，建议单独验证 Desktop build/runtime。
* modernc SQLite: `v1.34.5` -> `v1.50.1`，此前因 Go 基线受限，现在可评估升级，但需要 migration 和 SQLite 测试。
* testcontainers: `v0.35.0` -> `v0.42.0`，主要影响 integration tests。
* Koanf: `v2.1.2` -> `v2.3.4`，此前受 Go 版本限制，现在可评估升级。

### 前端依赖升级候选

* Vite 7 -> 8、TypeScript 5 -> 6、`@vitejs/plugin-react` 5 -> 6、`lucide-react` 0.x -> 1.x、`lint-staged` 16 -> 17。
* 这些都是 major 或显著版本跃迁；用户已选择本任务内一起升级，因此实现时需要拆分提交与验证，降低回退成本。

### Wails 升级决定

**Context**: 当前 Wails 为 `v3.0.0-alpha.9`，最新可用版本跨度较大。Wails 位于 Desktop runtime 和单二进制发布边界，升级风险高于普通库。

**Decision**: 用户选择本任务内一起升级 Wails 到最新 alpha，并接受必要的 API/构建适配。

**Consequences**: 本任务需要额外验证 `go test ./...`、`go build ./cmd/taskdaemon`，如果 Desktop 运行时无法自动验证，需要记录残余风险。

## Implementation Notes

### 后端依赖升级结果

* `services/taskdaemon-go/go.mod` 已更新到 `go 1.26.3`。
* Direct dependency 已升级：Ent `v0.14.6`、Gin `v1.12.0`、Koanf `v2.3.4`、Koanf confmap provider `v1.0.0`、testcontainers `v0.42.0`、Wails `v3.0.0-alpha.91`、`golang.org/x/crypto v0.51.0`、modernc SQLite `v1.50.1`。
* 已运行 `go mod tidy`。
* 已运行 `go generate ./internal/data/ent`，Ent 生成代码随升级更新。
* Wails v3 alpha 新版窗口创建 API 已从 `desktopApp.NewWebviewWindowWithOptions(...)` 适配为 `desktopApp.Window.NewWithOptions(...)`。

### 前端依赖升级结果

* `apps/web/package.json` 已升级到 `@vitejs/plugin-react ^6.0.2`、Vite `^8.0.13`、TypeScript `^6.0.3`、React/React DOM `^19.2.6`、lucide-react `^1.16.0`、dayjs `^1.11.20`。
* 根 workspace `lint-staged` 已升级到 `^17.0.4`。
* TypeScript 6 对 CSS side-effect import 更严格，已新增 `apps/web/src/vite-env.d.ts` 保留 Vite 客户端类型声明。
* 已运行 `pnpm build:web` 并通过 `pnpm sync:web-assets` 同步发布期 embed assets。

### 验证记录

* `go test ./...`：通过。
* `go build -trimpath -o ../../build/bin/taskdaemon ./cmd/taskdaemon`：通过。
* `pnpm typecheck`：通过。
* `pnpm lint`：通过。
* `pnpm build:web`：通过。
* `pnpm test:go`：通过。
* `pnpm vet:go`：通过。
* `pnpm outdated -r`：无输出。

### 残余风险

* Wails Desktop runtime 尚未在交互窗口中手工启动验证；当前已覆盖 Go 编译与后端测试，后续发布前建议执行一次 `pnpm desktop` 做人工 smoke test。
* `pnpm update -r --latest` 后 pnpm 提示 esbuild build script 被忽略；当前 `pnpm build:web` 已成功，说明本机现有依赖可用。若在全新机器安装遇到 esbuild 原生包问题，再按 pnpm 10 的 build approval 流程处理。

## Technical Notes

* 已检查：`services/taskdaemon-go/go.mod`
* 已检查：`go version`
* 已运行：`go list -m -u all`
* 已运行：`pnpm outdated -r`
* 已检查：`services/taskdaemon-go/internal/httpapi`
* 已检查：`services/taskdaemon-go/internal/data/ent`
* 已检查：`apps/web/package.json`
* 已检查：`.trellis/tasks/05-14-cross-platform-scheduler-daemon/prd.md`
* 已运行：`go mod tidy`
* 已运行：`go generate ./internal/data/ent`
* 已运行：`go test ./...`
* 已运行：`go build -trimpath -o ../../build/bin/taskdaemon ./cmd/taskdaemon`
* 已运行：`pnpm update -r --latest`
* 已运行：`pnpm typecheck`
* 已运行：`pnpm lint`
* 已运行：`pnpm build:web`
* 已运行：`pnpm sync:web-assets`
* 已运行：`pnpm test:go`
* 已运行：`pnpm vet:go`
