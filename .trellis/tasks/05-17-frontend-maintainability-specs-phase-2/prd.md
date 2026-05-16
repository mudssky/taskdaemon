# 前端长期维护性规范第二批落地

## Goal

承接已归档任务 `.trellis/tasks/archive/2026-05/05-16-frontend-maintainability-specs/prd.md` 中尚未落地的 Candidate Spec Backlog，按主题拆成多个小任务逐步写入 `.trellis/spec/frontend`。目标是继续补齐前端长期维护规范，而不是把所有 backlog 一次性塞进一个大文档。

## What I already know

* 第一批已落地：技术栈/spec 与真实工程对齐、前端文件组织与拆分、API/query 契约、shadcn 基础设施。
* 仍未系统落地的规范包括：轮询性能、表单校验、交互状态、错误恢复、设计系统、可访问性、路由认证、测试分层、依赖治理、Web/Desktop 边界、文案和观测调试。
* 用户确认“分批分多个任务”，说明第二批不应再作为单个大任务推进。
* 本轮主要是文档/spec 工作，原则上不改业务代码；若发现当前代码与规范严重漂移，另开实现任务处理。

## Requirements

* 从归档 PRD 的 Candidate Spec Backlog 拆分多个可独立推进的规范任务。
* 每个子任务只负责一组相近主题，避免互相阻塞。
* 每个子任务 PRD 记录目标、范围、验收标准和明确不做什么。
* 后续实现时，规范内容必须进入 `.trellis/spec/frontend`，而不是只留在任务 PRD。
* 不把一次性页面需求、临时控件清单或历史 PRD 全量复制进长期规范。

## Acceptance Criteria

* [x] 创建第二批父任务。
* [x] 创建多个主题明确的子任务。
* [x] 父任务 PRD 记录拆分策略和任务顺序。
* [x] 子任务 PRD 记录各自范围和验收标准。
* [ ] 每个子任务后续完成时，对应 `.trellis/spec/frontend` 文档已更新并提交。
* [ ] 所有子任务完成后，父任务可归档。

## Subtasks

1. `.trellis/tasks/05-17-frontend-interaction-error-specs`
   * 交互状态、错误恢复、可访问性、文案和观测调试。
2. `.trellis/tasks/05-17-frontend-form-test-specs`
   * 表单校验、软警告、测试分层。
3. `.trellis/tasks/05-17-frontend-runtime-polling-specs`
   * 轮询、刷新、运行时性能和 query 副作用边界。
4. `.trellis/tasks/05-17-frontend-routing-desktop-specs`
   * 路由、认证守卫、Web/Desktop 共用边界。
5. `.trellis/tasks/05-17-frontend-design-dependency-specs`
   * UI/design system、样式增长边界、依赖治理。

## Recommended Order

1. `frontend-interaction-error-specs`：最直接影响后续页面体验和组件一致性。
2. `frontend-form-test-specs`：与当前任务表单、认证表单和测试策略关联紧密。
3. `frontend-runtime-polling-specs`：避免后续运行状态/历史页面扩展时出现请求风暴。
4. `frontend-routing-desktop-specs`：适合在后续 Desktop 能力继续扩展前补齐。
5. `frontend-design-dependency-specs`：范围较大，适合作为最后一批整合和收口。

## Definition of Done

* 父任务只在所有子任务完成并归档后归档。
* 每个子任务完成前需要运行适合文档变更的检查；如果只改 Markdown/spec，至少确认链接路径和索引同步。
* 如规范改动涉及当前实现中的明显违例，记录为后续实现任务，不在规范任务里顺手重构业务代码。

## Out of Scope

* 不重新打开已归档的第一批任务目录。
* 不一次性修改全部前端规范。
* 不默认改业务 UI、API 或测试代码。
* 不把 backlog 中所有候选都写成强制规则；只有稳定、可复用、能指导未来维护的内容进入 spec。

## Technical Notes

* 来源 PRD：`.trellis/tasks/archive/2026-05/05-16-frontend-maintainability-specs/prd.md`。
* 当前前端规范目录：`.trellis/spec/frontend`。
* 当前技术栈：Vite + React + TypeScript、Tailwind CSS v4、shadcn/ui 本地组件模式、TanStack Router、TanStack Query、React Hook Form、Zod、Biome、Vitest + Testing Library。

## Decision (ADR-lite)

### Decision: 第二批按主题拆成 5 个子任务

**Context**: 第一批只落地了最基础的工程边界；归档 PRD 中仍有大量规范 backlog。如果继续放在一个任务里，会难以评审、难以验收，也容易把长期规则和一次性讨论混在一起。

**Decision**: 创建一个第二批父任务，并按交互错误、表单测试、运行时轮询、路由桌面、设计依赖五组拆分子任务。

**Consequences**: 每批可以独立实现、提交和归档；父任务作为追踪容器，等子任务全部完成后再归档。
