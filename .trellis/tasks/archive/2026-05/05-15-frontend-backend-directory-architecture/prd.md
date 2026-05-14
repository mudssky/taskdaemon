# brainstorm: 前后端分离目录结构调整

## Goal

重新定义 taskdaemon 仓库的目录结构与包边界，使项目从“Go 单二进制为主轴，前端作为嵌入 UI”调整为更清晰的前后端分离架构：前端、Desktop 壳、共享前端包和后端服务在仓库中拥有独立边界，Go 只是其中一个后端实现/服务，而不是整个仓库结构的唯一中心。

## What I already know

* 用户指出：之前讨论过目录结构，但当前没有完全采用 pnpm monorepo。
* 用户现在倾向：考虑前后端分离架构，Go 只是其中一个后端，需要改一下目录结构与相关约定。
* 迁移前仓库是根 Go module：`go.mod` 位于仓库根目录，module 为 `taskdaemon`。
* 迁移前前端是 pnpm workspace 的一个 package：`web/app`，包名为 `@taskdaemon/web`。
* 迁移前 `pnpm-workspace.yaml` 包含 `web/app` 和 `web/packages/*`。
* 迁移前 Wails v3 配置 `frontend.dir = ./web/app`。
* 迁移前 Go embed 边界是 `web/embedded`，构建脚本把 `web/app/dist` 同步到 `web/embedded/dist`。
* 迁移前后端目录规范明确写着“根 Go module + 轻量前端 workspace”，这与新方向冲突。
* 父任务 PRD 仍写着“Go 核心 + 多入口单二进制”，但同时要求开发期前后端能单独启动。

## Assumptions (temporary)

* 用户已确认本任务直接执行全量目录迁移，而不是只更新文档或只迁移前端。
* 仍保留 Wails v3 Desktop 作为发布/桌面壳能力，但它不应把前端目录结构锁死为 Go 子目录。
* Go baseline 仍保持 `1.22.6`，Wails v3 仍使用当前可编译版本。
* pnpm workspace 覆盖 `apps/*`、`packages/*` 和带 `package.json` 的 `services/*`；Go module 作为 `services/*` 下的独立后端 module，依赖仍由 `go.mod` 管理。

## Open Questions

* 无阻塞开放问题；等待最终确认后进入实施。

## Requirements (evolving)

* 明确根目录是产品级 workspace，而不是 Go module 专属根。
* 明确前端 app、Desktop 壳、共享 TS packages、Go 后端服务的职责边界。
* 采用完整产品 workspace split：`apps/*`、`services/*`、`packages/*` 并列。
* Desktop/Wails 不单独拆成 `apps/desktop`；第一轮仍作为 `services/taskdaemon-go` 的 desktop 发布模式。
* `services/taskdaemon-go` 包含 service-local `package.json`，用于声明 Go service 的启动、测试、生成、构建等脚本门面。
* 根 `package.json` 保留为 workspace 聚合入口，统一调度前端 app、Go service 和后续共享 packages 的脚本。
* 更新 Trellis spec/PRD，避免后续 UI/runner 任务继续沿用旧的 `web/app` + 根 Go module 假设。
* 如果执行迁移，需要同步 `pnpm-workspace.yaml`、根 `package.json` scripts、`wails.json`、Go embed 路径、构建脚本、README、lint-staged 和相关 spec。
* 保持开发期前后端可单独启动。
* 保持发布期前端资源可嵌入 Desktop/Go 构建产物。
* 明确 Go 后端内部仍可保持 `cmd/taskdaemon`、`internal/*` 的 Go 项目惯例，但这些目录应位于 Go service module 内，而不是仓库根。
* 根 `package.json` scripts 需要改为跨目录调用，Go 相关命令需要进入 `services/taskdaemon-go` 执行。
* `pnpm-workspace.yaml` 需要覆盖 `apps/*`、`packages/*`、`services/*`，使带 `package.json` 的 Go service 可被 `pnpm --filter` 调度。
* `go test ./...` 的项目级等价命令需要明确，避免用户在根目录误跑失败。

## Acceptance Criteria (evolving)

* [x] 新目录结构有明确树形图和命名约定。
* [x] Go 后端作为 `services/*` 或等价目录下的服务边界被定义清楚。
* [x] pnpm workspace 覆盖 `apps/*`、`packages/*`、`services/*`，但只作为脚本/workspace 调度层，不接管 Go 依赖。
* [x] Wails/Desktop 与 Web app 的关系被定义清楚。
* [x] `services/taskdaemon-go/package.json` 提供 service-local scripts。
* [x] 需要修改的文件清单和迁移步骤明确。
* [x] 与现有子任务（基础 UI、scheduler/runner）的影响范围明确。
* [x] 代码迁移后 `services/taskdaemon-go` 内 `go test ./...` 通过。
* [x] 根目录提供可用的 `pnpm` scripts 执行前端 build/typecheck/lint 和 Go 测试。
* [x] `pnpm build:web` 输出到 `apps/web/dist`。
* [x] `pnpm sync:web-assets` 把 `apps/web/dist` 同步到 `services/taskdaemon-go/web/embedded/dist`。
* [x] `services/taskdaemon-go/wails.json` 使用 Wails v3 嵌套配置，并指向 `../../apps/web`。
* [x] README 与 Trellis directory spec 不再描述根 Go module + `web/app`。
* [x] 用户确认 MVP 范围后再进入实施。

## Definition of Done (team quality bar)

* Tests added/updated where behavior or import paths change.
* Lint / typecheck / `go test ./...` green after implementation tasks.
* Docs/spec updated to prevent future agents following stale architecture.
* Migration/rollback considered if file moves are executed.

## Out of Scope (explicit)

* 不在本 brainstorm 阶段升级 Go baseline。
* 不在本 brainstorm 阶段升级 Wails v3 版本。
* 不把 pnpm workspace 扩展成管理 Go module 的工具。
* 不拆独立 `apps/desktop`。

## Research References

* [`research/pnpm-workspace-boundary.md`](research/pnpm-workspace-boundary.md) — pnpm workspace 管理 JS/TS package glob；不要求整个仓库是纯前端 monorepo，Go 可以作为独立 service module 并列存在。

## Research Notes

### Constraints from current repo

* `cmd/`、`internal/`、`go.mod` 目前位于根目录。
* 迁移前 `web/app` 是唯一前端应用，`web/packages/*` 预留但未使用。
* 迁移前 `web/embedded` 与 Go embed 强绑定；迁移时要决定 embed 仍属于 Go 服务，还是 Desktop app。
* `README.md`、`.trellis/spec/backend/directory-structure.md`、`.trellis/spec/frontend/directory-structure.md` 都包含旧路径。
* 迁移前 `package.json` 的 `lint-staged` 匹配 `web/app/**/*`。
* 迁移前 `wails.json` 的 `frontend.dir` 指向 `./web/app`。
* 迁移前 `scripts/sync-web-assets.ps1` 从 `web/app/dist` 同步到 `web/embedded/dist`。
* Go 代码通过 module 内的 `taskdaemon/web/embedded` 导入前端嵌入资产；迁移后该 import path 保持不变，文件实际位于 `services/taskdaemon-go/web/embedded`。

### Expansion Sweep

1. Future evolution
   * 后续可能增加非 Go 后端服务、SDK、OpenAPI/类型生成包或独立 worker；根目录应能容纳 `services/*`、`packages/*`、`apps/*`。
   * Desktop 可能从“Go 后端内的 Wails 模式”演进为更独立的桌面 app 包，但第一轮仍可复用 Go/Wails 发布路径。

2. Related scenarios
   * 基础管理 UI 子任务应写入 `apps/web` 或确认后的前端 app 目录，不应继续默认 `web/app`。
   * scheduler/runner/data/auth 等 Go 子任务应引用 Go service 内路径，避免后续大规模二次改 import。

3. Failure / edge cases
   * 直接移动 Go module 会影响 `go test ./...` 的执行位置、import path、Wails embed import、README 命令和可能的 IDE 配置。
   * 如果只改文档不迁移代码，短期会出现“spec 新、代码旧”的双轨状态，后续任务需要非常明确地按目标路径实施。

## Implementation Plan

1. 迁移目录
   * `web/app` -> `apps/web`
   * `cmd`、`internal`、`go.mod`、`go.sum`、`wails.json` -> `services/taskdaemon-go/`
   * `web/embedded` -> `services/taskdaemon-go/web/embedded`

2. 更新 workspace 与脚本
   * `pnpm-workspace.yaml`: `apps/*`、`packages/*`、`services/*`
   * 根 `package.json`: 更新 `dev:web`、`build:web`、`typecheck`、`lint`、`sync:web-assets`、Go 测试相关聚合脚本
   * `services/taskdaemon-go/package.json`: 新增 Go service local scripts
   * `scripts/sync-web-assets.ps1`: 默认源改为 `apps/web/dist`，目标改为 `services/taskdaemon-go/web/embedded/dist`
   * `.gitignore`: 更新 Vite dist/cache 与 embedded dist 例外

3. 更新 Go/Wails 引用
   * `services/taskdaemon-go/wails.json`: `frontend.dir = ../../apps/web`
   * Go import 中的 `taskdaemon/web/embedded` 改为迁移后的模块内路径
   * 检查 `go:embed all:dist` 目标仍可在干净 checkout 下存在

4. 更新文档与 Trellis spec
   * README 开发命令和项目结构
   * `.trellis/spec/backend/directory-structure.md`
   * `.trellis/spec/frontend/directory-structure.md`
   * 受影响的父/子任务 PRD 路径引用

5. 验证
   * `pnpm install`
   * `pnpm typecheck`
   * `pnpm lint`
   * `pnpm build:web`
   * `pnpm sync:web-assets`
   * 在 `services/taskdaemon-go` 下运行 `go test ./...`

### Feasible approaches here

**Approach A: Full product workspace split** (Recommended)

* How it works: 根目录只放 workspace/文档/脚本；前端迁到 `apps/web`；Desktop 壳可独立为 `apps/desktop` 或留在 Go backend 的 desktop entry；Go 后端迁到 `services/taskdaemon-go`；共享 TS 包放 `packages/*`。
* Pros: 最符合“前后端分离，Go 只是其中一个后端”；后续增加 Node/Python backend 或 SDK 时结构自然。
* Cons: 当前 Go module 移动成本高，需要同步 import、脚本、Wails、embed、CI 和 Trellis spec。
* User decision: 已选择。
* Follow-up decision: `services/taskdaemon-go` 内添加 `package.json` 作为脚本门面；根 `package.json` 作为 workspace 聚合入口。

**Approach B: Frontend apps split, Go root temporarily retained**

* How it works: 前端从 `web/app` 迁到 `apps/web`，共享包改为 `packages/*`；Go 仍暂时保留根 module，但文档明确这是过渡期。
* Pros: 迁移成本较低，先修正前端 workspace 结构。
* Cons: 根目录仍被 Go module 占据，和“Go 只是其中一个后端”的长期模型不完全一致，未来还要迁一次。

**Approach C: Documentation-only architecture decision first**

* How it works: 先更新 PRD/spec，冻结目标结构和迁移计划；现有代码路径暂不移动。
* Pros: 风险最低，不影响当前已完成的数据/auth 基础。
* Cons: 短期内代码和文档不一致，后续任务容易继续往旧目录写代码。

## Decision (ADR-lite)

**Context**: 当前仓库结构把 Go module 放在根目录，把前端放在 `web/app`，文档也把项目主轴描述为 Go 单二进制。但用户确认长期架构应是前后端分离，Go 只是其中一个后端服务。

**Decision**: 采用完整产品 workspace split。目标结构为：

```text
.
├── apps/
│   └── web/                 # Vite + React 管理台
├── services/
│   └── taskdaemon-go/       # Go 后端服务与 Wails 发布模式
│       ├── cmd/taskdaemon/
│       ├── internal/
│       ├── web/embedded/
│       ├── go.mod
│       ├── package.json      # service-local scripts
│       └── wails.json
├── packages/                # 前端共享包、API client、UI primitives 等
├── scripts/
├── docs/
├── package.json
└── pnpm-workspace.yaml
```

**Consequences**: 根目录成为产品 workspace；pnpm workspace 覆盖 `apps/*`、`packages/*` 与带 `package.json` 的 `services/*`；Go module 下沉到 `services/taskdaemon-go`。pnpm 负责脚本编排，不负责 Go module 依赖管理。迁移实施需要同步 Go 命令执行位置、embed import、Wails `frontend.dir`、构建脚本、README 和 Trellis spec。

### Desktop/Wails 边界

**Context**: Wails v3 典型项目仍以 Go app 为运行时根，当前 Desktop 模式需要复用 Go `internal/*`、配置、HTTP/API、认证、调度、runner 和 embed 资产。

**Decision**: Desktop/Wails 留在 `services/taskdaemon-go` 内，不创建独立 `apps/desktop`。`apps/web` 是独立前端管理台；`services/taskdaemon-go/wails.json` 通过 `frontend.dir = ../../apps/web` 指向前端 dev/build 目录。

**Consequences**: Desktop 发布路径与 Go 后端服务保持一致，避免 `apps/desktop` 反向依赖 Go `internal` 的边界问题。未来如果 Desktop 需要独立生命周期，再单独设计桌面 app 与 Go backend 的通信协议。

### Script Boundary

**Context**: 用户希望 `services/taskdaemon-go` 下也能有 `package.json`，用于启动脚本、测试等，同时根目录也保留项目级脚本入口。

**Decision**: 采用“双层脚本”：

* 根 `package.json`：workspace 聚合脚本，使用 `pnpm --filter` 调度 app/service/package。
* `apps/web/package.json`：前端 app 本地脚本，负责 Vite、typecheck、lint、build。
* `services/taskdaemon-go/package.json`：Go service 本地脚本，负责 `go run`、`go test`、`go generate`、`go vet` 和 Wails/desktop 相关命令。

Root scripts 初始目标：

```json
{
  "dev:web": "pnpm --filter @taskdaemon/web dev",
  "build:web": "pnpm --filter @taskdaemon/web build",
  "build:backend": "pnpm --filter @taskdaemon/service-go build",
  "dev:backend": "pnpm --filter @taskdaemon/service-go dev",
  "desktop": "pnpm --filter @taskdaemon/service-go desktop",
  "migrate": "pnpm --filter @taskdaemon/service-go migrate",
  "test:go": "pnpm --filter @taskdaemon/service-go test",
  "test:go:integration": "pnpm --filter @taskdaemon/service-go test:integration",
  "generate:go": "pnpm --filter @taskdaemon/service-go generate",
  "vet:go": "pnpm --filter @taskdaemon/service-go vet",
  "typecheck": "pnpm --filter @taskdaemon/web typecheck",
  "lint": "pnpm --filter @taskdaemon/web lint",
  "test": "pnpm typecheck && pnpm lint && pnpm test:go",
  "sync:web-assets": "powershell -NoProfile -ExecutionPolicy Bypass -File scripts/sync-web-assets.ps1"
}
```

`services/taskdaemon-go/package.json` 初始目标：

```json
{
  "name": "@taskdaemon/service-go",
  "private": true,
  "scripts": {
    "dev": "go run ./cmd/taskdaemon serve",
    "desktop": "go run ./cmd/taskdaemon desktop",
    "migrate": "go run ./cmd/taskdaemon db migrate",
    "build": "go build -trimpath -o ../../build/bin/taskdaemon ./cmd/taskdaemon",
    "test": "go test ./...",
    "test:integration": "go test -tags=integration ./internal/data",
    "generate": "go generate ./internal/data/ent",
    "vet": "go vet ./..."
  }
}
```

## Technical Notes

* Task directory: `.trellis/tasks/05-15-frontend-backend-directory-architecture`
* Parent task: `.trellis/tasks/05-14-cross-platform-scheduler-daemon`
* 已读取：`.trellis/spec/backend/directory-structure.md`
* 已读取：`.trellis/spec/frontend/directory-structure.md`
* 已读取：`.trellis/tasks/05-14-cross-platform-scheduler-daemon/prd.md`
* 已读取：`.trellis/tasks/05-14-basic-management-ui/prd.md`
* 已读取：`.trellis/tasks/05-14-scheduler-runner-core/prd.md`
* 已读取：`package.json`
* 已读取：`pnpm-workspace.yaml`
* 已读取：`web/app/package.json`
* 已读取：`wails.json`

## Verification

* `pnpm install`
* `pnpm typecheck`
* `pnpm lint`
* `pnpm generate:go`
* `pnpm build:web`
* `pnpm build:backend`
* `pnpm sync:web-assets`
* `pnpm vet:go`
* `pnpm test`
