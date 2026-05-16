# brainstorm: 代码规范整理与 Go 大文件拆分

## Goal

从现有代码与已归档任务中提炼长期有效的代码规范，更新 Trellis spec 的边界与内容；同时讨论 Go 后端过大文件的拆分原则，提升后续维护性。规范应沉淀稳定工程约定，避免把一次性需求、阶段性产品范围或具体任务验收清单塞进 `.trellis/spec/`。

## What I already know

* 用户希望“从现有代码去总结”代码规范，而不是只延续早期 PRD 里的设想。
* 用户特别指出 Go 侧存在过大代码文件拆分问题，需要先讨论可维护性原则。
* 用户希望区分长期规范与需求类内容：有些需求类内容不要放到规范 spec 里。
* 当前仓库是 pnpm workspace：前端在 `apps/web`，Go 后端在 `services/taskdaemon-go`。
* `.trellis/spec/backend` 与 `.trellis/spec/frontend` 已存在多份规范文档，部分文档包含 Scenario、Contracts、Tests Required 等任务型内容。
* Go 非生成代码中当前最大文件集中在少数包：`internal/scheduler/service.go` 约 950 行、`internal/config/config.go` 约 662 行、`internal/logging/logger.go` 约 601 行、`internal/httpapi/middleware.go` 约 476 行、`internal/cli/command.go` 约 431 行、`internal/app/app.go` 约 397 行。
* 前端当前最大源码文件约 212 行，主要文件分布在 route test、status page、task form/table、API client/schema/query；暂未表现出明显“大文件优先拆分”压力。
* 前端真实代码已经存在并包含任务、运行历史、认证、状态页等功能，`.trellis/spec/frontend/index.md` 仍写“frontend app has not been scaffolded yet”，已过期。
* 后端 spec 中的 `Scenario: Scheduler/Runner Core`、`Scenario: Data/Auth Foundation`、`HTTP Body Logging Contract` 等段落包含大量接口签名、错误矩阵、测试清单和 Good/Base/Bad case，更像历史任务契约或特定功能规格，需要判断是否压缩为长期规范或迁移。

## Assumptions (temporary)

* 本次优先产出规范整理方案与待改清单；是否直接重写 spec 文档，需要用户确认。
* “需求类内容”更适合保留在 `.trellis/tasks/**/prd.md`、研究笔记或专门 ADR/决策文档中，而非放入长期 coding spec。
* Go 大文件拆分应先建立判断标准，再决定是否立刻拆现有文件，避免为了行数而机械拆分。

## Open Questions

* 暂无阻塞问题。

## Requirements (evolving)

* 规范整理必须以现有代码事实为主要来源。
* 规范应保留长期、可重复执行的工程约定。
* 一次性需求、具体接口清单、任务验收矩阵应从长期 spec 中剥离或迁移到更合适的位置。
* Go 文件拆分要服务于领域边界、测试可读性和变更隔离，而不是只按固定行数切文件。
* 生成代码（例如 `services/taskdaemon-go/internal/data/ent`）不纳入大文件拆分判断。
* 过期时态说明（如“尚未 scaffold”“未来 implementation”）需要改成当前事实。
* 直接修改 `.trellis/spec/`，不只停留在方案层。

## Acceptance Criteria (evolving)

* [x] 识别现有 spec 中偏需求/任务型的内容类型。
* [x] 从前端现有代码总结稳定目录、组件、数据请求、表单、测试约定。
* [x] 从 Go 现有代码总结稳定包边界、handler/service/store/test 约定。
* [x] 给出 Go 大文件拆分原则与现有候选文件清单。
* [x] 明确哪些内容进入 `.trellis/spec/`，哪些内容留在任务 PRD/研究/ADR。

## Definition of Done (team quality bar)

* 规范改动前有清晰范围确认。
* 若修改 spec，文档主要内容使用中文。
* 若修改代码，按相关包规范跑 lint/typecheck/tests。
* 不把尚未实现或仅属于单个需求的内容写成长效规范。

## Out of Scope (explicit)

* 暂不默认改业务实现代码。
* 暂不默认拆分 Go 文件，除非 brainstorm 收敛后确认进入实施。
* 不把每个历史 PRD 的验收标准完整搬入 spec。

## Decision (ADR-lite)

**Context**: 现有 `.trellis/spec` 在项目早期承载了大量 PRD、接口清单、错误矩阵和测试验收内容。随着代码已经落地，长期规范需要回到“可重复执行的工程约定”，并以现有代码为准。

**Decision**: 直接重写 `.trellis/spec/` 中明显过期或需求型内容，保留长期代码规范、目录边界、测试原则、错误/日志/数据边界和前端工程约定；同时新增 Go 大文件拆分原则。

**Consequences**: spec 会更短、更稳定，未来任务仍需回查对应 PRD/研究笔记获取一次性需求细节；具体业务接口全集不再作为长期规范主体。

## Technical Notes

* 已创建任务目录：`.trellis/tasks/05-16-code-spec-go-file-splitting`。
* 已查看 `.trellis/spec/backend/index.md`、`.trellis/spec/frontend/index.md`、前后端 directory/quality guidelines。
* 初步观察：backend/frontend quality 与 directory spec 内包含较多 Scenario、Contracts、Validation Matrix、Tests Required、Good/Base/Bad Cases，这些可能更接近任务契约或 PRD，需要判断是否仍应作为长期规范保留。
* Go 函数分布初判：
  * `scheduler/service.go` 混合 cron 校验、任务 CRUD、调度注册、运行状态、执行历史、runner JSON 编解码和小型转换工具；可按 validation / task CRUD / cron registration / run lifecycle / runner config codec 拆分。
  * `config/config.go` 混合配置结构、默认值、路径解析、加载/合并、env 映射和 scalar 解析；可按 types/defaults / paths / loader / env / parsing 拆分。
  * `logging/logger.go` 混合 logger 工厂、file handler、fanout handler、pretty handler、HTTP 访问日志格式化和值格式化；可按 logger / fanout / pretty / format 分拆。
  * `httpapi/middleware.go` 混合 trace、response options、request logger、body capture/restore、脱敏、recovery、auth session；可按 trace_response / request_logger / body_capture / redaction / auth_middleware 拆分。
  * `cli/command.go` 混合 Cobra 根命令构建、config 输出、daemon API task actions、YAML/JSON 输出和 token context；可按 root/config/task_client/output 拆分。
  * `app/app.go` 混合应用装配、HTTP server、配置重载、daemon HTTP client、migration 和地址工具；可按 serve / reload / daemon_client / migrate 拆分。
* 当前质量命令：根 `pnpm test` 会运行前端 typecheck、lint 和 Go test；Go service 有 `test`、`test:integration`、`generate`、`vet` 脚本；前端有 `typecheck`、`lint`、`test`。
* 已直接重写 `.trellis/spec/backend/index.md`、`quality-guidelines.md`、`database-guidelines.md`、`error-handling.md`、`logging-guidelines.md`，并新增 `file-organization.md`。
* 已重写 `.trellis/spec/frontend/index.md`、`directory-structure.md`，并更新 hook/state/type/quality/component 规范中的过期时态。
* 已移除长期 spec 中的历史 7 段式 Scenario 段落，把完整接口清单、验收矩阵和 Good/Base/Bad case 从规范主体中剥离。
