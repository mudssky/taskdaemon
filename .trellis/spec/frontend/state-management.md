# State Management

> How state is managed in this project.

---

## Overview

taskdaemon 前端的状态分为 server state、URL state、form state、local UI state 和少量 app state。默认不要引入额外全局状态库；当前使用 TanStack Query、TanStack Router、React Hook Form 和 React 本地 state 覆盖主要需求。

---

## State Categories

* Server state：任务、执行历史、登录态、配置摘要等来自后端的数据。使用 TanStack Query。
* URL state：筛选、分页、选中 tab、详情页 ID。使用 TanStack Router search params。
* Form state：创建/编辑任务、登录、runner 配置、cron 确认。使用 React Hook Form。
* Local UI state：dialog 开关、当前展开行、临时 hover/focus。使用组件本地 `useState`。
* App state：主题、sidebar 折叠、Desktop/Web 能力探测等少量跨页面 UI 状态。优先用 React context，确认复杂后再评估 store。

---

## When to Use Global State

只有满足以下条件才引入全局状态：

* 多个远距离组件需要读写同一份非服务端状态。
* 状态不能自然表达为 URL search params。
* 状态不是表单内部临时值。
* 使用 context 已经明显造成不必要重渲染或复杂传递。

不要为了任务列表、执行历史或登录态引入 Zustand/Redux。它们分别由 TanStack Query 和路由保护处理。

---

## Server State

* TanStack Query 是服务端数据唯一缓存来源。
* query key 必须稳定，并按 feature 集中定义。
* mutation 成功后 invalidate、remove 或更新相关列表和详情；例如触发任务后刷新任务状态和执行历史。
* running 任务状态可以短轮询，但必须根据页面可见性、是否存在 running 项、用户所在页面控制频率。
* API 返回错误码后，组件根据错误类型展示字段错误、toast 或确认对话框。
* 具体 query key 和缓存失效规则见 [API 与 Query 契约](./api-query-contracts.md)。

---

## URL State

* 列表筛选、分页、执行历史 task id、状态过滤放进 URL，便于刷新和分享。
* dialog 是否打开通常不放 URL，除非它代表可独立访问的详情状态。
* URL search params 需要类型校验和默认值，避免非法 URL 让页面崩溃。

---

## Derived State

* 派生值优先在 render 中通过 memo 或纯函数计算，不复制成另一份 state。
* 任务状态、执行状态 badge、耗时格式化等放在纯函数中并测试。
* cron 风险等级由 cron 字段和确认状态推导，不手动维护多个容易不同步的布尔值。

---

## Common Mistakes

* 不要把 TanStack Query 的数据复制到 `useState` 后再编辑原对象；编辑表单应创建默认值快照。
* 不要用全局 store 保存服务端列表。
* 不要让轮询在没有 running 任务时继续高频请求。
* 不要把表单 dirty/valid 状态提升到全局。
