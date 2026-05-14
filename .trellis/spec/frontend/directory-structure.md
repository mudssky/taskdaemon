# Directory Structure

> How frontend code is organized in this project.

---

## Overview

taskdaemon 前端是 Web/Desktop 共用的管理台，放在 `apps/web`。它是 Vite + React + TypeScript 应用，开发期可独立启动，发布期构建产物由 `scripts/sync-web-assets.ps1` 同步到 `services/taskdaemon-go/web/embedded/dist`，再由 Wails/Go embed 打进 Go 后端发布产物。第一版不使用营销首页，首屏进入任务列表和运行状态。

根目录是 pnpm workspace；`apps/*` 放应用，`packages/*` 放共享前端包，`services/*` 中允许带 `package.json` 的后端服务提供脚本门面。

---

## Directory Layout

```text
apps/
└── web/
    ├── src/
    │   ├── app/             # 应用装配：router、query client、providers
    │   ├── routes/          # TanStack Router route definitions
    │   ├── features/
    │   │   ├── auth/        # 登录、session、受保护入口
    │   │   ├── tasks/       # 任务列表、创建、编辑、启停、触发
    │   │   ├── runs/        # 执行历史、截断输出、状态展示
    │   │   └── scheduler/   # cron 校验、风险确认等前端业务逻辑
    │   ├── components/
    │   │   ├── ui/          # Radix + cva + tailwind-merge 本地基础组件
    │   │   └── layout/      # sidebar、topbar、app shell
    │   ├── lib/             # API client、dayjs、工具函数、常量
    │   ├── styles/          # Tailwind 入口和全局样式
    │   └── test/            # 测试工具与 render helpers
    ├── public/              # 静态资源
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json
    └── biome.json
packages/
├── api-client/              # 未来 OpenAPI/TS client
└── ui/                      # 未来共享 UI primitives
services/
└── taskdaemon-go/
    └── web/embedded/        # Go embed 包，dist/ 由构建脚本同步
```

---

## Module Organization

* `apps/web/src/app` 放跨应用装配，不放具体业务页面。
* `apps/web/src/routes` 只做路由定义、loader/action 绑定和页面入口组合。
* `apps/web/src/features/<domain>` 放领域 UI、hooks、schema、query keys、业务工具函数和测试。
* `apps/web/src/components/ui` 放通用基础组件，按 shadcn 风格本地维护，不直接依赖远程生成代码作为运行时来源。
* `apps/web/src/components/layout` 放 app shell、sidebar、topbar 等跨页面布局。
* `apps/web/src/lib/api` 放 API client 和后端 DTO 映射；后续接 OpenAPI 生成时仍保持前端调用点稳定。
* `apps/web/src/lib` 中的通用工具必须有明确复用场景；只被单个 feature 使用的逻辑留在 feature 内。
* `apps/web/dist` 是 Vite 构建输出，受 `.gitignore` 管理；发布前用 `pnpm sync:web-assets` 同步到 `services/taskdaemon-go/web/embedded/dist`，该目录由 Go embed 编译进二进制并需要提交，保证干净 checkout 也能通过 Go 编译。
* `services/taskdaemon-go/wails.json` 使用 Wails v3 嵌套配置：`frontend.dir = ../../apps/web`、`frontend.devServerUrl = http://127.0.0.1:5173`；不要恢复成 Wails 默认 `frontend/` 目录，也不要退回 v2 的 `frontend:*` 扁平键。

---

## Naming Conventions

* React 组件文件使用 PascalCase，例如 `TaskList.tsx`、`RunStatusBadge.tsx`。
* hook 文件使用 camelCase 并以 `use` 开头，例如 `useTaskActions.ts`。
* Zod schema 文件使用 `*.schema.ts`，query key 或 API helper 使用 `*.queries.ts` / `*.api.ts`。
* 测试文件与被测文件相邻，使用 `*.test.ts` 或 `*.test.tsx`。
* 路由文件遵循 TanStack Router 生成/约定模式；不要为了个人偏好创建第二套路由组织方式。

---

## Examples

当前前端只有 app shell 骨架。第一批业务实现时可按以下垂直切片落地：

* `features/tasks`：任务 CRUD 表单、任务列表、启停、手动触发。
* `features/runs`：执行历史表格、状态 badge、stdout/stderr 截断显示。
* `features/scheduler`：cron 5/6 字段校验、高频/秒级软警告、确认提交状态。
* `features/auth`：单管理员登录、session 查询、受保护页面入口。
