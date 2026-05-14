# Directory Structure

> How frontend code is organized in this project.

---

## Overview

taskdaemon 前端是 Web/Desktop 共用的管理台，放在 `web/app`。它是 Vite + React + TypeScript 应用，开发期可独立启动，发布期构建产物由 Wails/Go embed 打进单二进制。第一版不使用营销首页，首屏进入任务列表和运行状态。

当前仓库还未创建 `web/app`，目录规范来自父任务 PRD 与基础 UI 子任务：`.trellis/tasks/05-14-cross-platform-scheduler-daemon/prd.md`、`.trellis/tasks/05-14-basic-management-ui/prd.md`。

---

## Directory Layout

```text
web/
├── app/
│   ├── src/
│   │   ├── app/             # 应用装配：router、query client、providers
│   │   ├── routes/          # TanStack Router route definitions
│   │   ├── features/
│   │   │   ├── auth/        # 登录、session、受保护入口
│   │   │   ├── tasks/       # 任务列表、创建、编辑、启停、触发
│   │   │   ├── runs/        # 执行历史、截断输出、状态展示
│   │   │   └── scheduler/   # cron 校验、风险确认等前端业务逻辑
│   │   ├── components/
│   │   │   ├── ui/          # Radix + cva + tailwind-merge 本地基础组件
│   │   │   └── layout/      # sidebar、topbar、app shell
│   │   ├── lib/             # API client、dayjs、工具函数、常量
│   │   ├── styles/          # Tailwind 入口和全局样式
│   │   └── test/            # 测试工具与 render helpers
│   ├── public/              # 静态资源
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── biome.json
└── embedded/                # Go embed 包或构建产物挂载点，按实现调整
```

---

## Module Organization

* `src/app` 放跨应用装配，不放具体业务页面。
* `src/routes` 只做路由定义、loader/action 绑定和页面入口组合。
* `src/features/<domain>` 放领域 UI、hooks、schema、query keys、业务工具函数和测试。
* `src/components/ui` 放通用基础组件，按 shadcn 风格本地维护，不直接依赖远程生成代码作为运行时来源。
* `src/components/layout` 放 app shell、sidebar、topbar 等跨页面布局。
* `src/lib/api` 放 API client 和后端 DTO 映射；后续接 OpenAPI 生成时仍保持前端调用点稳定。
* `src/lib` 中的通用工具必须有明确复用场景；只被单个 feature 使用的逻辑留在 feature 内。

---

## Naming Conventions

* React 组件文件使用 PascalCase，例如 `TaskList.tsx`、`RunStatusBadge.tsx`。
* hook 文件使用 camelCase 并以 `use` 开头，例如 `useTaskActions.ts`。
* Zod schema 文件使用 `*.schema.ts`，query key 或 API helper 使用 `*.queries.ts` / `*.api.ts`。
* 测试文件与被测文件相邻，使用 `*.test.ts` 或 `*.test.tsx`。
* 路由文件遵循 TanStack Router 生成/约定模式；不要为了个人偏好创建第二套路由组织方式。

---

## Examples

当前还没有正式前端代码。第一批实现时可按以下垂直切片落地：

* `features/tasks`：任务 CRUD 表单、任务列表、启停、手动触发。
* `features/runs`：执行历史表格、状态 badge、stdout/stderr 截断显示。
* `features/scheduler`：cron 5/6 字段校验、高频/秒级软警告、确认提交状态。
* `features/auth`：单管理员登录、session 查询、受保护页面入口。
