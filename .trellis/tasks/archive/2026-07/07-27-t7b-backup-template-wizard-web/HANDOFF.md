# HANDOFF · T7b 备份模板向导 Web

> 任务：`07-27-t7b-backup-template-wizard-web`  
> 分支：`mudssky/t7b-backup-template-wizard-web`  
> 日期：2026-07-27  
> 状态：**前端已交付** — 待协调者验收后 merge + archive（本 worker **不 merge**）

## 结论

- 备份模板多步向导已落地：选择模板 → 填写参数 → 预览命令 → 走既有 `POST /api/tasks` 创建。
- 动态表单完全由 T7a `ParamDef` 驱动，六种参数类型均支持；虚构第四模板定义的单测证明新增模板无需改前端映射。
- 不发明 DTO；`commandPreview` / `templateId` 仅用于预览，提交前 `draftToTaskPayload` 剥离。
- 未反向解掩码；预览可复制；服务端 `params.*` 字段错误映射到表单字段。
- 中途离开：`AlertDialog` + `beforeunload`。

## 交付物

### 代码

| 路径 | 说明 |
|---|---|
| `apps/web/src/features/tasks/templates/**` | schema / 步骤机 / queries / ParamFields / WizardPage + 测试 |
| `apps/web/src/lib/api/types.ts` | Template* DTO（对齐 T7a） |
| `apps/web/src/lib/api/client.ts` | `listTemplates` / `getTemplate` / `renderTemplate` |
| `apps/web/src/lib/api/client.test.ts` | 模板端点路径与 method |
| `apps/web/src/routes/router.tsx` | `/tasks/from-template`（append-only） |
| `apps/web/src/features/tasks/TaskManagementPage.tsx` | 「从模板创建」入口（subtle，主路径仍是新建） |
| `apps/web/src/app/queryClient.ts` | 登记 `templateKeys` 归属注释 |
| `apps/web/src/styles.css` | 向导 / 模板卡片样式 |

### 规划

- `design.md` / `implement.md` / PRD AC 已勾选

## 路由与 API

```text
GET  /api/templates
GET  /api/templates/:id
POST /api/templates/:id/render   → TemplateTaskDraft
POST /api/tasks                  ← draftToTaskPayload(draft) 既有创建

Web: /tasks/from-template
```

`templateKeys`：`["templates"]` / list / detail(id)

## 验证

```bash
pnpm typecheck   # 绿
pnpm lint        # 绿（web package biome clean；根仓库既有 warnings 与本任务无关）
pnpm --filter @taskdaemon/web test   # 62 passed
```

## 残留 / 下游

| 项 | 承接 |
|---|---|
| merge + archive | **协调者** |
| 路线图 §7.3 状态回写 | 协调者归档时 |
| 真后端联调冒烟 | 协调者 / 本地 daemon 起服后走一遍向导 |
| 模板用户自定义 | Out of Scope |

## 给协调者

- 不要由本 worker merge。
- 入口在任务列表「从模板创建」；主路径「新建任务」不变。
- 创建成功后 invalidate 复用 `useCreateTaskMutation` → `tasksKeys`。
