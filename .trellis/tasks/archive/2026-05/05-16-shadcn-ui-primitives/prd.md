# shadcn UI primitives 与视觉对齐

## Goal

补齐第一批 `components/ui` 本地组件，并让这些组件的默认视觉和当前管理台保持一致，为后续页面迁移提供稳定基础。

## Requirements

* 新增或完善 shadcn/ui 本地组件：`Button`、`Input`、`Textarea`、`Label`、`Checkbox`、`Select`、`Badge`、`Table`、`AlertDialog`。
* 所有组件放在 `apps/web/src/components/ui`，使用 Radix 原语、cva variants 和 `cn`。
* 默认视觉贴近当前 `styles.css`：8px 以内圆角、紧凑高度、浅色管理台配色、表格高信息密度。
* `Button` 需要覆盖当前 `primary`、`subtle`、`icon-button`、`link-button` 的使用场景，避免业务页面继续复制按钮 class。
* 组件函数按项目规范补充参数和返回值说明。

## Acceptance Criteria

* [x] `components/ui` 中存在后续迁移需要的基础组件。
* [x] 组件 API 能覆盖任务列表和表单迁移需求。
* [x] 默认样式和当前 UI 保持一致，不产生明显视觉改版。
* [x] 不引入 TanStack Table 等重型依赖。
* [x] `pnpm --filter @taskdaemon/web lint` 和 `typecheck` 通过。

## Out of Scope

* 不迁移具体业务页面。
* 不删除全局 CSS。
* 不引入新视觉主题或深色模式。

## Technical Notes

* 后续依赖子任务：`05-16-shadcn-task-list-migration`、`05-16-shadcn-form-auth-migration`。
* 当前按钮、表单、表格、badge、empty/error 样式仍在 `apps/web/src/styles.css`。
