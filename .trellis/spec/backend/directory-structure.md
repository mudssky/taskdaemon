# Directory Structure

> How backend code is organized in this project.

---

## Overview

taskdaemon 采用 pnpm monorepo + 多后端服务边界。仓库根目录是产品级 workspace，不再是 Go module 根。Go 后端只是 `services/*` 下的一个服务：`services/taskdaemon-go`。它负责 HTTP API、CLI、daemon、调度、runner、数据层和 Wails Desktop 发布模式；前端管理台放在 `apps/web`，发布期构建产物同步到 Go service 的 embed 边界。

---

## Directory Layout

```text
.
├── apps/
│   └── web/                         # Vite + React 管理台，Web/Desktop 共用 UI
├── packages/                        # 前端共享包、API client、UI primitives 等
├── services/
│   └── taskdaemon-go/
│       ├── cmd/
│       │   └── taskdaemon/          # Go service 入口：serve / desktop / CLI 子命令分发
│       ├── internal/
│       │   ├── app/                 # 应用装配、依赖注入、生命周期
│       │   ├── config/              # 配置加载、默认路径、flag/env 合并
│       │   ├── httpapi/             # Gin router、middleware、handler
│       │   ├── scheduler/           # cron 调度核心与 trigger 边界
│       │   ├── runner/              # shell/bash/pwsh/python/node/typescript runner
│       │   ├── data/                # Ent client、repository、migration 边界
│       │   ├── auth/                # 单管理员、session、CSRF/Host 安全边界
│       │   └── desktop/             # Wails 绑定、桌面生命周期与原生能力
│       ├── web/
│       │   └── embedded/            # Go embed 包，dist/ 由构建脚本同步
│       ├── go.mod
│       ├── package.json             # service-local scripts，不管理 Go 依赖
│       └── wails.json
├── docs/
├── scripts/
├── package.json                     # workspace 聚合脚本
├── pnpm-workspace.yaml
└── README.md
```

---

## Workspace Organization

* 根 `package.json` 只做 workspace 聚合脚本，通过 `pnpm --filter` 调度 `apps/*`、`packages/*` 和 `services/*`。
* `pnpm-workspace.yaml` 覆盖 `apps/*`、`packages/*`、`services/*`。对于 Go service，pnpm 只负责脚本编排，不接管 Go 依赖。
* `services/taskdaemon-go/package.json` 放 `dev`、`desktop`、`migrate`、`test`、`generate`、`vet` 等 service-local scripts。
* Go module 根是 `services/taskdaemon-go`；Go 命令在该目录执行，或通过根脚本 `pnpm test:go`、`pnpm vet:go` 等间接执行。
* `apps/web` 是 Vite + React 管理台；Desktop 和 Web 共用这套 UI。

---

## Go Module Organization

* `services/taskdaemon-go/cmd/taskdaemon` 只负责入口分发、命令行绑定和调用应用装配，不承载业务逻辑。
* `services/taskdaemon-go/internal/app` 负责组合配置、数据层、调度器、runner、HTTP API 和 Desktop 生命周期。
* `services/taskdaemon-go/internal/cli` 放 Cobra 命令树；`serve`、`desktop`、`db migrate` 等入口通过 hooks 调用 `internal/app` 或 `internal/desktop`。
* `services/taskdaemon-go/internal/config` 放默认配置、用户配置目录解析、文件/env/flag 覆盖和配置测试。覆盖顺序固定为 defaults < file < `TASKDAEMON_` env < overrides。
* `services/taskdaemon-go/internal/httpapi` 放 HTTP router、middleware、request/response DTO 和 handler；handler 调用 service 边界，不直接散落数据库方言。
* `services/taskdaemon-go/internal/scheduler` 放 cron 调度、任务注册、手动触发和 overlap 处理边界。
* `services/taskdaemon-go/internal/runner` 放内置结构化 runner、进程执行、timeout/cancel、输出截断。
* `services/taskdaemon-go/internal/data` 放 Ent client、repository、migration 和数据库方言隔离。
* `services/taskdaemon-go/internal/auth` 放单管理员账号、登录态、CSRF/Host 校验等认证安全能力。
* `services/taskdaemon-go/internal/desktop` 只放 Wails 绑定与桌面专属能力，不能复制 HTTP/API/调度业务逻辑；Desktop 资产来自 `taskdaemon/web/embedded` 包。
* `services/taskdaemon-go/web/embedded` 是发布期 Go embed 边界，构建脚本把 `apps/web/dist` 同步到 `services/taskdaemon-go/web/embedded/dist` 后再构建二进制；该 `dist` 需要提交，避免干净 checkout 缺少 `go:embed all:dist` 目标。

---

## Entry Contracts

* `taskdaemon serve` 启动 HTTP API，不初始化 Wails Desktop。
* `taskdaemon desktop` 启动 Wails v3 runtime，并加载 `services/taskdaemon-go/web/embedded` 中的同一套前端构建产物。
* `taskdaemon db migrate` 是 CLI-only 入口，不初始化 Desktop。
* `--config <path>` 是根级 persistent flag，所有子命令都应加载同一份配置。
* 默认配置路径为 `os.UserConfigDir()/taskdaemon/config.yaml`。
* `/api/health` 是后端探活 endpoint；Swagger route 默认关闭，只在 `server.swagger.enabled=true` 时注册。

---

## Dependency Compatibility

* Go module 基线保持 `go 1.22.6`，除非专门任务决定升级 Go 版本。
* 选择依赖时必须检查间接依赖是否把 `go.mod` 自动抬到 Go 1.23+。已知例子：Koanf `v2.2+` 及新版 provider 会要求 Go 1.23，当前配置层使用 `github.com/knadh/koanf/v2 v2.1.2`、`providers/confmap v0.1.0`，YAML/env 映射由项目代码转成 confmap。
* 数据层当前使用 `entgo.io/ent v0.13.1`。不要直接升级到 Ent `v0.14.3+`，这些版本会要求 Go 1.23+ 或更高；升级 Ent 前必须先有 Go baseline 升级任务。
* SQLite 当前使用 `modernc.org/sqlite v1.34.5`，对应 database/sql driver 名为 `sqlite`。新版 `modernc.org/sqlite v1.50.1` 已要求 Go 1.25，不能在 Go 1.22.6 baseline 下直接升级。
* PostgreSQL 集成测试当前使用 `github.com/testcontainers/testcontainers-go v0.35.0` 和 `modules/postgres v0.35.0`。新版 testcontainers-go 已要求 Go 1.25，不能在当前 baseline 下直接升级。
* PostgreSQL runtime driver 当前使用 `github.com/lib/pq v1.12.3`，配置方言字符串保持为 `postgres`。
* Wails 使用 `github.com/wailsapp/wails/v3 v3.0.0-alpha.9`，这是当前本机 Go 1.22.6 可编译的最新 v3 alpha；`alpha.10` 起要求 Go 1.24，`alpha.60` 起要求 Go 1.25。
* Wails v3 Windows 构建需要保持 `github.com/wailsapp/go-webview2 v1.0.19`，避免被旧依赖残留或手动升级抬到不兼容回调签名。
* Desktop 入口通过 `application.New`、`application.AssetFileServerFS(embedded.Assets)`、`NewWebviewWindowWithOptions` 与 `application.NewService` 加载前端资源和绑定服务。

---

## Naming Conventions

* 后端服务放在 `services/<service-name>`；Go 后端固定为 `services/taskdaemon-go`。
* 前端应用放在 `apps/<app-name>`；主 Web 管理台固定为 `apps/web`。
* 共享前端包放在 `packages/*`，例如 `packages/api-client`、`packages/ui`。
* Go package 目录使用小写单词，必要时使用短横线以外的自然组合；优先选择清晰的单词名，例如 `httpapi`、`scheduler`。
* Go service 对外发布入口统一为 `services/taskdaemon-go/cmd/taskdaemon`；第一版不创建多个 Go command 入口。
* spike 代码如果服务后续正式模块，优先放在目标模块附近，例如 gocron spike 放在 `services/taskdaemon-go/internal/scheduler` 附近。

---

## Examples

* gocron 调度库 spike：`services/taskdaemon-go/internal/scheduler` 附近的测试或实验代码，结论同步到调度核心任务。
* Web/Desktop 共用 UI：`apps/web`，Wails v3 通过 `services/taskdaemon-go/wails.json` 的嵌套 `frontend.dir = ../../apps/web` 指向开发期前端目录；发布期通过 `pnpm build:web` + `pnpm sync:web-assets` 将构建产物同步到 `services/taskdaemon-go/web/embedded/dist`。
