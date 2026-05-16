# Component Guidelines

> How components are built in this project.

---

## Overview

taskdaemon 前端是运维/开发工具型管理台，组件设计优先清晰、可扫描、高信息密度和长期使用舒适度。Web 与 Desktop 共用同一套页面；Desktop 专属能力通过边界封装，不维护独立 UI。

当前 UI 基础栈：React + TypeScript + shadcn/ui 本地组件模式。Tailwind CSS v4、Radix primitives、class-variance-authority、tailwind-merge 和 `lucide-react` 是长期组件基础；已有全局 CSS class 属于过渡层，后续新增可复用控件优先沉淀到 `components/ui`。

---

## Component Structure

* 页面组件只组合 route、数据 hook 和 feature 组件，不承载复杂业务计算。
* feature 组件按业务意图命名，例如 `TaskForm`、`TaskTable`、`RunHistoryTable`。
* 基础 UI 组件放在 `components/ui`，只表达通用交互能力，例如 Button、Dialog、Input、Table、Tabs；当前没有稳定复用前不要提前抽象。
* 复杂表单拆分为字段组件或 section 组件，但表单状态和提交逻辑保持在 feature 层统一管理。
* 任务列表和执行历史优先使用表格；移动端用横向滚动容器或紧凑卡片兜底，不能造成页面级横向滚动。

---

## Props Conventions

* props 类型就近定义，组件导出时一并导出必要类型。
* props 名称表达业务语义，例如 `task`、`run`、`onTrigger`、`onCancelRun`，避免 `data`、`item` 这种跨层含糊命名。
* 事件回调用 `onXxx` 命名，异步动作返回 `Promise<void>` 时由调用方决定 loading/error 展示。
* 组件不要直接构造 API payload；表单 schema 或 feature action 负责 DTO 映射。
* 可复用 UI 组件通过 variant/size 控制视觉变化，避免复制多个只差 className 的组件。

---

## Styling Patterns

* 新增可复用控件优先使用 shadcn/ui 模式：Radix 原语 + cva variants + `cn` 合并 class。
* `src/styles.css` 保留 Tailwind v4 入口、shadcn theme variables、基础布局和过渡期全局 class；不要继续把可复用控件逻辑堆进全局 CSS。
* 重复色值、间距、radius、字号等样式值应优先沉淀为 CSS variables 或 Tailwind theme token。
* 默认浅色专业管理台；深色模式需要整体设计后再做，不要为单个组件手写未验证的双主题样式。
* 视觉风格克制，不做营销式 hero、装饰性大卡片或低信息密度布局。
* 代码、cron 表达式、日志输出使用等宽字体。
* 不为单个页面引入重型 UI 依赖：图表库、Monaco Editor、Framer Motion、终端模拟器、重型日期库。
* 不在单个页面临时创造第二套 variant/helper；通用控件变体集中在 `components/ui`。

---

## Icons and Controls

* 图标统一来自 `lucide-react`，不使用 emoji 图标。
* 工具按钮优先使用图标加 tooltip；关键破坏性操作保留清晰文字。
* 二元设置使用 switch/checkbox；模式选择使用 segmented control、tabs 或 select。
* 创建、保存、触发、取消等命令需要 loading/disabled 状态，避免重复提交。
* 删除、取消 running 任务、高频 cron 确认等风险操作使用确认对话框或明确确认状态。

---

## Accessibility

* 表单字段必须有 label、错误信息和可聚焦控件。
* Dialog、Popover、Tooltip 等交互需要保留键盘导航和焦点管理；若未引入可访问原语库，自建实现必须补齐 focus/escape/aria 行为。
* 状态不能只靠颜色表达；执行状态 badge 需要文本。
* 表格操作按钮需要可访问名称，图标按钮提供 `aria-label` 或 tooltip。
* 日志/输出区域支持键盘选中复制，不把文本渲染成不可选择图像。

---

## Common Mistakes

* 不要创建 landing page 作为第一屏；用户登录后应直接进入任务列表/运行状态。
* 不要为 Desktop 复制一套页面；Desktop 与 Web 共用 UI。
* 不要用裸任意命令输入框替代结构化 runner 表单。
* 不要让表格在移动端撑出页面级横向滚动。
* 不要为了单个页面引入重型依赖。
