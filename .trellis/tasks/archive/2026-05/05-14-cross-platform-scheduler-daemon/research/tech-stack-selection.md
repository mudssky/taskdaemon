# 技术栈选型草案

## 约束

* Go 核心，跨平台、低占用、单二进制。
* Wails Desktop 与 Web 端复用同一套 UI。
* 开发期前端/后端单独启动，发布期前端嵌入二进制。
* 第一轮目标是调度核心可用，不追求完整平台能力。
* 参考 `vote-system` 的 pnpm monorepo、Vite/React/Tailwind/Radix/TanStack 组合。

## 后端推荐栈

* Language/runtime: Go。
* Desktop shell: Wails。
* HTTP API: Gin。
* CLI: Cobra 或等价子命令框架，优先支持 `serve`、`desktop`、`db migrate`、`task trigger`、`task cancel`。
* ORM/data modeling: Ent。
* Database:
  * SQLite 默认。
  * PostgreSQL 可配置。
  * SQLite driver 优先考虑 pure-Go 方案，降低跨平台 cgo 构建成本；实施前需要用 Ent 做兼容性 spike。
  * PostgreSQL driver 优先考虑 pgx。
* Scheduler: `robfig/cron/v3`。
* Process runner: Go `os/exec` + context timeout/cancel；跨平台进程组终止逻辑单独封装。
* Config: Koanf，合并配置文件、环境变量和 CLI flag。
* Logging: Go `log/slog` 起步，避免第一版引入额外日志框架。
* API docs: `swaggo/swag` 生成 Swagger/OpenAPI 文档。
* Testing:
  * `testing` + `testify` 做断言和测试套件。
  * `testcontainers-go` 做 PostgreSQL 等集成测试依赖。
* Quality: `golangci-lint` 作为 Go lint 聚合器。
* Auth:
  * 单管理员账号。
  * 密码哈希优先 Argon2id 或 bcrypt，后续实施时确定。
  * Cookie/session + CSRF/Host 校验，避免公网可演进场景下裸奔。

## 前端推荐栈

* Package manager/workspace: pnpm monorepo。
* App framework: Vite + React + TypeScript。
* Routing: TanStack Router。
* Server state: TanStack Query。
* Styling: Tailwind CSS。
* UI primitives/components: Radix UI + class-variance-authority + tailwind-merge，按 shadcn 风格本地维护组件。
* Icons: lucide-react。
* Forms: React Hook Form。
* Validation: Zod。
* Date/time: dayjs，按需引入插件。
* Testing/quality:
  * Biome 负责前端 lint/format。
  * TypeScript `tsc --noEmit` 负责 typecheck。
  * lint-staged 参考 `vote-system/lint-staged.config.cjs`：
    * JS/TS/JSON: `biome check --write --changed --no-errors-on-unmatched --files-ignore-unknown=true`
    * CSS/SCSS: `biome format --write --changed --no-errors-on-unmatched`
    * TS/TSX: `pnpm typecheck`
  * 业务逻辑、组件交互、通用函数测试使用 Vitest + Testing Library；不测试纯页面结构/CSS 样式。
* API client/types:
  * 推荐方向：OpenAPI/类型生成，减少前后端 DTO 漂移。
  * 第一轮可先手写轻量 client，但需要尽快补生成链路。
* Notifications:
  * 第一轮 UI 只保留事件/提示位。
  * Desktop 原生通知桥接进入后续通知任务。

## 为什么不选

* Next.js/Nuxt 等前端全栈框架：桌面壳和单二进制发布场景会引入不必要复杂度。
* Electron：本项目核心是 Go + Wails + 单二进制，Electron 更适合 Node 后端路线。
* Asynq/Redis：用户定时任务不是分布式队列场景，第一版不引入。
* 插件系统：长期有价值，但第一版先做结构化 runner。

## 决策更新

* 用户明确将 HTTP API 框架从 Fiber 改为 Gin。
* 用户明确配置库选择 Koanf。
* 用户明确加入 `testify`、`testcontainers-go`、`swaggo/swag`、`golangci-lint`。
* 前端质量链路参考 `D:\coding\Projects\frontend\vibe-coding\vote-system\lint-staged.config.cjs`，使用 Biome + typecheck + lint-staged。

## 待确认

* Wails 版本：倾向 Wails 当前主线，但 scaffold 子任务需要验证打包、前端嵌入和 CLI 共存。
* SQLite driver：pure-Go 优先，但需要验证 Ent 兼容性和跨平台构建表现。
* API 契约：OpenAPI 生成链路是否第一轮就纳入。
