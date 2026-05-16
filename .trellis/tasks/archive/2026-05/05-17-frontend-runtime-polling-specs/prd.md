# 前端运行时与轮询性能规范

## Goal

补齐前端轮询、刷新、运行时性能和 query 副作用边界规范，避免后续运行状态、执行历史和任务列表页面扩展后出现重复请求、常驻高频轮询或加载状态互相干扰。

## Requirements

* 规范何时允许自动轮询，何时应停止轮询。
* 规范轮询触发条件：running task/run、页面可见性、页面范围、数据为空等。
* 规范轮询间隔常量化，避免多个文件写死同类毫秒值。
* 规范手动刷新与自动 refetch 的 UI 状态边界。
* 规范 TanStack Query hook 中允许的轻量派生状态和禁止塞入的 UI 副作用。
* 规范同一业务数据在状态页、任务页、历史页之间的缓存复用和失效策略补充。
* 将稳定规则写入 `.trellis/spec/frontend`，优先更新 `hook-guidelines.md`、`state-management.md`、`api-query-contracts.md`。

## Acceptance Criteria

* [x] `.trellis/spec/frontend` 包含轮询和刷新长期规则。
* [x] 明确 query hook、mutation hook 和组件展示之间的职责边界。
* [x] 明确轮询间隔和停止条件的治理方式。
* [x] 明确手动刷新和自动刷新状态展示原则。
* [x] 更新 `index.md` 的开发前检查项。

## Definition of Done

* 文档主要内容使用中文。
* 规范应能指导未来新增运行状态页面、历史页面或实时任务视图。
* 如果只改 spec/Markdown，不需要跑前端测试。

## Out of Scope

* 不重构当前 `useTasksQuery` 或现有 query hook。
* 不引入 WebSocket、SSE 或后台同步机制。
* 不做性能压测或 bundle 优化。

## Technical Notes

* 来源 backlog：归档 PRD 中的“轮询、刷新与性能规范”“Query、mutation 与缓存失效规范”。
* 相关现有规范：`.trellis/spec/frontend/hook-guidelines.md`、`.trellis/spec/frontend/state-management.md`、`.trellis/spec/frontend/api-query-contracts.md`。
* 相关代码示例：`apps/web/src/features/tasks/tasks.queries.ts`、状态页和执行历史页 query 使用。
