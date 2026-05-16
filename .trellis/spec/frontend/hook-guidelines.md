# Hook Guidelines

> How hooks are used in this project.

---

## Overview

前端 hook 用于封装状态ful 逻辑、server state 和业务动作。项目使用 TanStack Query 管理服务端数据，React Hook Form 管理表单状态，TanStack Router 管理 URL 和路由状态。自定义 hook 应该让组件更薄，而不是把跨层副作用藏起来。

---

## Custom Hook Patterns

* hook 名称必须以 `use` 开头，并放在对应 feature 内，除非已经被多个 feature 复用。
* 查询 hook 使用业务名词，例如 `useTasksQuery`、`useRunHistoryQuery`。
* mutation/action hook 使用动作名，例如 `useTriggerTaskMutation`、`useCancelRunMutation`。
* hook 返回值保持稳定结构，优先返回 TanStack Query 原始状态加少量业务派生值。
* 只在 hook 内封装确实复用的逻辑；单个组件独有的 `useState` 不需要抽成 hook。

---

## Data Fetching

* server state 统一使用 TanStack Query。
* query key 放在 feature 附近集中管理，例如 `tasksKeys.list(filters)`。
* mutation 成功后显式 invalidate、remove 或更新相关 query cache。
* API client 负责 HTTP 细节，query hook 负责缓存和错误状态，组件负责展示。
* API、query key、mutation 和缓存失效细节遵守 [API 与 Query 契约](./api-query-contracts.md)。
* 登录态/session 查询应作为 app shell 或受保护 route 的基础 query，不在每个页面重复请求；首次初始化分流优先使用 `GET /api/auth/status`，避免把“未初始化”和“已初始化但未登录”都折叠成 `/api/auth/me` 的 401。

---

## Forms

* 表单使用 React Hook Form。
* Zod schema 是表单输入和 API payload 的边界校验来源。
* cron 表达式、timezone、runner 配置这类规则放在 schema 或 feature 工具函数中，并覆盖测试。
* 高频/秒级 cron 属于软警告：hook 或表单状态需要能表达“有 warning、用户已确认、允许提交”。
* 表单提交 hook 不直接显示 toast；由页面或调用组件决定反馈方式。

---

## Side Effects

* 副作用尽量放在事件处理、mutation lifecycle 或 route loader 中，不在 render 路径触发。
* interval/polling 必须有清理和启停条件；执行历史或 running 状态轮询不能无条件常驻高频刷新。
* 轮询条件优先集中在 query hook；组件不要散落 `setInterval`。
* Desktop 专属能力通过边界 hook 封装，并提供 Web fallback。
* 不把 localStorage/sessionStorage 访问散落到组件中；需要时集中在 auth/config 相关 hook。

---

## Naming Conventions

* `useXxxQuery`：读取服务端数据。
* `useXxxMutation`：改变服务端数据。
* `useXxxForm`：封装表单默认值、schema、提交准备。
* `useXxxState`：确实跨组件复用的纯前端状态。
* `useDebouncedXxx`、`useIntervalXxx` 等通用 hook 只有在多个调用点出现后才放入 `lib`。

---

## Testing Requirements

* hook 中的业务分支需要 Vitest 覆盖，尤其是 cron warning、runner 表单映射、query invalidation 逻辑。
* 数据获取 hook 测试使用 QueryClient 测试包装，避免共享全局 cache。
* 不为只转发 TanStack Query 的薄 hook 写无价值快照测试。

---

## Common Mistakes

* 不要把 server state 复制进全局 store 再手动同步。
* 不要在 hook 中吞掉错误；保留给调用方展示或上报。
* 不要让 hook 同时负责 API、toast、导航、复杂 UI 状态，必要时拆成 action hook 和页面逻辑。
