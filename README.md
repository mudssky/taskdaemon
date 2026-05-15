# taskdaemon

跨平台、低占用的定时任务守护进程。当前仓库采用 pnpm monorepo 形态：`apps/*` 放前端应用，`services/*` 放后端服务，`packages/*` 预留共享前端包和 API client。Go 后端位于 `services/taskdaemon-go`，同时承载 HTTP API、CLI、daemon 和 Wails Desktop 发布模式。

## 开发命令

```bash
pnpm install
pnpm build:web
pnpm sync:web-assets
pnpm dev:web
pnpm dev:backend
pnpm dev:desktop
pnpm desktop
pnpm migrate
pnpm typecheck
pnpm lint
pnpm test:go
pnpm vet:go
pnpm exec lint-staged

cd services/taskdaemon-go
go test ./...
go run ./cmd/taskdaemon serve
go run ./cmd/taskdaemon serve --config ./taskdaemon.yaml
go run ./cmd/taskdaemon db migrate
```

## 项目结构

```text
apps/web/                         # Vite + React 管理台
packages/                         # 共享前端包、API client、UI primitives 等
services/taskdaemon-go/cmd/       # Go service 入口：serve / desktop / CLI 子命令分发
services/taskdaemon-go/internal/  # Go service 内部业务包
services/taskdaemon-go/web/       # 发布期前端嵌入边界
scripts/                          # 本地开发与构建脚本
```

## 配置

默认配置路径为系统用户配置目录下的 `taskdaemon/config.yaml`，可通过 `--config <path>` 覆盖。配置覆盖顺序为：

1. 默认值
2. 配置文件：开发工作区启动目录中的项目配置文件，或系统用户配置目录下的 `taskdaemon/config.yaml`
3. `TASKDAEMON_` 环境变量
4. CLI flag 或调用方 overrides

项目配置文件按 `taskdaemon.yaml`、`taskdaemon.yml`、`config.yaml`、`config.yml` 顺序查找，仅用于开发工作区。项目内未找到时才回退到用户配置目录；如果显式传入 `--config <path>`，则只读取该文件，不再自动叠加项目内配置文件。发布环境默认只依赖系统用户配置目录。

配置示例位于 `services/taskdaemon-go/taskdaemon.example.yaml`。开发时可复制为 `services/taskdaemon-go/taskdaemon.yaml` 后在 Go service 目录启动；发布部署时建议复制到用户配置目录，或通过 `--config` 显式指定。

Swagger route 默认关闭，打开 `server.swagger.enabled` 后注册 `/swagger/index.html`。

## 前端嵌入

前端开发期在 `apps/web` 独立运行。普通 Web 开发使用 `pnpm dev:web`，Vite 默认监听 `127.0.0.1:5173`。Desktop 壳使用 Wails v3，开发桌面端时使用 `pnpm dev:desktop` 进入 Wails dev 模式：Wails 读取 `services/taskdaemon-go/build/config.yml`，后台启动 Vite，前端由 Vite 提供 HMR，Go 代码变更由 Wails 监控后重建并重启桌面壳。

`services/taskdaemon-go/package.json` 通过 `go tool wails3` 调用 Wails CLI，实际 Wails 版本由 `services/taskdaemon-go/go.mod` 的 `github.com/wailsapp/wails/v3` 与 `tool github.com/wailsapp/wails/v3/cmd/wails3` 管理。升级 Wails 时在 Go module 内更新依赖即可，脚本不需要同步改 `@version`。

发布前先执行 `pnpm build:web` 生成 `apps/web/dist`，再执行 `pnpm sync:web-assets` 同步到 `services/taskdaemon-go/web/embedded/dist`，由 Go `embed` 边界打入二进制。`services/taskdaemon-go/web/embedded/dist` 会提交到仓库，保证干净 checkout 也能通过 Go 编译。

## 提交前检查

`pnpm install` 会通过 `prepare` 初始化 Husky。当前 `.husky/pre-commit` 运行 `pnpm exec lint-staged`，Go 文件会执行 `gofmt -w`，前端相关文件会执行 Biome lint。

## 发布体积基线

骨架任务会记录一次基础 release binary 体积，后续依赖增长以该记录为比较基线。
