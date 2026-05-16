# 修复前端 Sidebar 路由切换

## Goal

修复前端左侧 sidebar 点击后页面内容没有按路由切换的问题，避免任务列表、创建任务、执行历史等多个界面都堆在首页。目标是在现有 TanStack Router 基础上整理 route component，让每个导航入口只渲染对应页面。

## What I Already Know

* 用户反馈 sidebar 点击没生效，好几个界面堆在首页。
* 项目已经依赖并使用 `@tanstack/react-router`。
* `apps/web/src/routes/router.tsx` 里 `/`、`/tasks`、`/runs` 当前都渲染 `TaskDashboard`。
* `TaskDashboard` 同时包含任务列表、任务表单和执行历史，因此多个导航入口看起来像同一个页面。
* `AppShell` 已经使用 TanStack Router 的 `Link` 和 `Outlet`，可以继续作为布局壳。

## Research References

* [`research/tanstack-router-layout.md`](research/tanstack-router-layout.md) — TanStack Router v1 推荐 root layout + child route + Outlet，项目应沿用现有路由库并拆分页面组件。

## Assumptions

* 不引入新的路由库。
* 本任务聚焦路由与页面内容分离，不做大规模视觉重设计。
* 认证分流继续保留在当前 protected layout 逻辑中。

## Requirements (Evolving)

* sidebar 的 `/`、`/tasks`、`/runs` 点击必须切换 URL 与页面内容。
* `/` 做成轻量但有用的运行状态概览页，展示任务总数、启用数、运行中数、最近执行摘要等可从现有接口推导的信息。
* `/tasks` 只展示任务管理相关内容。
* `/runs` 只展示执行历史相关内容。
* `/` 不应继续堆叠完整任务管理和执行历史界面。
* 导航 active 状态应与当前 route 一致。

## Acceptance Criteria (Evolving)

* [ ] 点击“运行状态”后只显示运行状态页内容。
* [ ] 运行状态页展示基于现有任务/执行历史接口聚合出来的概览信息，并覆盖 loading、empty、error 基础状态。
* [ ] 点击“任务”后只显示任务列表/创建编辑任务相关内容。
* [ ] 点击“执行历史”后只显示执行历史相关内容。
* [ ] TanStack Router route tree 清晰表达 root layout 与子页面。
* [ ] 添加或更新前端测试覆盖导航切换关键行为。

## Definition of Done

* Tests added/updated for routing/page behavior where appropriate.
* Typecheck passes.
* Lint status documented; existing CRLF issue不在本任务内大面积处理，除非用户确认。
* Docs/spec updated if route organization convention changes.

## Out of Scope

* 不更换 React Router、TanStack Router 之外的新路由库。
* 不做完整 dashboard 信息架构重设计，不新增图表库或后端统计接口。
* 不处理 Biome 对全量前端 CRLF 的既有格式失败。

## Technical Notes

* 关键文件：`apps/web/src/routes/router.tsx`、`apps/web/src/components/layout/AppShell.tsx`、`apps/web/src/features/tasks/TaskDashboard.tsx`、`apps/web/src/features/runs/RunHistoryTable.tsx`。
* 当前 `TaskDashboard` 可拆成任务管理页和执行历史页，避免 route 共享同一大组件。
* 用户确认本次顺手把运行状态页做成稍微像样的概览页，但不扩大到全新后端统计 API。
