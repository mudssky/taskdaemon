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
    ├── src/vite-env.d.ts       # Vite 客户端与 CSS side-effect import 类型声明
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
* `apps/web/dist` 是 Vite 构建输出，受 `.gitignore` 管理；发布前用 `pnpm sync:web-assets` 同步到 `services/taskdaemon-go/web/embedded/dist`。`services/taskdaemon-go/web/embedded/dist` 也视为生成产物，不提交真实 JS/CSS/HTML，只保留 `.gitkeep` 让 `go:embed all:dist` 在干净 checkout 下可编译。
* `services/taskdaemon-go/wails.json` 使用 Wails v3 嵌套配置：`frontend.dir = ../../apps/web`、`frontend.devServerUrl = http://127.0.0.1:9245`；不要恢复成 Wails 默认 `frontend/` 目录，也不要退回 v2 的 `frontend:*` 扁平键。桌面开发使用 `pnpm dev:desktop` 进入 Wails dev 模式，实际 dev watcher 配置在 `services/taskdaemon-go/build/config.yml`，前端仍由 Vite 提供 HMR。
* `apps/web/vite.config.ts` 的普通开发端口固定为 `9245`，Vite proxy 默认转发 `/api` 到 `http://127.0.0.1:39245`。Wails dev 会注入 `WAILS_VITE_PORT`，Vite 需要优先使用该端口；所有开发模式都启用 strict port，端口冲突时直接失败，不自动漂移到随机端口。并行多实例通过 `TASKDAEMON_WEB_PORT`、Wails `-port`、`TASKDAEMON_API_ORIGIN` 或 `VITE_TASKDAEMON_API_ORIGIN` 显式覆盖。
* `taskdaemon desktop` 在 Wails dev 模式下窗口直接打开 `FRONTEND_DEVSERVER_URL`，让 `/api` 由 Vite proxy 走普通网络请求；非 dev 桌面窗口打开本机 Go API server origin，由后端托管同步后的前端静态资源。不要让桌面前端从 `wails.localhost` 发 `/api` POST，否则 WebView 资源拦截层可能丢失 request body。
* TypeScript 6 会检查 CSS side-effect import 的类型声明；`apps/web/src/vite-env.d.ts` 必须保留 `/// <reference types="vite/client" />`，否则 `import "./styles.css"` 会在 `tsc --noEmit` 下失败。

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

* `features/tasks`：任务 CRUD 表单、任务列表、启停、手动触发；`tasks.queries.ts` 集中维护 TanStack Query key、查询和 mutation 失效逻辑。
* `features/runs`：执行历史表格、状态 badge、stdout/stderr 截断显示；历史查询复用任务 feature 的 `tasksKeys.runs(taskId)`。
* `features/scheduler`：cron 5/6 字段校验、高频/秒级软警告、确认提交状态；`validateCronExpression` 只做前端快速反馈，后端仍是最终校验来源。
* `features/auth`：首次创建管理员、单管理员登录、auth status 查询、受保护页面入口；入口分流使用 `GET /api/auth/status` 作为 app shell 的基础 query，`GET /api/auth/me` 保留给需要当前管理员详情的调用点。
* `lib/api`：手写轻量 API client 和 DTO 类型。API 错误必须映射为带 `status`、`code`、`details` 的 `ApiClientError`，组件不要解析后端 `message` 做稳定分支。

## Scenario: Basic Management UI

### 1. Scope / Trigger

* Trigger: 第一版 Web/Desktop 共用管理台落地，前端需要真实调用任务列表、创建、编辑、启停、触发、取消和执行历史 API。
* Scope: `apps/web/src/lib/api` 维护 HTTP 契约；`features/tasks` 维护表单、列表和 mutation；`features/runs` 维护历史展示；`features/scheduler` 维护 cron 前端反馈。

### 2. Signatures

* `apiClient.listTasks(): Promise<{ tasks: Task[] }>`
* `apiClient.createTask(payload: TaskPayload): Promise<Task>`
* `apiClient.updateTask(taskId: number, payload: TaskPayload): Promise<Task>`
* `apiClient.setTaskEnabled(taskId: number, enabled: boolean): Promise<Task>`
* `apiClient.triggerTask(taskId: number): Promise<TaskRun>`
* `apiClient.cancelTask(taskId: number): Promise<void>`
* `apiClient.listTaskRuns(taskId: number): Promise<{ runs: TaskRun[] }>`
* `validateCronExpression(expression: string, timezone: string): CronValidation`
* `toTaskPayload(values: TaskFormValues): TaskPayload`

### 3. Contracts

* `Task.runnerType` 使用集中 union：`shell | bash | pwsh | python | node | typescript`。
* `Task.running` 来自 daemon 进程内状态，只用于 UI 控制按钮和轮询判断，不写回表单。
* `TaskPayload.runner` 必须包含结构化字段：`type`、`inline` 或 `scriptPath`、`args`、`workDir`、`env`、`timeoutSeconds`、`outputLimitBytes`。
* cron 软警告包括 `second_level_cron` 和 `high_frequency_cron`；保存时必须把 `confirmCronWarnings=true` 传给后端。
* 任务列表和执行历史用表格展示；移动端只允许表格容器横向滚动，不能造成页面级横向滚动。

### 4. Validation & Error Matrix

* cron 字段数不是 5/6 -> 前端表单硬错误。
* timezone 为空或不符合 `Local` / `Area/City` 形态 -> 前端表单硬错误。
* 秒级或高频 cron 未确认 -> 前端阻止提交并显示确认控件。
* runner 既没有 inline 也没有 scriptPath -> 前端表单硬错误。
* API 返回非 2xx -> `ApiClientError(status, code, message, details)`；UI 按稳定 `code` 或泛化失败态展示，不依赖 message。

### 5. Good/Base/Bad Cases

* Good: 用户保存 TypeScript 脚本任务时，表单提交 `{ runner: { type: "typescript", scriptPath: "jobs/a.ts", timeoutSeconds: 3600 } }`，后端 runner 默认通过 `tsx` 执行。
* Base: `GET /api/tasks` 失败时显示可恢复错误态，成功后根据 `running` 短轮询列表和历史。
* Bad: 组件直接拼接 fetch payload 或在多个组件中重复写 runner 类型字符串。

### 6. Tests Required

* cron helper 覆盖 5/6 字段、非法字段数、timezone、秒级/高频 warning。
* task schema 覆盖 runner payload 映射、默认 timeout、TypeScript runner、env/args 映射和高频确认。
* API client 覆盖稳定错误码映射、trigger/cancel 关键动作。
* 组件交互覆盖创建/编辑任务、启停、触发、取消和历史查看中的关键业务分支；不测试纯 CSS。

### 7. Wrong vs Correct

#### Wrong

```tsx
fetch(`/api/tasks/${task.id}/trigger`, { method: "POST" });
```

#### Correct

```tsx
const triggerTask = useTriggerTaskMutation();
triggerTask.mutate(task.id);
```
