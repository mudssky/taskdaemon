# pnpm Workspace Boundary

## 问题

当前仓库已经有根 `package.json`、`pnpm-workspace.yaml` 和 `web/app`，但 Go module 仍在仓库根目录。用户现在希望按前后端分离架构重新考虑目录结构：Go 只是其中一个后端，而不是整个仓库的唯一主轴。

## Context7 结论

已使用 Context7 查询 pnpm 官方文档：

```bash
npx ctx7@latest library pnpm "前后端分离架构下 pnpm workspace 目录结构 packages apps 配置"
npx ctx7@latest docs /pnpm/pnpm.io "前后端分离架构下 pnpm workspace 目录结构 packages apps 配置"
```

pnpm workspace 通过 `pnpm-workspace.yaml` 的 `packages` glob 管理 JS/TS workspace package，例如：

```yaml
packages:
  - "packages/*"
  - "apps/*"
  - "tools/*"
```

这说明 pnpm workspace 可以覆盖任意带 `package.json` 的 workspace 目录，例如 `apps/*`、`packages/*`、`services/*`。它不要求整个仓库都变成纯前端 monorepo，也不会天然管理 Go module；对 Go service 来说，pnpm 只作为脚本调度层，依赖仍由 `go.mod` 管理。

## 对本项目的映射

可行方向有三类：

1. 保持当前轻量结构：`web/app` + 根 Go module。
2. 改为产品级分离结构：`apps/web`、`services/taskdaemon-go`、`packages/*`，其中 `services/taskdaemon-go/package.json` 提供脚本门面。
3. 折中结构：把前端迁到 `apps/web`，Go 保持根 module，但文档上声明 Go 是后端服务之一。

如果目标是“前后端分离，Go 只是其中一个后端”，第 2 类结构最清晰：根目录成为产品 workspace，前端 apps、共享 TS packages、后端 services 并列；Go module 移入 `services/taskdaemon-go` 或类似目录。

## 约束

* 当前 Wails v3 配置指向 `web/app`，如果迁移到 `apps/web` 或 `apps/desktop`，需要同步 `wails.json`、embed 路径、构建脚本和 lint-staged。
* 当前 Go import module 是 `taskdaemon`，如果移动 Go module，会影响命令路径、相对路径、CI/脚本和 Trellis spec。
* 当前 `web/embedded/dist` 被提交以保证 Go embed 干净 checkout 可编译；迁移后仍需保留等价策略。

## Wails v3 补充

已使用 Context7 查询 Wails v3 官方文档：

```bash
npx ctx7@latest library Wails "Wails v3 project structure wails.json frontend.dir desktop app root Go module"
npx ctx7@latest docs /websites/v3_wails_io "Wails v3 wails.json frontend.dir project root Go module external frontend directory"
```

文档显示 Wails v3 使用嵌套 `frontend` 配置：

```json
{
  "frontend": {
    "dir": "./frontend",
    "install": "npm install",
    "build": "npm run build",
    "dev": "npm run dev",
    "devServerUrl": "http://localhost:5173"
  }
}
```

这支持我们在 Go service module 的 `wails.json` 中把 `frontend.dir` 指向 workspace 外层的前端 app，例如 `../../apps/web`。Wails 典型结构仍是 Go app 负责桌面运行时和服务绑定，因此如果把 Desktop 单独放到 `apps/desktop`，需要特别处理 Go `internal` 可见性和服务复用边界。
