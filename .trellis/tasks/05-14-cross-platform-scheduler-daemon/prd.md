# 跨平台低占用定时任务守护进程

## Goal

设计并实现一个跨平台、常驻后台、资源占用低的定时任务守护进程。项目的直接起点是定时执行 PostgreSQL/pgBackRest 备份脚本，但产品形态应抽象为通用任务调度工具：同一套核心能力同时服务 Web 管理台、Desktop 壳、HTTP API 和 CLI。

## Requirements

### 产品形态

* 使用 Go 作为首个后端服务，目标是跨平台、低占用、单二进制发布。
* 仓库采用 pnpm monorepo：`apps/*` 放前端应用，`services/*` 放后端服务，`packages/*` 放共享前端包和 API client。
* 当前 Go 后端服务位于 `services/taskdaemon-go`，前端管理台位于 `apps/web`。
* 使用 Wails 承载 Desktop 壳和前端资源打包；Desktop 与 Web 端复用同一套 UI。
* Desktop 第一阶段主要补充系统原生通知能力，不维护独立页面。
* 前端与后端在开发期必须能单独启动。
* 发布期前端构建产物需要嵌入最终二进制。
* 同一个二进制需要支持 CLI 执行，CLI 模式不应强制启动桌面窗口。
* 默认配置文件放在系统用户配置目录，并支持 `--config <path>` 覆盖。

### 第一轮范围：调度核心可用

* 第一轮实施目标是让系统真实可定义、触发、执行任务并查看执行历史。
* 第一轮包含项目骨架、多入口、Ent 多数据库数据层、单管理员登录、任务定义、调度核心、runner、CLI 手动触发、取消运行中任务、基础 Web 管理 UI。
* 第一轮前端做到基础 CRUD UI：任务创建、编辑、启停、手动触发、取消运行中任务、查看执行历史。
* 完整通知中心、完整设置页、备份模板、高级 runner 配置、Webhook、邮件、插件系统、系统服务安装器拆到第二轮或后续任务。

### 访问与认证

* Web 访问边界按公网访问可演进设计，避免锁死为本机单用户工具。
* 第一版采用单管理员账号模型。
* 登录后可管理任务、触发任务、取消任务、查看历史。
* 未登录请求不能创建、编辑、删除、启停或触发任务。
* 第一版不实现多用户、多租户或 RBAC。
* 第一版需要为公网可演进场景保留登录态、CSRF/Host 校验、反向代理/TLS 部署边界等安全设计。

### 数据层

* SQLite 与 PostgreSQL 从第一版开始作为一等目标设计。
* 默认数据库为 SQLite，PostgreSQL 可通过配置启用。
* 使用 Ent 作为数据建模和访问基础，优先 schema/查询的强类型约束和编辑期检查体验。
* Ent 细节封装在服务/仓储边界内，避免业务层散落数据库方言。
* 第一版支持对当前选定数据库执行 schema migration。
* 第一版不实现 SQLite 与 PostgreSQL 之间的一键数据迁移；后续可通过 export/import 或 transfer 命令实现。

### 调度

* 第一版使用进程内 cron + 手动触发，不引入 Redis、Asynq 或外部分布式队列。
* cron 使用 `robfig/cron/v3`。
* cron 表达式支持 5 字段和 6 字段，秒字段可选。
* 使用秒级 cron 或高频表达式时，前端需要提示风险并提供显式确认入口。
* 后端校验区分硬错误和软警告：
  * 硬错误不能保存，例如 cron 语法非法、字段数量非法、timezone 非法、runner 必填字段缺失。
  * 软警告允许用户显式确认后保存，例如秒级/高频表达式。
* 同一任务默认不允许重叠执行；上一次仍在运行时，下一次触发记录为 `skipped`，不启动新进程。

### Runner 与执行

* 第一版只允许内置结构化 runner 类型，不提供裸的任意命令输入框。
* 初始 runner 覆盖 `shell`、`bash`、`pwsh`、`python`、`node`、`typescript`。
* TypeScript runner 默认使用 `tsx`，不使用 `ts-node` 作为默认方案。
* runner 配置需要结构化字段：脚本路径或命令片段、参数、工作目录、环境变量、超时等。
* 每个任务必须配置 timeout，默认 1 小时，可按任务调整。
* 超时后终止进程并记录为 `timeout`。
* 第一轮支持手动取消正在运行的任务；API/CLI 可请求取消，后端终止进程并记录为 `cancelled`。
* 第一版执行历史在数据库保存截断后的 stdout/stderr，不保存完整日志文件。
* 每次执行至少保存任务 ID、触发来源、状态、退出码、开始/结束时间、耗时、错误摘要、stdout/stderr 截断内容。

### API / CLI / UI

* 后端 API 需要被 Web 管理台、CLI 和未来 Webhook 触发路径复用。
* CLI 至少支持启动模式、schema migration、任务手动触发、取消运行中任务和查看执行历史的核心路径。
* Web/Desktop 共用基础管理 UI，支持任务 CRUD、启停、触发、取消和历史查看。
* 通知系统第一轮只保留任务事件/通知扩展点；Desktop 原生通知和完整通知中心进入后续任务。

## Acceptance Criteria

* [ ] 开发期可以分别启动后端 API 和前端开发服务器。
* [ ] 发布构建方案明确前端资源如何进入单二进制。
* [ ] CLI 模式、serve 模式、desktop 模式入口边界明确。
* [ ] 默认配置路径跨平台合理，并允许通过 `--config` 指定。
* [ ] 默认配置可使用 SQLite 启动。
* [ ] 可通过配置连接 PostgreSQL。
* [ ] CLI 或启动流程支持对当前数据库执行 schema migration。
* [ ] 系统初始化/登录流程支持单管理员账号。
* [ ] 未登录请求不能创建、编辑、删除、启停或触发任务。
* [ ] 可通过 API/CLI/UI 定义至少一个 cron 任务。
* [ ] cron parser 支持 5/6 字段表达式。
* [ ] 前端对秒级/高频 cron 给出提示和确认提交入口。
* [ ] 后端 cron 校验返回硬错误/软警告；软警告允许带确认标记后保存。
* [ ] 任务只能选择内置 runner 类型，不能直接保存未分类的任意命令。
* [ ] runner 配置保存前经过结构化校验。
* [ ] TypeScript runner 明确以 `tsx` 为默认执行器。
* [ ] 守护进程能在后台按计划触发任务。
* [ ] API/CLI 可以手动触发任务。
* [ ] 同一任务重叠触发时默认跳过，并记录 `skipped` 执行历史。
* [ ] 任务超时后终止进程，并记录 `timeout` 状态。
* [ ] API/CLI 可以取消正在运行的任务，并记录 `cancelled` 执行历史。
* [ ] 执行历史保存状态、退出码、耗时、错误摘要和截断 stdout/stderr。
* [ ] Web/Desktop 共用的基础管理 UI 支持任务 CRUD、启停、触发、取消和历史查看。
* [ ] 调度核心不依赖 Redis、Asynq 或外部分布式队列服务。
* [ ] PRD 完成后拆分为可独立实施的 Trellis 子任务。

## Definition of Done

* TDD first for business logic: write or update failing tests before implementing scheduler, runner, data, auth, API, and frontend business behavior.
* Tests added/updated (unit/integration where appropriate)
* Lint / typecheck / CI green
* Docs/notes updated if behavior changes
* Rollout/rollback considered if risky

## Testing Constraints

* 业务逻辑采用 TDD：先写失败测试或更新测试，再实现。
* 后端重点测试调度、runner、数据层、认证、API handler/service、配置加载逻辑。
* 前端重点测试业务逻辑、组件交互、表单校验、通用函数、API client。
* PostgreSQL 等数据库集成测试使用 `testcontainers-go`。
* 样式、纯配置文件、纯文档不要求单元测试。
* 前端不测试纯页面结构和 CSS 样式。
* 对配置解析这类有业务分支的代码需要测试；静态配置文件本身不需要测试。

## Technical Approach

### Architecture

采用“pnpm monorepo + Go service 多入口发布”：

* Go service 承载配置、数据库、认证、调度、runner、执行历史和通知事件。
* 根 `package.json` 负责 workspace 聚合脚本；`services/taskdaemon-go/package.json` 负责 Go service 的启动、测试、生成和构建脚本门面。
* Go module 位于 `services/taskdaemon-go`，Go 依赖仍由 `go.mod` 管理，pnpm 只做脚本编排。
* `taskdaemon serve` 只启动后端 API、调度器和静态资源服务。
* `taskdaemon desktop` 或默认桌面入口启动 Wails，并复用同一套前端 UI 和后端核心能力。
* `taskdaemon trigger/cancel/db ...` 等 CLI 子命令不启动桌面窗口。
* 前端开发期使用 `apps/web` 中的 Vite 单独启动；发布期构建产物同步到 `services/taskdaemon-go/web/embedded/dist`，由 Wails/Go embed 打入二进制。

### Backend Stack

* Language/runtime: Go。
* Desktop shell: Wails。
* HTTP API: Gin。
* CLI: Cobra 或等价子命令框架，支持 `serve`、`desktop`、`db migrate`、`task trigger`、`task cancel` 等模式。
* Data modeling: Ent。
* Database: SQLite 默认，PostgreSQL 可配置。
* Scheduler: 优先 spike `gocron`；若验证通过，第一版使用 gocron，否则回退 `robfig/cron/v3`。
* Runner: Go `os/exec` + context timeout/cancel，跨平台进程终止单独封装。
* Logging: Go `log/slog` 起步。
* Config: Koanf，合并配置文件、CLI flag 和环境变量。
* Auth: 单管理员账号，Cookie/session，CSRF/Host 校验；密码哈希方案实施时在 Argon2id/bcrypt 中确认。
* API docs: `swaggo/swag` 生成 Swagger/OpenAPI 文档。
* Testing: `testing` + `testify`，PostgreSQL 等集成测试使用 `testcontainers-go`。
* Quality: `golangci-lint`。

### Frontend Stack

* Package manager/workspace: pnpm monorepo，workspace 覆盖 `apps/*`、`packages/*`、`services/*`。
* App framework: Vite + React + TypeScript。
* Routing: TanStack Router。
* Server state: TanStack Query。
* Styling: Tailwind CSS。
* UI primitives/components: Radix UI + class-variance-authority + tailwind-merge，按 shadcn 风格本地维护组件。
* Icons: lucide-react。
* Forms: React Hook Form。
* Validation: Zod。
* Date/time: dayjs，按需引入插件。
* API client/types: 推荐 OpenAPI/类型生成方向；第一轮可先手写轻量 client，但需要尽快补生成链路。
* Frontend quality: Biome lint/format，`tsc --noEmit` typecheck，lint-staged 参考 `vote-system/lint-staged.config.cjs`。
* Frontend tests: Vitest + Testing Library，重点测试业务逻辑、组件交互、通用函数；不测试纯页面结构/CSS 样式。

### Frontend Design Direction

* 产品界面定位为运维/开发工具型管理台，不做营销式首页。
* 第一屏默认进入任务列表/运行状态，而不是 landing/hero。
* Desktop 与 Web 复用同一套页面；Desktop 主要增强系统通知。
* 使用左侧 sidebar + 顶部状态/操作栏 + 主内容区的 app shell。
* 视觉风格克制、专业、高信息密度，优先可扫描和长期使用舒适度。
* 默认浅色专业管理台；深色模式可作为后续增强。
* 任务列表和执行历史优先使用表格；移动端使用横向滚动或紧凑卡片兜底。
* 图标统一使用 lucide-react，不使用 emoji 图标。
* 代码、cron、日志输出使用等宽字体。

### Package Size Strategy

* 当前技术栈保持 Wails + Go + Vite/React 单二进制方向，不改用 Electron 或前端全栈框架。
* Release 构建使用 strip debug symbols 和 `-trimpath`。
* 第一轮不引入图表库、Monaco Editor、Framer Motion、终端模拟器、重型日期库等高体积依赖；日期处理使用 dayjs。
* `swaggo/swag` 作为文档生成工具；Swagger UI/docs route 通过配置开关启用/关闭，默认可关闭，开发或自托管需要时显式开启。
* `testify`、`testcontainers-go`、`golangci-lint`、Vitest、Biome、Testing Library 仅作为开发/测试/质量工具，不进入运行时路径。
* SQLite driver 优先 pure-Go 方向以降低跨平台构建复杂度，但需在骨架任务中实测体积与 Ent 兼容性。

### Data

* 使用 Ent 定义任务、执行记录、管理员账号、会话/登录态、配置等实体。
* SQLite 作为默认数据库，PostgreSQL 通过配置启用。
* 业务层通过服务/仓储边界访问数据，避免直接依赖方言细节。
* 第一版只负责当前数据库 schema migration，不负责跨数据库数据迁移。

### Scheduler

* 使用进程内调度库，优先 spike `gocron`。
* 支持 cron + manual trigger。
* 不使用 Asynq/Redis；用户定时任务与分布式任务队列属于不同场景。
* `robfig/cron/v3` 更轻、更专注 cron；`gocron` 更贴近任务调度产品，内置 singleton、全局并发限制和 interval 能力。
* 已决定优先做 gocron spike，验证 5/6 字段 cron、timezone、任务更新/删除、singleton skip 与 shutdown。验证通过则使用 gocron，否则回退 `robfig/cron/v3`。

### Runner

* runner 本质仍是外部进程执行，但对用户暴露结构化任务类型。
* 内置 runner 初始覆盖 shell/bash/pwsh/python/node/typescript。
* TypeScript 使用 `tsx`。
* 执行器负责 timeout、cancel、stdout/stderr 截断、退出码和执行状态归档。

## Decision (ADR-lite)

### 数据层选择 Ent

**Context**: 项目需要从第一版支持 SQLite 与 PostgreSQL，并且任务、执行记录、通知、用户、登录态等模型会持续演进。数据层需要减少业务层方言泄漏，同时提升编辑和检查阶段的问题发现能力。

**Decision**: 使用 Ent 作为数据建模与访问基础，并在服务/仓储边界内封装 Ent 细节。

**Consequences**: 项目会引入 Ent schema 与代码生成流程；换来更强的类型约束、跨数据库 schema 管理能力和更好的编辑期检查。复杂 SQL 或数据库特有能力后续集中放在数据层处理，不散落到业务逻辑。

### 不使用 Asynq

**Context**: 用户任务调度是本机/单节点守护进程场景，目标是低占用、单二进制和本地脚本执行。

**Decision**: 第一版采用进程内 cron 调度，不引入 Asynq/Redis。

**Consequences**: 第一版不会具备分布式队列、跨节点 worker、至少一次投递等能力；但能保持部署轻量和用户心智简单。未来出现分布式任务需求时再重新评估 Asynq、NATS、Temporal 等方案。

### Runner 先于插件系统

**Context**: 项目需要支持 shell/pwsh/python/node/typescript 等用户脚本，但第一版开发量已经偏大。

**Decision**: 第一版做内置结构化 runner；插件系统作为后续优化。

**Consequences**: 第一版能覆盖数据库备份和常见脚本任务，同时避免插件协议、插件生命周期、沙箱等复杂度过早进入。

## Out of Scope

* 第一版不实现完整邮件通知。
* 第一版不实现完整 Webhook 平台能力。
* 第一版不实现系统服务安装器，除非后续确认它是 MVP 必需项。
* 第一版不为 Desktop 维护一套独立于 Web 的页面。
* 第一版不实现多用户、多租户或 RBAC。
* 第一版不实现插件系统。
* 第一版不实现完整通知系统，仅保留任务事件/通知扩展点。
* 第一版不实现完整 Web 管理台高级能力；通知中心、完整设置页、备份模板、高级 runner 配置拆到第二轮或后续。
* 第一版不实现分布式任务队列、跨节点 worker 或 Asynq 集成。
* 第一版不实现 SQLite 与 PostgreSQL 之间的一键数据迁移。
* 第一版不保存完整执行日志文件，不做日志轮转或外部日志归档。

## Task Breakdown

### 第一轮：调度核心可用

1. 项目骨架与多入口
   * Go/Wails/Fiber/pnpm monorepo
   * `serve`、`desktop`、CLI 入口
   * 前后端独立启动
   * 发布期前端嵌入二进制

2. 数据层与认证基础
   * Ent schema
   * SQLite/PostgreSQL 配置
   * 当前数据库 schema migration
   * 单管理员初始化/登录
   * 基础登录态、CSRF/Host 安全边界

3. 调度核心与 runner
   * 任务定义
   * 5/6 字段 cron + 手动触发
   * skip-overlap
   * timeout/cancel
   * 执行历史与截断输出
   * shell/bash/pwsh/python/node/typescript runner

4. 基础 Web/Desktop 管理 UI
   * 任务列表
   * 创建/编辑/启停
   * 手动触发/取消
   * 执行历史

### 第二轮或后续

* 通知系统：站内通知、任务完成事件、Desktop 原生通知桥接、通知中心。
* 完整设置页：数据库配置展示、运行配置、日志策略、危险操作确认。
* 备份模板：pgBackRest/pg_dump 任务模板。
* Webhook、邮件通知。
* 系统服务安装器。
* 插件系统。
* SQLite/PostgreSQL export/import/transfer 数据迁移工具。

## Research References

* [`research/wails-dev-release-cli.md`](research/wails-dev-release-cli.md) — Wails 适合作为 Go + Web 前端 + 单二进制发布层，但 CLI 需要通过主程序模式分发设计。
* [`research/multi-database-go-data-layer.md`](research/multi-database-go-data-layer.md) — GORM、Ent、sqlc 都可支持 SQLite/PostgreSQL；已选择 Ent，优先强类型 schema 与生成代码检查。
* [`research/typescript-runner-tsx.md`](research/typescript-runner-tsx.md) — TypeScript runner 推荐默认使用 `tsx`，支持零配置执行 TS 脚本并透传 Node 参数。
* [`research/scheduler-cron.md`](research/scheduler-cron.md) — 调度库对比：`robfig/cron/v3` 更轻，`gocron` 更贴近调度产品能力；实施前需要 spike 后定稿。
* [`research/asynq-fit.md`](research/asynq-fit.md) — Asynq 是 Redis-backed 分布式任务队列；第一版本机低占用调度器不引入。
* [`research/tech-stack-selection.md`](research/tech-stack-selection.md) — 前后端技术栈草案：Go/Wails/Fiber/Ent/cron + Vite/React/TanStack/Tailwind/Radix。
* [`research/frontend-design-direction.md`](research/frontend-design-direction.md) — 前端设计方向：运维/开发工具型管理台，克制、高信息密度、Web/Desktop 同 UI。
* [`research/package-size-analysis.md`](research/package-size-analysis.md) — 打包体积分析：当前栈可保留，重点控制重型前端依赖、Swagger/测试工具运行时隔离、SQLite driver 体积实测。

## Technical Notes

* Task directory: `.trellis/tasks/05-14-cross-platform-scheduler-daemon`
* 已读取：`需求.md`
* 已读取：`C:\home\env\powershellScripts\config\database\backup\pgBackRest\README.md`
* 已读取：`D:\coding\Projects\frontend\vibe-coding\vote-system\package.json`
* 已读取：`D:\coding\Projects\frontend\vibe-coding\vote-system\pnpm-workspace.yaml`
* 已读取：`D:\coding\Projects\frontend\vibe-coding\vote-system\apps\web\package.json`
* 已读取：`D:\coding\Projects\frontend\vibe-coding\vote-system\docs\desktop-app-stack-selection.md`
* 已读取：`.trellis/spec/guides/index.md`
* 已读取：`.trellis/spec/backend/index.md`
* 已读取：`.trellis/spec/frontend/index.md`

## Auth Setup UX Discussion

### What I already know

* 当前后端已有 `POST /api/auth/init`：用于创建首个单管理员账号；重复初始化返回 `409 admin_already_initialized`。
* 当前后端已有 `POST /api/auth/login` 与 `GET /api/auth/me`：登录成功写入 `taskdaemon_session` HttpOnly Cookie，未登录 `/api/auth/me` 返回 `401 unauthorized`。
* 当前前端 `ProtectedApp` 只根据 `/api/auth/me` 是否报错决定显示登录页；它没有区分“已经初始化但未登录”和“系统还没有管理员”。
* 当前前端 `LoginPanel` 默认用户名填 `admin`，但密码为空，用户首次打开时不知道应该输入什么。
* 当前认证契约坚持密码只保存 hash，不应把初始密码明文写入配置文件。

### Requirements (evolving)

* 首次打开系统时，用户不能被要求输入未知的用户名/密码。
* UI 需要能表达“首次初始化管理员”和“已有管理员后登录”两个状态。
* 首次初始化成功后，应自动进入已登录状态，或至少引导用户立即登录。
* 不应提供长期有效的硬编码默认密码。
* 首次使用采用“创建管理员”向导：用户自己设置首个管理员用户名和密码，之后再进入普通登录流程。
* 创建首个管理员成功后，后端直接建立登录态并设置 `taskdaemon_session` Cookie，前端立即进入管理台。

### Feasible Approaches

**Approach A: 首次访问显示“创建管理员”向导（Recommended）**

* How it works: 前端增加初始化模式；后端提供可查询初始化状态的轻量接口，未初始化时显示创建管理员表单，提交到 `/api/auth/init`。
* Pros: 没有默认密码，用户心智清晰，适合未来公网访问边界。
* Cons: 需要补一个初始化状态判断接口或等价响应语义。

**Approach B: 启动日志生成一次性初始密码**

* How it works: 首次启动时生成随机管理员密码并打印到日志或控制台，用户用该密码登录后修改。
* Pros: 自动化部署友好，常见于自托管服务。
* Cons: 本地 Desktop/Web 用户不一定看得到日志；还需要做一次性凭据生命周期与重置路径。

**Approach C: 开发环境默认账号，发布环境禁用**

* How it works: 开发工作区或显式 dev flag 下启用默认 `admin/admin` 或固定密码，发布版必须走初始化。
* Pros: 开发最快。
* Cons: 安全边界容易被误用，也会让产品体验和开发体验分叉。

### Resolved Questions

* 前端通过 `GET /api/auth/status` 判断系统是否尚未初始化，不复用 `/api/auth/me` 的 401 错误语义做首次设置分流。

### Decision (ADR-lite): 首次访问创建管理员

**Context**: 当前前端只在 `/api/auth/me` 失败后显示登录页，但新数据库里没有管理员账号，用户首次打开时不知道应该输入什么。

**Decision**: 首次访问显示“创建管理员”向导。用户自己设置首个管理员用户名和密码；不提供长期有效的默认账号或默认密码。

**Consequences**: 前端需要区分“未初始化”和“未登录”；后端需要提供初始化状态或等价错误语义。该方案比默认账号更安全，也比日志里的一次性密码更适合 Desktop/Web 共用 UI。

### Decision (ADR-lite): 初始化成功即登录

**Context**: 用户刚创建首个管理员账号后，如果还要重新输入同一组账号密码，会造成重复操作，并让首次体验显得不确定。

**Decision**: `POST /api/auth/init` 在成功创建管理员后同时创建 session，设置 `taskdaemon_session` HttpOnly Cookie，并返回与登录接口兼容的 `adminId`、`username`、`csrfToken`。

**Consequences**: 后端初始化路径需要复用登录态创建逻辑，前端初始化 mutation 成功后刷新 `/api/auth/me` 或直接进入管理台。重复初始化仍返回 `409 admin_already_initialized`。

### Decision (ADR-lite): Auth status 作为入口分流

**Context**: `/api/auth/me` 的 401 只能说明当前请求未认证，不能区分“数据库还没有管理员”和“已有管理员但当前浏览器未登录”。

**Decision**: 新增 `GET /api/auth/status`，返回 `initialized`、`authenticated` 和可选 `admin`。未初始化与已初始化未登录都返回 200，前端据此显示“创建管理员”或“登录”。

**Consequences**: 前端 app shell 以 auth status 作为基础 query；`/api/auth/me` 继续保留为需要当前管理员详情时的传统 session endpoint。

### Decision (ADR-lite): 配置示例归属 Go service

**Context**: 配置文件实际由 `services/taskdaemon-go/internal/config` 读取，根目录只负责 pnpm workspace 聚合。如果把示例配置放在仓库根目录，会让它看起来像 workspace 级配置，也容易和开发期自动发现的项目配置混淆。

**Decision**: 默认配置示例放在 `services/taskdaemon-go/taskdaemon.example.yaml`。开发时可复制为 `services/taskdaemon-go/taskdaemon.yaml`，发布时复制到用户配置目录或通过 `--config` 显式指定。

**Consequences**: 示例配置和 Go service 启动目录保持一致；仓库根目录不承担运行配置语义。
