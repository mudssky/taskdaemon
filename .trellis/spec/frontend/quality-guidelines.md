# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

前端质量链路使用 Biome lint/format、`tsc --noEmit` typecheck、Vitest + Testing Library。测试重点是业务逻辑、组件交互、表单校验、API client 和通用函数；不测试纯页面结构、CSS 样式或静态配置文件。

当前仓库还未创建前端工程，相关约定来自父任务和基础 UI 子任务 PRD。

---

## Forbidden Patterns

* 不创建营销式 landing/hero 作为应用首页；登录后第一屏进入任务列表/运行状态。
* 不使用 emoji 图标；图标统一来自 `lucide-react`。
* 不引入第一轮禁止的重型依赖：图表库、Monaco Editor、Framer Motion、终端模拟器、moment 等重型日期库。
* 不提供裸任意命令输入框；runner 必须是结构化类型和字段。
* 不把服务端状态复制到全局 store。
* 不用快照测试替代业务断言。
* 不测试纯 CSS、静态页面结构或配置文件。

---

## Required Patterns

* 使用 Vite + React + TypeScript。
* 使用 TanStack Router 管理路由。
* 使用 TanStack Query 管理 server state。
* 使用 React Hook Form + Zod 管理表单和校验。
* 使用 Tailwind CSS + Radix UI + cva + tailwind-merge 维护本地组件。
* 使用 dayjs 处理日期时间，并按需引入插件。
* 任务列表和执行历史优先用表格展示，移动端必须避免页面级横向滚动。
* 高频/秒级 cron 需要软警告和显式确认提交流程。

---

## Testing Requirements

* cron 解析辅助函数需要覆盖 5/6 字段、非法字段、timezone、秒级/高频 warning。
* runner 表单需要覆盖不同 runner 类型的必填字段、默认 timeout、TypeScript 默认 `tsx`。
* API client 需要覆盖错误码映射、认证失败、trigger/cancel 等关键动作。
* 组件交互测试覆盖创建/编辑任务、启停、手动触发、取消 running、查看历史。
* 使用 Testing Library 从用户行为角度断言，不依赖 className 或 DOM 层级细节。
* 对纯样式、布局微调、静态配置文件不写测试。

---

## Code Review Checklist

* 页面是否仍是工具型管理台，而不是营销页或低密度展示页。
* 任务状态、执行状态、runner 类型是否使用集中类型/常量。
* 表单硬错误和软警告是否区分清楚。
* mutation 成功后是否刷新了相关 query。
* 是否存在未处理 loading、empty、error、disabled 状态。
* 移动端是否会出现页面级横向滚动。
* 是否新增了第一轮禁止的重型依赖。
* 是否跑过 `biome`、`tsc --noEmit` 和相关 Vitest 测试。

---

## Common Mistakes

* 不要只做静态页面而缺少真实表单状态和提交路径。
* 不要让 Desktop 和 Web 分叉成两套组件。
* 不要把 API 错误 message 当作稳定逻辑分支。
* 不要为了视觉效果牺牲信息密度和可扫描性。
