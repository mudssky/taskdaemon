# brainstorm: 优化 Go 后端规范

## Goal

梳理当前 Go 后端规范相对前端规范优化后的差距，结合现有 Go 代码、测试、构建配置与 Trellis 规范体系，找出值得补充或收紧的规范项，形成可执行的优化范围与后续落地计划。

## What I already know

* 用户希望延续此前“前端规范优化”的思路，这次重点查看 Go 后端规范有哪些需要优化的地方。
* 当前项目已有 `.trellis/spec/backend/` 后端规范目录，包含数据库、目录结构、错误处理、文件组织、日志、质量等文档。
* 当前项目也已有较完整的 `.trellis/spec/frontend/` 规范目录，可作为规范粒度和组织方式的参照。
* 前端规范优化采用“先产出 Candidate Spec Backlog，再分批拆成主题任务”的方式；第一批落地技术栈对齐、文件组织与拆分、API/query 契约，第二批再拆交互错误、表单测试、运行时轮询、路由桌面、设计依赖等主题。
* Go 后端规范当前基础边界较清楚，但主题数量明显少于前端：后端索引 6 个文档，前端索引 12 个文档。
* `services/taskdaemon-go` 是 Go module 根，根 workspace 通过 pnpm 脚本编排 Go 命令。
* 后端包包括 `app`、`auth`、`cli`、`config`、`data`、`desktop`、`httpapi`、`logging`、`runner`、`scheduler`。
* 非生成 Go 文件中，`scheduler/service.go`、`config/config.go`、`logging/logger.go`、`httpapi/middleware.go`、`cli/command.go`、`app/app.go` 等已经是大文件；`file-organization.md` 已记录拆分候选。
* 后端已有 API envelope、trace id、稳定错误码、DTO 转换、前端 API client 联动等跨层契约，但缺少独立的后端 API/DTO 契约规范。
* 配置加载、环境变量映射、项目 local 覆盖、运行时 reload 与 CLI/daemon API 路径已经形成复杂长期规则，但目前分散在目录结构、日志和质量规范里。
* runner/scheduler 已形成结构化 runner、timeout/cancel、输出截断、overlap skip、运行态和执行历史边界，但规则混在质量规范中，缺少独立生命周期/runner 规范。
* auth/httpapi 已形成单管理员、session cookie、token hash、CSRF token 返回、受保护 API middleware、敏感信息脱敏等规则，但缺少独立认证安全规范。
* desktop/Wails 与 Web/API 共用边界已有不少长期约定，部分写在 `directory-structure.md`，但后续扩展原生能力时可能需要更明确的平台边界规范。

## Assumptions (temporary)

* 本次先做规范巡检与方案收敛，不直接大规模改 Go 业务代码。
* 优化目标是提升后续 AI/人类开发时的可执行性，而不是写泛泛的 Go 风格指南。

## Open Questions

* 需要确认本次 MVP 是只产出优化建议，还是直接按推荐第一批修改 `.trellis/spec/backend/` 规范文件。

## Requirements (evolving)

* 对比前端规范与后端规范的覆盖范围、颗粒度和可执行性。
* 检查现有 Go 代码中的架构、错误处理、日志、测试、配置、并发与平台适配模式，识别规范缺口。
* 沿用前端规范优化经验：先沉淀完整候选池，再分批落地，避免一次性写成大而泛的 spec。
* 优先补长期复用、跨多个未来任务、能指导 AI 代理判断边界的规范，不复制单个历史 PRD。

## Acceptance Criteria (evolving)

* [x] 列出 Go 后端规范当前已覆盖的主题。
* [x] 列出与现有代码风险相关、值得补充或优化的规范项。
* [x] 给出 2-3 个可选优化范围，并标注推荐选项与取舍。
* [x] 经用户确认后形成明确的后续实施计划。
* [x] 新增 `api-contracts.md`、`configuration-runtime-guidelines.md`、`scheduler-runner-guidelines.md`。
* [x] 同步 `.trellis/spec/backend/index.md` 的规范索引和开发前检查。

## Definition of Done (team quality bar)

* Tests added/updated (unit/integration where appropriate)
* Lint / typecheck / CI green
* Docs/notes updated if behavior changes
* Rollout/rollback considered if risky

## Out of Scope (explicit)

* 本阶段不默认重构业务代码。
* 本阶段不默认引入新的 Go 依赖或框架。

## Technical Notes

* 已检查：`.trellis/spec/backend/index.md`、后端全部 6 个 spec、`.trellis/spec/frontend/index.md`、`.trellis/tasks/archive/2026-05/05-16-frontend-maintainability-specs/prd.md`、`.trellis/tasks/05-17-frontend-maintainability-specs-phase-2/prd.md`。
* 已检查：`services/taskdaemon-go/go.mod`、`package.json`、`services/taskdaemon-go/package.json`、`.golangci.yml`。
* 已抽样检查：`internal/httpapi/*`、`internal/auth/service.go`、`internal/runner/runner.go`、`internal/scheduler/service.go`、`internal/config/config.go`、`internal/app/app.go`、`internal/desktop/desktop.go`。
* 子代理说明：本轮曾派发两个只读 explorer 辅助核对，但均超时未返回可用结论；已关闭。以下结论仅基于主线程已验证文件。

## Current Backend Spec Coverage

* `directory-structure.md`：workspace、Go module、入口、包边界、Wails/Desktop 和依赖兼容。
* `file-organization.md`：Go 大文件职责拆分、当前拆分候选、拆分流程和评审清单。
* `database-guidelines.md`：Ent、SQLite/PostgreSQL、migration、事务、认证数据和数据层禁止模式。
* `error-handling.md`：Go error、typed/sentinel error、API envelope、错误码、CLI 错误和敏感信息边界。
* `quality-guidelines.md`：调度/runner 行为、HTTP API/CLI 质量规则、测试要求和质量命令。
* `logging-guidelines.md`：slog、结构化字段、HTTP body 日志、脱敏、运行时日志配置和测试要求。

## Candidate Spec Backlog

### 1. 后端 API / DTO / 前后端契约规范

* 现状：API envelope 与错误码在 `internal/httpapi/responses.go`、`task_routes.go`、`auth_routes.go` 中稳定存在，前端 `apps/web/src/lib/api/client.ts` 与 `types.ts` 依赖这些契约。
* 缺口：后端只有错误处理文档提到 envelope，没有独立说明 DTO 命名、handler/service 边界、错误码新增流程、traceId、分页/limit、前后端联动清单。
* 建议：新增 `api-contracts.md` 或扩展错误处理为 API 契约文档，明确新增 API 时必须同步后端 DTO、前端类型/API client/query hook、测试与错误码。

### 2. 配置、CLI 与运行时 reload 规范

* 现状：`internal/config/config.go` 负责 defaults、项目配置、local 覆盖、env 映射、显式 config、运行时可热更新字段；`internal/app` 与 `internal/cli` 负责 daemon reload 路径。
* 缺口：长期规则分散在目录结构、日志和质量规范中；缺少“配置键新增、env 命名、reload 可热更新与需重启边界、CLI 输出脱敏、session token 传递”的集中规范。
* 建议：新增 `configuration-runtime-guidelines.md`。

### 3. Scheduler / Runner 生命周期规范

* 现状：`internal/scheduler/service.go` 和 `internal/runner/runner.go` 已实现 cron 校验、软警告、overlap skip、run lifecycle、timeout/cancel、输出截断和状态映射。
* 缺口：`quality-guidelines.md` 有规则，但还不够像“业务状态机契约”；新增任务时容易把进程内 running、数据库 run history、gocron 行为和 runner result 混在一起。
* 建议：新增 `scheduler-runner-guidelines.md`，重点写生命周期状态、context/cancel、输出限制、结构化 runner、测试矩阵和禁止模式。

### 4. 认证、安全与敏感信息规范

* 现状：`internal/auth/service.go` 维护单管理员、bcrypt、session token hash、CSRF token hash；`internal/httpapi/auth_routes.go` 写 Cookie；日志/错误规范已有脱敏要求。
* 缺口：认证安全规则散落在数据库、错误、日志规范中；缺少 Cookie 属性、session 生命周期、CSRF 使用边界、Host/Origin 后续策略、测试替身和敏感字段统一清单。
* 建议：新增 `auth-security-guidelines.md`，同时引用数据/日志/错误规范。

### 5. App / Daemon / Desktop 平台生命周期规范

* 现状：`internal/app/app.go` 装配 HTTP server、migration、daemon API client、reload；`internal/desktop/desktop.go` 启动 Wails、等待 API ready、处理关闭。
* 缺口：Wails/Desktop 细节目前主要塞在目录结构；后续新增 desktop 原生能力、Web fallback、API ready、shutdown timeout 时缺少集中边界。
* 建议：新增 `daemon-desktop-lifecycle.md` 或在第二批处理。

### 6. 测试分层与测试辅助规范

* 现状：后端测试覆盖较广，包含 package-level 单元测试、HTTP handler fake service、PostgreSQL integration、gocron spike、跨平台 shell helper。
* 缺口：`quality-guidelines.md` 已有测试要求，但没有系统说明 fake service 放置、integration tag、跨平台命令 helper、timer/fake clock、HTTP envelope 解包辅助、生成代码例外。
* 建议：可以先增强 `quality-guidelines.md`；若后续测试增长，再拆 `testing-guidelines.md`。

### 7. 依赖与生成代码治理规范

* 现状：`directory-structure.md` 记录 go.mod 关键依赖和 Ent/Wails 版本兼容；`.golangci.yml` 已启用 govet/staticcheck/ineffassign/unused/gofmt。
* 缺口：新增 Go 依赖准入、间接依赖膨胀、Go tool 依赖、生成代码提交边界、`go mod tidy`/`go generate` 时机还可更明确。
* 建议：短期并入目录结构或质量规范；不建议第一批单独成文。

## Feasible Approaches

### Approach A: 第一批只补最高频三件事（Recommended）

* 范围：新增/强化 `api-contracts.md`、`configuration-runtime-guidelines.md`、`scheduler-runner-guidelines.md`，并同步后端 `index.md`。
* 优点：覆盖当前最容易跨层漂移、最容易被后续任务反复触碰的 Go 规则；范围类似前端第一批，利于快速落地。
* 缺点：认证安全、Desktop 生命周期、测试分层会进入 backlog，需第二批继续。

### Approach B: 完整 backlog 分批任务化

* 范围：本任务只确认 backlog，再创建父任务和 5-6 个子任务，逐批写入 `.trellis/spec/backend`。
* 优点：组织最稳，和前端第二批方式一致，适合后端长期规范系统化。
* 缺点：前置流程更重，本轮不会立刻改善 spec 可读性。

### Approach C: 不新增文件，只增强现有 6 个 spec

* 范围：把 API、配置、runner、auth、desktop、测试规则分别塞回现有目录结构/错误/质量/日志/数据库文档。
* 优点：文件数少，导航简单。
* 缺点：现有文档会继续变胖，主题边界不如前端规范清楚，后续 AI 读取时更容易漏上下文。

## Recommended First Batch

* `api-contracts.md`：HTTP handler、DTO、envelope、错误码、traceId、前后端联动和测试要求。
* `configuration-runtime-guidelines.md`：配置加载顺序、env 映射、local 覆盖、运行时 reload、CLI/daemon API 和脱敏输出。
* `scheduler-runner-guidelines.md`：cron 校验、软警告、run lifecycle、running 视图、overlap、timeout/cancel、输出截断和 runner 配置 JSON。

## Decision (ADR-lite)

### Decision: 第一批直接落地 3 个后端 spec

**Context**: 后端现有 6 个 spec 覆盖基础工程边界，但 API 契约、配置/runtime 和 scheduler/runner 生命周期已经在代码中形成复杂长期规则。若继续散落在错误、目录、质量和日志文档中，后续跨层变更和 AI 上下文选择容易漏读关键规则。

**Decision**: 用户选择 Approach A。第一批直接新增 3 个后端规范文档：`api-contracts.md`、`configuration-runtime-guidelines.md`、`scheduler-runner-guidelines.md`，并更新后端 spec 索引。认证安全、Desktop 生命周期、测试分层、依赖治理保留为后续 backlog。

**Consequences**: 本轮能快速改善最高频后端开发上下文；spec 文件数增加但主题边界更清晰。后续如果认证或 Desktop 能力继续扩展，再按同样方式拆第二批任务。
