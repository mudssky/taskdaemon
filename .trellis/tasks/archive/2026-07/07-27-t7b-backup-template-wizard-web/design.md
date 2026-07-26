# T7b Design · 备份模板向导 Web

## 1. 边界

| 层 | 职责 |
|---|---|
| `apps/web/src/features/tasks/templates/**` | 向导状态机、动态表单、预览、提交衔接（**独占**） |
| `apps/web/src/lib/api/{types,client}.ts` | 追加 T7a DTO 与 `listTemplates` / `getTemplate` / `renderTemplate`（**append-only**） |
| `apps/web/src/routes/router.tsx` | 追加 `/tasks/from-template`（**append-only**） |
| `apps/web/src/app/queryClient.ts` | 可选登记 `templateKeys` 前缀说明（**append-only**） |
| `apps/web/src/features/tasks/TaskManagementPage.tsx` | 「从模板创建」入口（**append-only**） |
| `apps/web/src/features/tasks/tasks.queries.ts` | **只消费** `useCreateTaskMutation`；不新增第二条创建路径 |

**禁止**：发明 DTO 字段；绕过 `POST /api/tasks`；硬编码某一模板字段；反向解掩码 `commandPreview`。

## 2. 消费契约（T7a，不发明）

### 2.1 参数类型

`string` | `number` | `boolean` | `enum` | `path` | `secret_ref`

### 2.2 模板定义（列表/详情 `data`）

```ts
type TemplateParamDef = {
  name: string;
  type: TemplateParamType;
  required: boolean;
  default?: unknown;
  enumOptions?: string[];
  help?: string;
  min?: number;
  max?: number;
  sensitive?: boolean;
};

type TemplateDefinition = {
  id: string;
  name: string;
  description: string;
  scenario: string;
  runnerType: string;
  params: TemplateParamDef[];
};
```

列表：`GET /api/templates` → `{ templates: TemplateDefinition[] }`  
详情：`GET /api/templates/:id` → `TemplateDefinition`

### 2.3 渲染

`POST /api/templates/:id/render` body `{ params: Record<string, unknown> }`

成功 `data`（`TemplateTaskDraft`）与 `createTaskRequest` 同形，并额外：

- `commandPreview: string`（敏感已掩码）
- `templateId: string`

字段级错误：`error.details.fields[]` → `{ path, reason, code }`，`path` 为 `params.<name>`。

### 2.4 提交

`draftToTaskPayload(draft)` 去掉 `commandPreview` / `templateId`，其余交给既有 `apiClient.createTask` / `useCreateTaskMutation`（已 invalidate `tasksKeys`）。

## 3. 模块拆分

| 文件 | 职责 |
|---|---|
| `template.schema.ts` | 参数类型常量、`buildParamSchema(params)`、默认值、`toRenderParams`、`draftToTaskPayload`、服务端 fields → 字段 map |
| `template-wizard.ts` | 步骤枚举、进度、是否可前进/后退、dirty 判定（纯函数） |
| `templates.queries.ts` | `templateKeys`、list/detail query、render mutation |
| `TemplateParamFields.tsx` | 按 `ParamDef` 渲染控件（无模板硬编码） |
| `TemplateWizardPage.tsx` | 多步编排：选模板 → 填参 → 预览确认 → 创建 |
| `*.test.ts(x)` | 动态 schema、payload、步骤机、新模板定义可渲染 |

## 4. 向导流程

```text
select ──(选中模板)──▶ params ──(客户端校验通过 + render)──▶ preview ──(createTask)──▶ /tasks
   ▲                    │                                      │
   └──── back ──────────┴──────────── back（保留 params） ─────┘
```

- 步骤与进度可见（1/3 选择、2/3 参数、3/3 预览）。
- 返回上一步**不丢**已填 `params` / 已选 `templateId`。
- dirty：已选模板或参数相对默认值变化 → 离开列表/路由前 `AlertDialog`；浏览器关闭用 `beforeunload`。
- 预览：展示 `commandPreview` 原文（可复制），**不做**二次加工；提交用同一次 render 的 draft。

## 5. 动态表单

- Zod 由 `ParamDef[]` **运行时生成**，不按模板 id 分支。
- 控件映射：
  - `string` / `path` / `secret_ref` → `Input`（secret_ref 文案标明「环境变量名」）
  - `number` → `Input type=number`
  - `boolean` → `Checkbox`
  - `enum` → `Select` + `enumOptions`
- 必填 `*` + `help` 展示在字段旁。
- 客户端规则 ⊆ 服务端：required、number min/max、enum ∈ options、string 非空（required 时）。
- 服务端 `params.foo` 错误 strip 前缀后 `setError(foo, …)`。

「新增模板无需改前端」：测试喂入虚构 `ParamDef[]`（含全部六种类型），断言 schema 默认值、校验与控件映射表成立。

## 6. Query

```ts
export const templateKeys = {
  all: ["templates"] as const,
  list: () => [...templateKeys.all, "list"] as const,
  detail: (id: string) => [...templateKeys.all, "detail", id] as const,
};
```

Render 为 mutation（每次预览显式触发），不缓存为 query（避免陈旧预览）。

## 7. 状态 / 错误

| 状态 | 表现 |
|---|---|
| list loading | 面板内加载文案 |
| list empty | 明确「暂无备份模板」 |
| list/detail error | 可恢复 + 展示 `traceId`（若有） |
| render 字段错 | 回 params 步并字段级展示 |
| render 其它错 | 预览区错误 + traceId |
| create 失败 | 留在 preview，保留 draft/params，可重试 |
| create 成功 | `navigate(/tasks)`；cache 由 create mutation 失效 |

## 8. 测试矩阵

| 场景 | 期望 |
|---|---|
| 六种类型 ParamDef → schema | 必填/类型/min-max/enum |
| 虚构第四模板定义 | 无需改映射代码即可 default + parse |
| `toRenderParams` / `draftToTaskPayload` | 无 preview/templateId 泄漏进 create body |
| fields 映射 | `params.host` → `host` |
| 步骤机 back/forward | 保留 params |
| apiClient | GET list/detail、POST render 路径与 method |
| 不测 | CSS、页面 DOM 结构快照 |

## 9. 不做

- 模板 CRUD / 用户自定义模板
- 备份文件管理 / restore UI
- 向导内直接执行备份
- 修改后端
