# 项目骨架与多入口

## Goal

建立 taskdaemon 的第一轮工程骨架，让 Go 后端、Wails Desktop、Web 前端和 CLI 共享同一套项目结构，并支持开发期前后端单独启动、发布期前端嵌入二进制。

## Requirements

* 建立 Go + Gin 后端入口。
* 建立 Wails v3 Desktop 入口，Desktop 复用 Web UI。
* 建立 pnpm monorepo 与 Vite/React 前端工程。
* 前端质量链路参考 `vote-system`：Biome、typecheck、lint-staged、Husky pre-commit。
* 后端质量链路包含 `golangci-lint`。
* 同一二进制支持 `serve`、`desktop` 和 CLI 子命令模式。
* CLI 模式不启动桌面窗口。
* 默认配置文件读取系统用户配置目录，并支持 `--config <path>`。
* 发布构建路径明确前端构建产物如何嵌入二进制。
* release 构建需要启用 strip debug symbols 和 `-trimpath`。
* 骨架阶段需要记录一次基础 release binary 体积。
* Swagger UI/docs route 需要配置开关控制，关闭时不注册文档路由。
* 已确认整体目录结构采用根 Go module + `cmd/`、`internal/`、`web/` 的轻量 monorepo 方案。
* Go module 位于仓库根目录，后端业务代码按 `cmd/`、`internal/` 分层组织。
* 前端位于 `web/app`，保留 pnpm workspace 能力，避免 Wails 默认 `frontend/` 目录与后续 Web/Desktop 复用边界混淆。

## Acceptance Criteria

* [x] 后端 API 可单独启动。
* [x] 前端开发服务器可单独启动并访问后端 API。
* [x] Desktop 模式可加载同一套前端 UI。
* [x] CLI 子命令入口存在且不启动 Desktop。
* [x] 默认配置路径与 `--config` 覆盖逻辑可用。
* [x] README 或开发文档说明开发/构建命令。
* [x] 前端 `biome`、`typecheck`、`lint-staged` 与 Husky pre-commit 基础配置存在。
* [x] 后端 `golangci-lint` 基础配置存在。
* [x] release 构建产物体积有记录，作为后续依赖增长基线。
* [x] CLI-only 模式不会初始化 Desktop 窗口逻辑。
* [x] Swagger 文档路由可通过配置开启或关闭。
* [x] 整体目录结构方案记录在 PRD 或 spec 中，并被 gocron spike 复用。

## Testing Constraints

* 入口分发、配置路径解析、CLI-only 不启动 Desktop 等逻辑采用 TDD。
* 样式、静态配置文件、文档不要求单元测试。

## Out of Scope

* 不实现完整业务 API。
* 不实现完整通知系统。
* 不实现系统服务安装器。

## Directory Structure Decision

采用根 Go module + 前端 workspace 的结构：

```text
.
├── cmd/
│   └── taskdaemon/          # 单二进制入口：serve / desktop / CLI 子命令分发
├── internal/
│   ├── app/                 # 应用装配、依赖注入、生命周期
│   ├── config/              # 配置加载、默认路径、flag/env 合并
│   ├── httpapi/             # Gin router、middleware、handler
│   ├── scheduler/           # cron 调度核心与 trigger 边界
│   ├── runner/              # shell/bash/pwsh/python/node/typescript runner
│   ├── data/                # Ent client、repository、migration 边界
│   ├── auth/                # 单管理员、session、CSRF/Host 安全边界
│   └── desktop/             # Wails 绑定、桌面生命周期与原生能力
├── web/
│   ├── app/                 # Vite + React 管理台，Web/Desktop 共用
│   └── embedded/            # Go embed 包或构建产物挂载点，可按实现调整
├── docs/                    # 开发、构建、release 体积记录
├── scripts/                 # 本地开发与构建脚本
├── go.mod
├── wails.json
├── package.json
├── pnpm-workspace.yaml
└── README.md
```

决策理由：

* `cmd/taskdaemon` 是唯一发布入口，CLI-only、serve、desktop 都由同一二进制模式分发。
* `internal/desktop` 只放 Wails v3 绑定和桌面专属能力，业务能力仍通过 `internal/app`、`internal/scheduler`、`internal/runner` 等共享。
* `web/app` 明确是可独立运行的前端应用；Wails v3 通过配置引用它的 build 输出。
* `web/embedded` 是否需要独立 Go package 可在实现时根据 Wails embed 与 serve 静态资源复用方式微调。
* gocron spike 若进入正式 module，优先放在 `internal/scheduler` 附近，避免创建临时结构后再搬迁。
* 不采用完整 `apps/*`、`packages/*` 重 monorepo 作为第一版结构；后续若前端共享组件或生成 client 变多，可在 `web/packages/*` 中自然扩展。

## Research References

* Context7 `/websites/v3_wails_io` 文档显示 Wails v3 使用 `application.New`、`application.AssetFileServerFS` 和 `NewWebviewWindowWithOptions` 装配 Desktop；`wails.json` 的前端配置从 v2 `frontend:*` 扁平键迁移为 v3 嵌套 `frontend` 对象。
* 本机 Go 1.22.6 下可编译的 Wails v3 版本 pin 为 `github.com/wailsapp/wails/v3 v3.0.0-alpha.9`；`alpha.10` 起要求 Go 1.24，`alpha.60` 起要求 Go 1.25，后续升级 Wails 前需先升级 Go 基线并重新验证 Windows `go-webview2` 组合。

## Parent

* `.trellis/tasks/05-14-cross-platform-scheduler-daemon/prd.md`
