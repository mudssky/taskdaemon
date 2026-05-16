# brainstorm: shadcn 组件迁移

## Goal

在已经接入 shadcn/ui 基础设施后，逐步用 `components/ui` 本地组件替换现有散落的按钮、表单控件、表格、状态展示和确认交互，减少 `styles.css` 与业务组件之间的耦合，降低后续维护成本。

## What I already know

* 用户希望“前端应该引入 shadcn 组件”，并把部分现有代码替换成组件来降低维护成本。
* 当前 `apps/web` 已有 shadcn/ui 基础：`components.json`、Tailwind CSS v4、`@` alias、`cn`、`Button`。
* 现有代码仍大量使用过渡期全局 class：`.button`、`.icon-button`、`.link-button`、`.field input/select/textarea`、`.table-wrap`、`.badge`、`.panel`、`.empty-state`、`.form-error`。
* 可迁移候选分布：
  * `TaskManagementPage` / `TaskTable`：按钮、图标按钮、表格、badge、empty/error、`window.confirm` 删除确认。
  * `TaskForm`：input/select/textarea/checkbox、field error、fieldset、warning box、submit button。
  * `LoginPanel` / `AdminSetupPanel`：input、submit button、form error。
  * `RunHistoryTable` / `RunHistoryPage` / `StatusOverviewPage`：表格、panel、empty/error、badge-like 状态展示。
* shadcn/ui 官方模式是把组件复制到本地 `components/ui`，通过 CLI 增量添加，例如 `button`、`select`、`table`、`card`；Data Table 文档会额外引入 `@tanstack/react-table`，本轮未必需要。

## Assumptions (temporary)

* 本轮迁移目标是降低维护成本，不追求视觉大改版。
* 不一次性重写所有页面；优先替换重复度最高、风险最低的基础控件。
* 组件应保持工具型管理台的信息密度，不把页面改成低密度卡片化布局。
* 对破坏性确认，可以优先引入 Dialog/AlertDialog 取代 `window.confirm`，但是否纳入 MVP 需要确认。

## Open Questions

* 暂无阻塞问题。

## Requirements (evolving)

* 复用 shadcn/ui 本地组件模式，新增组件放在 `apps/web/src/components/ui`。
* 替换现有散落 UI class 时优先保证行为、可访问性和测试稳定。
* 减少业务组件直接依赖 `.button`、`.icon-button`、`.field input/select/textarea`、`.table-wrap`、`.badge`、`.empty-state` 等过渡期全局样式。
* 不为了迁移引入重型依赖；表格本轮优先使用 shadcn `Table` primitives，不默认引入 TanStack Table。
* 拆成 3 个子任务推进：UI primitives 与视觉对齐、任务列表迁移、表单/认证迁移。
* UI 设计必须和当前保持一致：布局密度、颜色观感、圆角、按钮尺寸、表格信息密度、中文文案和交互路径不做产品层变化。

## Acceptance Criteria (evolving)

* [x] 明确第一批 shadcn 组件迁移范围。
* [x] PRD 记录现有重复 UI 模式和迁移优先级。
* [x] 创建 3 个可独立推进的子任务。
* [x] 若进入实现，新增的 `components/ui` 组件符合本地 shadcn 模式。
* [x] 若进入实现，相关页面行为不变，已有测试通过。
* [x] 若替换确认交互，删除/取消等破坏性操作仍可取消且禁用态正确。
* [x] 迁移后页面视觉与当前 UI 保持一致，不引入明显布局、密度或配色变化。

## Definition of Done (team quality bar)

* 讨论结论记录在本 PRD。
* 若修改代码或配置，运行 `pnpm --filter @taskdaemon/web lint`、`typecheck`、相关测试；范围较大时跑 `build`。
* 文档主要内容使用中文。
* 不把一次性页面需求写入长期 spec，只有形成可复用约定时再更新 `.trellis/spec/frontend`。

## Out of Scope (explicit)

* 不做视觉大改版或整体重新设计。
* 不改变当前信息架构、导航、页面文案或业务流程。
* 不引入第二套 UI 库。
* 不默认引入 TanStack Table；除非后续需要排序、筛选、分页等数据表能力。
* 不一次性删除所有全局 CSS；保留布局、shell 和过渡期样式。

## Technical Notes

* 已扫描 `apps/web/src` 中裸 `button/input/select/textarea/table`、`.button`、`.icon-button`、`.badge`、`.panel`、`.empty-state` 等使用点。
* 已查看 `TaskForm.tsx`、`TaskTable.tsx`、`TaskManagementPage.tsx`、`LoginPanel.tsx`、`AdminSetupPanel.tsx` 和 `styles.css` 相关样式段落。
* 已通过 Context7 查询 shadcn/ui 官方文档：`/shadcn-ui/ui`。组件按需添加，Vite 项目通过本地 `components/ui` 导入；`select`、`table`、`card`、`button` 均是独立组件；Data Table 方案会额外需要 `@tanstack/react-table`。

## Feasible Approaches

### Approach A: 基础控件优先（Recommended）

* 先补 `Input`、`Textarea`、`Label`、`Checkbox`、`Select`、`Badge`，替换登录、初始化和任务表单里的裸控件。
* 优点：风险低、能立刻减少表单样式重复，测试影响可控。
* 缺点：任务列表里的表格、empty/error 和删除确认仍会保留一段时间。

### Approach B: 任务列表闭环优先

* 先围绕 `TaskManagementPage` / `TaskTable` 补 `Button` 使用、`Badge`、`Table`、`AlertDialog`，替换刷新、新建、行操作、删除确认和任务表格。
* 优点：用户最核心页面收益明显，能去掉 `window.confirm` 这个长期维护风险。
* 缺点：单次改动触及更多交互和测试，回归面更大。

### Approach C: 基础控件 + 最小列表按钮

* 先做表单基础控件，同时把所有 `.button` / `.icon-button` 替换成现有 `Button`，但暂不迁移 Table/Dialog。
* 优点：能最快消除按钮样式分叉，范围比 Approach B 小。
* 缺点：表格、badge、empty/error 仍然保留旧样式，迁移会更分散。

## Decision (ADR-lite)

### Decision: 拆成 3 个子任务渐进迁移

**Context**: shadcn 迁移会同时触及基础控件、任务列表、表单和认证页面。如果一次性替换所有 UI，容易造成视觉回归和测试面过大；如果只零散替换按钮，又难以降低长期维护成本。

**Decision**: 创建 3 个子任务，按依赖关系推进：

1. `05-16-shadcn-ui-primitives`：补齐本地 UI primitives，并让它们的默认尺寸、颜色和圆角贴近当前 UI。
2. `05-16-shadcn-task-list-migration`：迁移任务列表闭环，包括 Button、Badge、Table、AlertDialog，移除 `window.confirm`。
3. `05-16-shadcn-form-auth-migration`：迁移任务表单、登录和初始化表单控件。

**Consequences**: 子任务之间边界清晰，第一步先解决“组件长什么样”，后续迁移只替换使用点。所有子任务都必须以视觉保持当前一致为验收标准，不做产品视觉改版。

## Subtasks

* `.trellis/tasks/05-16-shadcn-ui-primitives` — 建立可复用 UI primitives 与当前视觉对齐。
* `.trellis/tasks/05-16-shadcn-task-list-migration` — 迁移任务管理页面和任务表格。
* `.trellis/tasks/05-16-shadcn-form-auth-migration` — 迁移任务表单、登录和管理员初始化表单。
