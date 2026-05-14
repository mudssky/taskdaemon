# taskdaemon

跨平台、低占用的定时任务守护进程。当前仓库处于第一轮骨架阶段：Go 后端、CLI、多入口、Wails Desktop 壳与 Vite/React 前端 workspace 会在同一仓库中协作。

## 开发命令

```bash
go test ./...
go run ./cmd/taskdaemon serve
go run ./cmd/taskdaemon serve --config ./config.yaml
go run ./cmd/taskdaemon db migrate
pnpm install
pnpm --filter @taskdaemon/web dev
pnpm --filter @taskdaemon/web typecheck
pnpm --filter @taskdaemon/web lint
pnpm exec lint-staged
pnpm build:web
pnpm sync:web-assets
```

## 项目结构

```text
cmd/taskdaemon/       # 单二进制入口：serve / desktop / CLI 子命令分发
internal/app/         # 应用装配与生命周期
internal/cli/         # Cobra CLI 命令树
internal/config/      # 配置默认值、文件、环境变量、flag 覆盖
internal/httpapi/     # Gin HTTP router
internal/desktop/     # Desktop 壳边界
internal/scheduler/   # 调度核心
web/app/              # Vite + React 管理台
web/embedded/         # 发布期前端嵌入边界
```

## 配置

默认配置路径为系统用户配置目录下的 `taskdaemon/config.yaml`，可通过 `--config <path>` 覆盖。配置覆盖顺序为：

1. 默认值
2. 配置文件
3. `TASKDAEMON_` 环境变量
4. CLI flag 或调用方 overrides

Swagger route 默认关闭，打开 `server.swagger.enabled` 后注册 `/swagger/index.html`。

## 前端嵌入

前端开发期在 `web/app` 独立运行。Desktop 壳使用 Wails v3，`wails.json` 采用 v3 的嵌套 `frontend` 配置并指向 `web/app`，发布前先执行 `pnpm build:web` 生成 `web/app/dist`，再执行 `pnpm sync:web-assets` 同步到 `web/embedded/dist`，由 Go `embed` 边界打入二进制。`web/embedded/dist` 会提交到仓库，保证干净 checkout 也能通过 Go 编译。

## 提交前检查

`pnpm install` 会通过 `prepare` 初始化 Husky。当前 `.husky/pre-commit` 运行 `pnpm exec lint-staged`，Go 文件会执行 `gofmt -w`，前端相关文件会执行 Biome lint。

## 发布体积基线

骨架任务会记录一次基础 release binary 体积，后续依赖增长以该记录为比较基线。
