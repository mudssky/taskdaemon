# 前端交互状态与错误恢复规范

## Goal

补齐前端交互状态、错误恢复、可访问性、文案和观测调试相关长期规范，让后续页面在 loading、empty、error、disabled、pending、破坏性确认和 traceId 展示上保持一致。

## Requirements

* 从当前代码和归档 PRD 中提炼交互状态与错误恢复规则。
* 明确 loading、empty、error、disabled、pending、success feedback 的职责边界。
* 规范破坏性操作确认：何时必须使用 `AlertDialog`/Confirm，何时可使用普通按钮。
* 规范错误文案来源：字段错误、页面错误、mutation 错误、API code/message 的使用边界。
* 补充可访问性规则：图标按钮可访问名称、Dialog 标题/描述、状态不能只靠颜色、表单错误关联。
* 补充观测与调试规则：traceId 展示、复制和日志敏感信息边界。
* 将稳定规则写入 `.trellis/spec/frontend`，必要时新增独立规范文件或更新现有 `component-guidelines.md` / `quality-guidelines.md`。

## Acceptance Criteria

* [x] `.trellis/spec/frontend` 包含交互状态和错误恢复的长期规则。
* [x] 明确页面级错误和字段级错误的处理边界。
* [x] 明确破坏性操作确认和取消路径要求。
* [x] 明确前端可访问性基础要求。
* [x] 明确 traceId / 调试日志的展示和隐私边界。
* [x] 更新 `index.md` 的规范索引或开发前检查项。

## Definition of Done

* 文档主要内容使用中文。
* 只沉淀长期规则，不描述单个页面当前控件清单。
* 如只改 spec/Markdown，不需要新增业务测试；但需要确认文档索引路径正确。

## Out of Scope

* 不实现新的 Toast、Alert、ErrorBoundary 或日志系统。
* 不重构现有页面错误展示。
* 不补充完整 WCAG 审计清单，只写当前项目可执行的基础约定。

## Technical Notes

* 来源 backlog：归档 PRD 中的“交互状态与错误恢复规范”“可访问性规范”“国际化与文案规范”“观测与调试规范”。
* 相关现有规范：`.trellis/spec/frontend/component-guidelines.md`、`.trellis/spec/frontend/quality-guidelines.md`。
* 相关代码示例：`TaskManagementPage` 的 `AlertDialog`、各页面的 `empty-state error`、API `traceId` 类型。
* 已新增规范文件：`.trellis/spec/frontend/interaction-guidelines.md`。
