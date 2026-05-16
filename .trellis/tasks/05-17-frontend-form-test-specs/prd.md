# 前端表单校验与测试分层规范

## Goal

补齐复杂表单、软警告、schema/payload 映射和测试分层规范，让后续任务表单、认证表单和更多配置表单能稳定扩展，同时避免测试文件变成不可维护的大杂烩。

## Requirements

* 规范复杂表单分层：schema、default values、DTO -> form、form -> payload、UI section 的职责边界。
* 规范 React Hook Form + Zod 的使用边界，避免局部 state 校验和 RHF/Zod 长期并行失控。
* 规范软警告模式：例如 cron 高频 warning，区分硬错误和需要显式确认的风险。
* 规范测试分层：纯函数、schema、API client、组件交互、router 集成测试分别覆盖什么。
* 规范大型测试文件拆分时机和拆分方式。
* 将稳定规则写入 `.trellis/spec/frontend`，必要时更新 `type-safety.md`、`quality-guidelines.md`、`file-organization.md` 或新增表单规范文件。

## Acceptance Criteria

* [ ] `.trellis/spec/frontend` 包含表单校验和 payload 映射长期规则。
* [ ] 明确硬错误、软警告、显式确认的边界。
* [ ] 明确复杂表单和简单表单的实现选择。
* [ ] 明确测试分层与测试文件拆分规则。
* [ ] 更新 `index.md` 的规范索引或开发前检查项。

## Definition of Done

* 文档主要内容使用中文。
* 不测试纯页面结构和 CSS 样式的现有规则继续保留。
* 只修改 spec 时不需要运行前端测试；如同步改测试工具或代码，再跑对应检查。

## Out of Scope

* 不重构 `TaskForm`、`LoginPanel` 或 `AdminSetupPanel`。
* 不引入新的表单库。
* 不把每个当前字段的校验规则复制进 spec。

## Technical Notes

* 来源 backlog：归档 PRD 中的“表单与校验规范”“测试分层规范”。
* 相关现有规范：`.trellis/spec/frontend/type-safety.md`、`.trellis/spec/frontend/quality-guidelines.md`、`.trellis/spec/frontend/file-organization.md`。
* 相关代码示例：`apps/web/src/features/tasks/task.schema.ts`、`TaskForm.tsx`、`router.test.tsx`、`client.test.ts`。
