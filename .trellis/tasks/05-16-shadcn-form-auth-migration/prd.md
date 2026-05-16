# 表单认证 shadcn 控件迁移

## Goal

将任务表单、登录表单和管理员初始化表单迁移到 shadcn 本地表单控件，减少裸 `input/select/textarea/checkbox` 与全局 `.field` 样式耦合，同时保持当前视觉和表单行为不变。

## Requirements

* 迁移 `TaskForm` 的 `Input`、`Textarea`、`Select`、`Checkbox`、提交 `Button`。
* 迁移 `LoginPanel` 和 `AdminSetupPanel` 的输入框和提交按钮。
* 保留当前 React Hook Form / Zod 的状态和校验边界，不因 UI 迁移改变 payload 映射。
* 保留当前 field label、错误文案、warning box、启用任务 checkbox 和提交禁用逻辑。
* UI 设计与当前保持一致：表单两列布局、控件高度、错误样式、按钮位置和中文文案不做产品层变化。

## Acceptance Criteria

* [x] `TaskForm` 不再直接使用裸 `input/select/textarea/checkbox` 作为主要控件。
* [x] `LoginPanel` 和 `AdminSetupPanel` 使用 `components/ui` 控件。
* [x] 任务创建/编辑、登录、初始化行为保持不变。
* [x] 现有 `TaskForm` 和 `AdminSetupPanel` 测试通过；必要时补充交互断言。
* [x] `pnpm --filter @taskdaemon/web lint`、`typecheck`、相关测试通过。

## Out of Scope

* 不重构表单 schema 或 API payload 映射。
* 不引入完整 Form abstraction，除非迁移过程中发现多个表单确实需要共享封装。
* 不迁移任务列表、运行历史或状态概览。

## Technical Notes

* 主要文件：`apps/web/src/features/tasks/TaskForm.tsx`、`apps/web/src/features/auth/LoginPanel.tsx`、`apps/web/src/features/auth/AdminSetupPanel.tsx`。
* 依赖子任务：`05-16-shadcn-ui-primitives`。
