# 基础 Web Desktop 管理 UI

## Goal

实现第一轮共用 Web/Desktop 的基础管理界面，让用户能完成任务 CRUD、启停、手动触发、取消运行中任务和查看执行历史。

## Requirements

* Web 与 Desktop 复用同一套 UI。
* 界面定位为运维/开发工具型管理台，不做营销式首页。
* 第一屏默认进入任务列表/运行状态。
* 使用左侧 sidebar + 顶部状态/操作栏 + 主内容区的 app shell。
* 默认浅色专业管理台；深色模式可后续增强。
* 支持单管理员登录后的任务管理体验。
* 支持任务列表、创建、编辑、启停。
* 支持手动触发任务。
* 支持取消 running 任务。
* 支持执行历史查看，包括状态、退出码、耗时、错误摘要、截断 stdout/stderr。
* cron 输入支持 5/6 字段；秒级/高频表达式需要提示风险并提供确认提交入口。
* runner 配置使用结构化表单，不提供裸任意命令输入框。
* 前端使用 Biome lint/format、`tsc --noEmit` typecheck、lint-staged。
* 前端测试使用 Vitest + Testing Library，重点覆盖业务逻辑、组件交互、通用函数。
* 图标统一使用 lucide-react，不使用 emoji 图标。
* 表格用于任务列表和执行历史；移动端需避免横向溢出破坏布局。
* 日期处理使用 dayjs，按需引入插件。
* 第一轮不引入图表库、Monaco Editor、Framer Motion、终端模拟器或其他重型日期库。

## Acceptance Criteria

* [ ] 登录后可访问任务管理页。
* [ ] 可创建、编辑、启停任务。
* [ ] 可手动触发任务。
* [ ] 可取消 running 任务。
* [ ] 可查看执行历史和截断输出。
* [ ] 秒级/高频 cron 显示提示并支持确认提交。
* [ ] Web 与 Desktop 加载同一套页面。
* [ ] 第一屏是任务列表/运行状态，而不是 landing page。
* [ ] 任务列表和执行历史在桌面端可扫描，在移动端不出现页面级横向滚动。
* [ ] 前端 typecheck 通过。
* [ ] 关键业务逻辑/组件交互有 Vitest/Testing Library 覆盖。

## Testing Constraints

* 前端业务逻辑、组件交互、表单校验、API client 采用 TDD。
* 不测试纯页面结构、CSS 样式、静态配置和文档。
* [ ] 前端依赖保持轻量，未引入第一轮禁止的重型 UI 依赖。

## Out of Scope

* 不实现完整通知中心。
* 不实现完整设置页。
* 不实现备份模板和高级 runner 配置。

## Parent

* `.trellis/tasks/05-14-cross-platform-scheduler-daemon/prd.md`
