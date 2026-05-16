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
│       ├── build/
│       │   └── config.yml           # Wails v3 开发模式配置
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
* `services/taskdaemon-go/internal/config` 放默认配置、用户配置目录解析、文件/env/flag 覆盖和配置测试。未显式 `--config` 时覆盖顺序固定为 defaults < project/user file < project local file < `TASKDAEMON_` env < overrides；显式 `--config` 时为 defaults < explicit file < `TASKDAEMON_` env < overrides。
* `services/taskdaemon-go/internal/httpapi` 放 HTTP router、middleware、request/response DTO 和 handler；handler 调用 service 边界，不直接散落数据库方言。
* `services/taskdaemon-go/internal/scheduler` 放 cron 调度、任务注册、手动触发和 overlap 处理边界。
* `services/taskdaemon-go/internal/runner` 放内置结构化 runner、进程执行、timeout/cancel、输出截断。
* `services/taskdaemon-go/internal/data` 放 Ent client、repository、migration 和数据库方言隔离。
* `services/taskdaemon-go/internal/auth` 放单管理员账号、登录态、CSRF/Host 校验等认证安全能力。
* `services/taskdaemon-go/internal/desktop` 只放 Wails 绑定与桌面专属能力，不能复制 HTTP/API/调度业务逻辑；Desktop 资产来自 `taskdaemon/web/embedded` 包。
* `services/taskdaemon-go/web/embedded` 是发布期 Go embed 边界，构建脚本把 `apps/web/dist` 同步到 `services/taskdaemon-go/web/embedded/dist` 后再构建二进制；该 `dist` 是生成产物，不提交真实 JS/CSS/HTML，只提交 `.gitkeep` 保证干净 checkout 下 `go:embed all:dist` 目标存在。

---

## Entry Contracts

* `taskdaemon serve` 启动 HTTP API，不初始化 Wails Desktop。
* `taskdaemon desktop` 启动 Wails v3 runtime，同时启动本机 HTTP API server。Wails dev 模式下窗口直接打开 `FRONTEND_DEVSERVER_URL`；非 dev 模式下窗口打开本机 Go API server origin，由 `internal/httpapi` 从 `services/taskdaemon-go/web/embedded/dist` 托管前端静态资源和 SPA fallback。
* `pnpm dev:desktop` 通过 `go tool wails3 dev -config ./build/config.yml -port 9245` 启动 Wails v3 开发模式；前端走 Vite HMR，Go 代码变更由 Wails 监控后重建并重启桌面壳。
* `taskdaemon db migrate` 是 CLI-only 入口，不初始化 Desktop。
* `taskdaemon config path/show/validate/reload` 是 CLI-only 配置排障入口；`reload` 通过 daemon HTTP API 触发运行时重载，需要管理员 session token。
* 开发期从 workspace 根目录运行 CLI 优先用 `pnpm cli -- <args>`，它会透传到 `go run ./cmd/taskdaemon <args>`；需要安装真实命令时使用 `pnpm install:cli`。
* `--config <path>` 是根级 persistent flag，所有子命令都应加载同一份配置。
* 未显式传入 `--config` 时，`internal/config` 只在开发工作区里自动查找项目内配置文件，按 `taskdaemon.yaml`、`taskdaemon.yml`、`config.yaml`、`config.yml` 的顺序匹配；非工作区环境不会自动读取当前目录同名文件，找不到时回退到 `os.UserConfigDir()/taskdaemon/config.yaml`。
* 未显式传入 `--config` 且处于开发工作区时，`internal/config` 会额外查找同目录本地覆盖文件，按 `taskdaemon.local.yaml`、`taskdaemon.local.yml`、`config.local.yaml`、`config.local.yml` 的顺序匹配，并在基础配置后、环境变量前叠加。
* 显式 `--config <path>` 只读取指定文件，不再叠加项目内配置文件或 local 配置。
* 默认配置路径为 `os.UserConfigDir()/taskdaemon/config.yaml`。
* 配置示例归属 Go service 边界，放在 `services/taskdaemon-go/taskdaemon.example.yaml`；开发时可复制为 `services/taskdaemon-go/taskdaemon.yaml`，本机私有覆盖写入 `services/taskdaemon-go/taskdaemon.local.yaml`，发布时复制到用户配置目录或通过 `--config` 显式指定。
* 运行时配置重载只应用 `logging.http` 与 `observability.traceId`。`server.*`、`database.*`、`logging.level/output/file/console` 这类涉及监听地址、连接池或 handler/文件句柄的配置变更需要重启。
* 开发默认端口固定为前端 `127.0.0.1:9245`、后端 `127.0.0.1:39245`。端口冲突时应 fail fast，不自动漂移；并行多实例通过 `TASKDAEMON_SERVER_PORT`、配置文件 `server.port`、Wails `-port`、`TASKDAEMON_WEB_PORT` 和前端代理目标环境变量显式覆盖。
* `/api/health` 是后端探活 endpoint；Swagger route 默认关闭，只在 `server.swagger.enabled=true` 时注册。启用前端静态资源时，`internal/httpapi` 的 SPA fallback 不处理 `/api` 和 `/swagger` 保留路径，避免把真实后端 404 伪装成前端页面。

---

## Dependency Compatibility

* Go module 基线保持 `go 1.26.3`。依赖升级前先确认本机 `go version` 与 `services/taskdaemon-go/go.mod` 一致，再运行 `go mod tidy`。
* Go 后端业务代码、配置、测试和 Ent JSON 字段默认使用标准库 `encoding/json`。不要新增直接第三方 JSON 依赖；只有 profiling 证明 HTTP hot path 的 JSON 编解码是瓶颈时，才在 `internal/httpapi` 局部评估替换。
* 配置层使用 `github.com/knadh/koanf/v2 v2.3.4` 与 `github.com/knadh/koanf/providers/confmap v1.0.0`；YAML/env 映射仍由项目代码转成 confmap，避免配置层散落 provider 差异。
* 数据层当前使用 `entgo.io/ent v0.14.6`。升级 Ent 后必须在 `services/taskdaemon-go` 内运行 `go generate ./internal/data/ent`，并提交 `internal/data/ent` 生成代码 diff。
* SQLite 当前使用 `modernc.org/sqlite v1.50.1`，对应 database/sql driver 名为 `sqlite`。
* PostgreSQL 集成测试当前使用 `github.com/testcontainers/testcontainers-go v0.42.0` 和 `modules/postgres v0.42.0`。
* PostgreSQL runtime driver 当前使用 `github.com/lib/pq v1.12.3`，配置方言字符串保持为 `postgres`。
* Wails 使用 `github.com/wailsapp/wails/v3 v3.0.0-alpha.91`。Wails CLI 通过 Go tool 依赖管理，脚本使用 `go tool wails3`，不要在 `package.json` 中重复硬编码 `@version`。Desktop 入口通过 `application.New`、`application.AssetFileServerFS(embedded.Assets)`、`desktopApp.Window.NewWithOptions(...)` 与 `application.NewService` 加载前端资源和绑定服务；不要恢复旧的 `NewWebviewWindowWithOptions` 调用。桌面窗口的 `/api` 请求必须走 Vite proxy 或 Go API server 的普通网络路径，不要依赖 `wails.localhost` AssetServer 转发 POST body。

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
* Web/Desktop 共用 UI：`apps/web`，Wails v3 通过 `services/taskdaemon-go/wails.json` 的嵌套 `frontend.dir = ../../apps/web` 指向开发期前端目录；开发桌面壳使用 `pnpm dev:desktop`，发布期通过 `pnpm build:web` + `pnpm sync:web-assets` 将构建产物同步到 `services/taskdaemon-go/web/embedded/dist`。
