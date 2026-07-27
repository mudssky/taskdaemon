# T7b Implement · 备份模板向导 Web

## 顺序

1. `lib/api/types.ts`：追加 Template* DTO（对齐 T7a，不发明字段）
2. `lib/api/client.ts`：`listTemplates` / `getTemplate` / `renderTemplate` + client 测试
3. `features/tasks/templates/template.schema.ts` + test：动态 schema、defaults、mapper、fields 映射
4. `features/tasks/templates/template-wizard.ts` + test：步骤状态机
5. `features/tasks/templates/templates.queries.ts`：`templateKeys` + hooks
6. `TemplateParamFields.tsx` + `TemplateWizardPage.tsx`
7. `router.tsx` 追加 `/tasks/from-template`
8. `TaskManagementPage.tsx` 追加「从模板创建」入口（subtle，主路径仍是新建）
9. `queryClient.ts` 注释/登记 `templateKeys` 前缀归属（append-only）
10. 验收命令；勾选 PRD AC；`HANDOFF.md`

## 验证

```bash
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/web test
```

## 回滚点

- 删除 `apps/web/src/features/tasks/templates/**`
- 回退 router / TaskManagementPage / api types+client 追加段

## 不做

- merge
- 后端改动
- 发明 DTO
