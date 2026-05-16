# 任务列表 shadcn 组件迁移

## Goal

将任务管理页面和任务表格迁移到 shadcn 本地组件，减少 `.button`、`.icon-button`、`.badge`、裸 `table` 和 `window.confirm` 的维护负担，同时保持当前 UI 观感和行为不变。

## Requirements

* 迁移 `TaskManagementPage` 的刷新、新建任务按钮到 `Button`。
* 迁移 `TaskTable` 的编辑、启停、触发、取消、删除等行操作到 `Button` 的 icon 场景。
* 迁移任务状态到 `Badge`，保持当前 success/running/muted 等视觉语义。
* 迁移任务表格到 `Table` primitives，保留横向滚动和当前信息密度。
* 用 `AlertDialog` 替换删除任务的 `window.confirm`，保留当前确认文案含义、取消路径和禁用态。
* 迁移后页面视觉设计与当前保持一致，不改变布局、文案或业务流程。

## Acceptance Criteria

* [x] `TaskManagementPage` / `TaskTable` 不再依赖 `.button`、`.icon-button`、`.link-button`、`.badge`、裸 `table`。
* [x] 删除任务确认不再使用 `window.confirm`。
* [x] 删除、启停、触发、取消的禁用态和 loading/busy 行为保持不变。
* [x] 表格列、空状态、选中行和移动端横向滚动保持当前体验。
* [x] 相关组件测试通过，至少覆盖删除确认取消/确认路径。
* [x] `pnpm --filter @taskdaemon/web lint`、`typecheck`、相关测试通过。

## Out of Scope

* 不迁移任务表单。
* 不迁移运行历史表格和状态概览。
* 不引入排序、筛选、分页等新表格能力。

## Technical Notes

* 主要文件：`apps/web/src/features/tasks/TaskManagementPage.tsx`、`apps/web/src/features/tasks/TaskTable.tsx`。
* 依赖子任务：`05-16-shadcn-ui-primitives`。
