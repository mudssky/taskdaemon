# 前端目录结构

> 前端代码按应用装配、路由、feature、通用组件和库边界组织。

---

## 概览

taskdaemon 前端是 Web/Desktop 共用的管理台，放在 `apps/web`。它是 Vite + React + TypeScript 应用，开发期可独立启动，发布期构建产物由 `scripts/sync-web-assets.ps1` 同步到 `services/taskdaemon-go/web/embedded/dist`，再由 Wails/Go embed 打进 Go 后端发布产物。

根目录是 pnpm workspace；`apps/*` 放应用，`packages/*` 预留共享前端包，`services/*` 中允许带 `package.json` 的后端服务提供脚本门面。

---

## 目录布局

```text
apps/
└── web/
    ├── src/
    │   ├── app/             # 应用装配：query client、providers 等
    │   ├── routes/          # TanStack Router route definitions
    │   ├── features/
    │   │   ├── auth/        # 登录、初始化管理员、受保护入口
    │   │   ├── tasks/       # 任务列表、创建、编辑、启停、触发
    │   │   ├── runs/        # 执行历史、状态展示
    │   │   ├── scheduler/   # cron 校验、风险确认等业务逻辑
    │   │   └── status/      # 运行状态概览
    │   ├── components/
    │   │   ├── ui/          # 本地基础组件
    │   │   └── layout/      # app shell、sidebar、topbar
    │   ├── lib/             # API client、dayjs、通用工具
    │   ├── test/            # 测试工具与 render helpers
    │   ├── main.tsx
    │   ├── styles.css
    │   └── vite-env.d.ts
    ├── public/
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json
    └── biome.json
services/
└── taskdaemon-go/
    └── web/embedded/        # Go embed 包，dist/ 由构建脚本同步
```

---

## 模块边界

* `src/app` 放跨应用装配，不放具体业务页面。
* `src/routes` 只做路由定义、loader/action 绑定和页面入口组合；复杂业务逻辑下沉到 feature。
* `src/features/<domain>` 放领域 UI、hooks、schema、query keys、业务工具函数和测试。
* `src/components/layout` 放 app shell、sidebar、topbar 等跨页面布局。
* `src/components/ui` 放通用基础组件；当前项目只有少量基础 UI，新增前先确认有复用场景。
* `src/lib/api` 放 API client 和后端 DTO 映射；后续接 OpenAPI 生成时仍保持调用点稳定。
* `src/lib` 中的通用工具必须有明确复用场景；只被单个 feature 使用的逻辑留在 feature 内。
* 测试文件与被测文件相邻，跨测试共享工具放入 `src/test`。

---

## 路由与页面

* sidebar 顶级导航目标必须对应清晰的页面组件。
* 不要让 `/`、`/tasks`、`/runs` 等不同入口全部渲染同一个聚合大组件；共享内容应拆成 feature 组件，由各页面组合。
* 任务创建/编辑这类复杂表单使用独立路由页面承载，例如 `/tasks/new` 和 `/tasks/$taskId/edit`。
* `/tasks` 负责列表、筛选和行级操作入口，不内嵌复杂创建/编辑表单。
* 页面组件保持薄层：组合路由参数、query/mutation hook 和 feature 组件。

---

## API 与 Feature 约定

* API client 封装 fetch、credentials、envelope 解包和错误映射。
* 组件不要直接拼 API payload；表单 schema 或 feature 工具函数负责 DTO 映射。
* query key 放在 feature 附近集中维护，例如 `tasksKeys`。
* mutation 成功后显式 invalidate 或 remove 相关 query cache。
* 执行状态、任务状态、runner 类型和触发来源使用集中 union/常量，不在组件里重复裸字符串。
* 前端可以做快速校验和软警告展示，后端仍是最终校验来源。

---

## 构建与资源

* `apps/web/dist` 是 Vite 构建输出，受 `.gitignore` 管理。
* 发布前用 `pnpm sync:web-assets` 同步到 `services/taskdaemon-go/web/embedded/dist`。
* `services/taskdaemon-go/web/embedded/dist` 视为生成产物，不提交真实 JS/CSS/HTML，只保留 `.gitkeep` 让 `go:embed all:dist` 可编译。
* Web favicon 放在 `apps/web/public/favicon.png`，并由 `apps/web/index.html` 使用 `/favicon.png` 引用。
* Web/Desktop 品牌图标保持同源；浏览器 favicon 使用压缩副本，不直接使用高分辨率母版。

---

## Vite 与 Desktop

* `apps/web/vite.config.ts` 普通开发端口固定为 `9245`，strict port 开启，端口冲突时 fail fast。
* Vite proxy 默认转发 `/api` 到 `http://127.0.0.1:39245`。
* Wails dev 会注入 `WAILS_VITE_PORT`，Vite 需要优先使用该端口。
* 并行多实例通过 `TASKDAEMON_WEB_PORT`、Wails `-port`、`TASKDAEMON_API_ORIGIN` 或 `VITE_TASKDAEMON_API_ORIGIN` 显式覆盖。
* 桌面开发使用 `pnpm dev:desktop` 进入 Wails dev 模式，前端仍由 Vite 提供 HMR。
* 桌面前端的 `/api` 请求走 Vite proxy 或 Go API server 普通网络路径，不依赖 `wails.localhost` AssetServer 转发 POST body。

---

## 命名约定

* React 组件文件使用 PascalCase，例如 `TaskForm.tsx`、`RunHistoryTable.tsx`。
* hook 文件使用 camelCase 并以 `use` 开头。
* Zod schema 文件使用 `*.schema.ts`。
* TanStack Query helper 使用 `*.queries.ts`。
* API helper 使用 `*.api.ts` 或集中在 `lib/api`。
* 测试文件与被测文件相邻，使用 `*.test.ts` 或 `*.test.tsx`。
* 路由文件遵循 TanStack Router 当前组织方式，不为个人偏好创建第二套路由结构。

---

## 常见错误

* 不创建营销式 landing page 作为应用第一屏。
* 不让 Desktop 和 Web 分叉成两套组件。
* 不绕过 `apiClient` 在组件里直接 `fetch` 业务接口。
* 不把只被单个 feature 使用的工具提前放进全局 `lib`。
* 不提交 `dist` 中真实构建产物。
