# brainstorm: 前端长期维护性规范优化

## Goal

从长期维护性角度审视 `apps/web` 现有代码与 `.trellis/spec/frontend` 规范，讨论哪些规范需要优化、补充或拆分。目标是让未来前端迭代在目录边界、组件复杂度、状态管理、API 契约、测试策略和 UI 可维护性上更稳定，而不是把单次需求写成长期规则。

## What I already know

* 用户希望先讨论前端规范优化，而不是马上实现。
* 刚完成一轮 `.trellis/spec` 整理，前端已有 `directory-structure`、`component-guidelines`、`hook-guidelines`、`state-management`、`type-safety`、`quality-guidelines`。
* 当前前端位于 `apps/web`，使用 Vite + React + TypeScript、Tailwind CSS v4、shadcn/ui 本地组件模式、TanStack Router、TanStack Query、React Hook Form、Zod、Biome、Vitest + Testing Library。
* 本次应以长期维护性为核心，重点识别“还需要什么规范”，而不是重复历史 PRD 或具体页面需求。
* 现有前端源码最大文件约 212 行：`routes/router.test.tsx`、`features/status/StatusOverviewPage.tsx`、`features/tasks/TaskForm.tsx`、`TaskTable.tsx`、`lib/api/client.test.ts`、`task.schema.ts`、`lib/api/client.ts`。
* 代码目前没有明显过大文件，但 `TaskForm`、`StatusOverviewPage`、`TaskTable`、`router.test.tsx` 是后续功能增长时最容易膨胀的候选。
* 现有规范覆盖目录、组件、hook、状态管理、类型安全和质量检查，但还缺少类似后端 `file-organization` 的前端文件/组件拆分判断。
* API client 当前是唯一 `fetch` 边界，DTO 类型集中在 `src/lib/api/types.ts`，query/mutation hook 集中在 feature 附近；这个模式适合沉淀成更明确的 API 契约演进规范。
* 当前存在 `window.confirm` 用于删除确认；从长期可维护性和一致 UX 看，破坏性操作确认应该有规范，未来更适合统一 Dialog/Confirm 组件边界。
* `tasks.queries.ts` 中存在 `refetchInterval` 轮询逻辑；长期需要规范轮询触发条件、页面可见性、running 状态和避免无条件高频请求。
* `styles.css` 已经沉淀大量 token-like 色值、布局类和控件类，但 spec 还没有明确“样式/token/design system 演进”的维护边界。

## Assumptions (temporary)

* 本轮先形成规范优化方向和优先级，不默认直接改 `.trellis/spec/frontend`。
* 可从现有代码推导的规范缺口不问用户，先整理候选。
* 可能新增的规范方向包括：前端文件/组件拆分、路由与页面组织、API 契约演进、表单复杂度、测试分层、可访问性/交互状态、性能与轮询、错误展示与恢复。
* 如果进入 spec 修改，优先新增少量高价值规范文件，而不是把所有内容塞回现有文档。

## Open Questions

* 暂无阻塞问题。

## Requirements (evolving)

* 从现有前端代码和 spec 出发，总结长期维护性风险。
* 区分应写入长期 spec 的工程规则与不应写入 spec 的需求型内容。
* 提出需要新增或优化的前端规范清单，并给出优先级。
* 保持每次只问一个高价值问题。
* 规范应服务未来维护者和 AI 代理判断边界，避免写成“当前页面功能说明”。
* 本轮先产出完整规范缺口清单。
* 第一批落地范围：规范与实际技术栈对齐、前端文件组织与拆分、API/query 契约。

## Acceptance Criteria (evolving)

* [x] 识别现有前端 spec 已覆盖内容与缺口。
* [x] 从 `apps/web` 代码中总结维护性风险和可沉淀规范。
* [x] 给出 2-3 个可选规范优化方向。
* [x] 明确本轮是否进入 spec 修改，还是只产出讨论结论。
* [x] 更新前端 spec，使技术栈描述与当前依赖一致。
* [x] 新增或更新前端文件组织/拆分规范。
* [x] 新增或更新 API/query 契约规范。
* [x] 最小接入 shadcn/ui 基础设施，不重写现有业务页面。

## Definition of Done (team quality bar)

* 讨论结论记录在本 PRD。
* 若后续修改 spec，文档主要内容使用中文。
* 若修改代码或配置，运行对应检查；纯讨论不跑测试。

## Out of Scope (explicit)

* 暂不默认修改业务前端代码。
* 暂不默认重写所有前端 spec。
* 不把具体页面需求、控件清单、一次性验收标准写入长期规范。

## Technical Notes

* 已创建任务目录：`.trellis/tasks/05-16-frontend-maintainability-specs`。
* 已查看 `.trellis/spec/frontend/index.md`、`component-guidelines.md`、`type-safety.md`、`quality-guidelines.md`。
* 已查看关键实现：`apps/web/src/features/status/StatusOverviewPage.tsx`、`features/tasks/TaskForm.tsx`、`TaskTable.tsx`、`TaskManagementPage.tsx`、`features/runs/RunHistoryPage.tsx`、`routes/router.tsx`、`routes/router.test.tsx`、`lib/api/client.ts`、`lib/api/types.ts`、`features/tasks/tasks.queries.ts`、`styles.css`、`components/layout/AppShell.tsx`。
* 维护性候选规范：
  * 新增 `frontend/file-organization.md`：页面/组件/form/query/test 何时拆分、文件体量阈值、拆分原则。
  * 新增或强化 API 契约规范：`apiClient` 单一 fetch 边界、DTO/Envelope/Error 映射、query key/mutation invalidation、API 变更联动测试。
  * 强化组件规范：破坏性操作确认、统一 loading/empty/error/disabled 状态、避免 `window.confirm` 长期散落。
  * 强化状态/Hook 规范：轮询必须由 running/可见性/页面范围驱动，避免常驻高频刷新。
  * 新增设计系统/样式规范：颜色、间距、radius、表格、按钮、badge、响应式布局的 token 与复用边界。
  * 强化测试规范：router 集成测试覆盖导航和受保护入口；组件测试聚焦业务交互，不测试纯布局；复杂表单 schema/映射用纯函数测试兜底。

## Decision (ADR-lite)

**Context**: 前端规范已有基础骨架，但用户希望先从长期维护性角度完整展开缺口，不急于进入优先级排序或 spec 修改。

**Decision**: 本轮先产出完整规范缺口清单，作为待办池；后续再决定哪些进入第一批 spec 优化。

**Consequences**: 清单会覆盖工程边界、UI 设计系统、测试、API、状态、性能和可访问性等多个方向；其中部分可能只需补章节，部分可能适合新增独立规范文件。

### Decision: 第一批落地范围

**Context**: 完整清单覆盖较广，需要先挑最能防止后续维护漂移的规范进入 spec。

**Decision**: 第一批落地三块：1) 规范与实际技术栈对齐；2) 前端文件组织与拆分；3) API/query 契约。

**Consequences**: 暂不处理 UI 设计系统、测试分层、可访问性、轮询性能等其余清单项；这些保留在 backlog，后续分批进入 spec。

### Decision: 引入 shadcn/ui

**Context**: 现有 spec 曾描述 Tailwind/Radix/cva 方向，但此前工程没有接入对应依赖和配置。用户确认需要把 shadcn 引入，避免规范和真实工程继续漂移。

**Decision**: 在 `apps/web` 最小接入 shadcn/ui 基础设施：Tailwind CSS v4 Vite 插件、`components.json`、`@` alias、`cn` 工具、`components/ui/button.tsx` 和 theme variables。现有页面暂不大规模重写，后续新增/迁移可复用控件走 shadcn 本地组件模式。

**Consequences**: 当前全局 CSS 进入过渡期；后续 UI 工作应逐步把按钮、表单、表格、Dialog 等可复用控件迁入 `components/ui`，避免 shadcn 与旧 class 长期并行失控。

## Candidate Spec Backlog

### 1. 规范与实际技术栈对齐

* 已决定引入 shadcn/ui 作为长期 UI 组件模式。
* Tailwind CSS v4、Radix primitives、cva、tailwind-merge 和 `components.json` 应与 spec 同步。
* 现有 `src/styles.css` 全局 class 作为过渡层保留，后续新增可复用控件优先沉淀到 `components/ui`。
* 维护风险：如果 spec 与真实依赖/CLI 配置再次漂移，AI/开发者会写出无法编译或风格分裂的 UI。

### 2. 前端文件组织与拆分规范

* 需要新增类似后端 `file-organization.md` 的前端文件拆分规则。
* 候选规则：
  * 页面组件只做 route/query/action 组合，超过约 150-200 行或出现 3 个职责簇时拆分。
  * 表单组件按 section/field 拆分，但 form state、schema、submit mapping 保持单一边界。
  * 表格组件把列渲染、行操作、empty/loading/error 状态分清。
  * 测试文件超过约 200 行时按行为拆分，例如 auth gate、sidebar navigation、task routes。
  * `styles.css` 继续增长时需要按 token/base/layout/components/utilities 或引入样式体系拆分。
* 当前候选文件：`StatusOverviewPage.tsx`、`TaskForm.tsx`、`TaskTable.tsx`、`router.test.tsx`、`styles.css`。

### 3. API 契约与前后端联动规范

* 需要更明确 `apiClient` 是唯一 fetch 边界，组件和 feature 不直接调业务 API。
* DTO 类型、API envelope、`ApiClientError`、`traceId`、稳定错误码应有长期契约。
* API 变更时需要联动：
  * `src/lib/api/types.ts`
  * `src/lib/api/client.ts`
  * feature query/mutation hook
  * schema/default value/payload mapper
  * API client 测试和关键组件交互测试
* 手写 client 阶段是否对响应做最小运行时校验，需要明确边界；否则 `as ApiEnvelope<T>` 会把后端漂移推迟到 UI 运行时才暴露。

### 4. Query、mutation 与缓存失效规范

* 现有 `tasksKeys` 模式值得保留，但需要规范 key 层级、详情/列表/history 的命名方式。
* mutation 成功后的 invalidate/remove/setQueryData 需要标准化，尤其删除任务、触发任务、取消任务这类会影响多个视图的动作。
* 需要规范 query hook 的返回边界：hook 负责缓存和派生少量业务状态，组件负责展示，不把 toast/导航/复杂 UI 副作用塞进 hook。
* 需要明确同一数据在状态页、任务页、历史页之间的复用策略，避免多个 feature 各自请求和缓存同一业务数据。

### 5. 轮询、刷新与性能规范

* 当前 `refetchInterval` 根据 running 状态短轮询，这是好模式，但需要更完整的长期规则。
* 候选规则：
  * 默认不常驻轮询，只在存在 running task/run 或用户所在页面需要实时状态时开启。
  * 轮询间隔集中常量化，避免多个文件写死 `3000`。
  * 页面不可见、没有 running 项、任务为空时停止轮询。
  * 手动刷新按钮和自动轮询的 loading 状态不要互相干扰。
* 维护风险：后续页面增多后，多处轮询叠加会造成无感请求风暴。

### 6. 表单与校验规范

* 现有 `task.schema.ts` 已经把 default values、payload mapping、env/args parsing 集中管理，值得规范化。
* 需要明确复杂表单的分层：
  * schema 负责输入边界。
  * `defaultXxxValues` 负责 DTO -> form。
  * `toXxxPayload` 负责 form -> API。
  * UI section 只绑定字段和展示错误。
* `AdminSetupPanel`、`LoginPanel` 当前用局部 `useState`，后续如果认证表单复杂化，应统一到 RHF/Zod，避免两套校验风格长期并存。
* 需要定义软警告模式，例如 cron warning：硬错误阻断提交，软警告通过显式确认进入 payload。

### 7. 交互状态与错误恢复规范

* 需要统一 loading、empty、error、disabled、pending、success feedback 的展示方式。
* 当前多个页面直接写中文错误文案，后续应规定：
  * 字段错误来自 schema 或稳定 API code。
  * 页面级错误显示可恢复动作，例如刷新或重新登录。
  * mutation 失败时按错误码分支，不解析 message。
* `window.confirm` 适合作为早期实现，但不适合作为长期规范；破坏性操作应统一 Confirm Dialog/Modal 边界，支持可访问性、禁用态和错误恢复。

### 8. UI / 设计系统 / 样式规范

* 当前 `styles.css` 已经包含 shell、sidebar、panel、table、badge、button、form、auth、responsive 等多类规则，需要明确增长边界。
* 候选规范：
  * 色彩、间距、radius、字号、阴影等 token 先集中在 `:root` CSS variables，避免散落 hex。
  * 组件 class 命名按语义分层：layout、panel、button、form、table、badge、state。
  * 禁止为单个页面复制只差颜色/间距的 class。
  * 全局 CSS 只放 Tailwind 入口、theme variables、基础布局和过渡期模式；可复用控件沉淀到 `components/ui`。
  * 图标按钮、危险操作、表格、badge、empty state 应形成统一视觉和交互模式。
* 已决定引入 Tailwind/Radix/cva/shadcn 模式并同步 spec 与依赖。

### 9. 可访问性规范

* 现有代码已有 `aria-label`、`aria-labelledby`、图标 `aria-hidden` 等基础做法。
* 还需要规范：
  * 图标按钮必须有可访问名称。
  * Dialog/Popover/Tooltip 使用可访问原语或自建时补齐 focus trap/escape/aria。
  * 表单错误需要和字段关联；不仅仅在字段下显示文本。
  * 状态不能只靠颜色，badge 必须有文字。
  * 表格行操作在键盘下可达，破坏性操作可取消。

### 10. 路由与认证守卫规范

* `ProtectedApp` 当前集中处理 initialized/authenticated/loading/error，这是好边界。
* 需要规范：
  * auth status 是受保护入口的基础 query，不在每个页面重复请求认证。
  * 页面标题来自 route/path 时要保持集中映射，避免每页重复。
  * 创建/编辑等复杂流程使用独立路由，不塞进列表页。
  * URL search params 用于筛选、分页、选中对象时必须有类型校验和默认值。
* 未来如果 route tree 增长，需要考虑 route 文件拆分规则。

### 11. 测试分层规范

* 现有测试覆盖 cron、schema、API client、关键组件和 router 集成，方向正确。
* 缺口：
  * router 测试文件增长后按 auth gate、navigation、task flows 拆分。
  * API client 测试需要覆盖 envelope 成功、非 JSON 错误、traceId、稳定 error code。
  * query hook 可用 QueryClient 测试包装，覆盖 invalidation/remove cache。
  * 复杂组件测试聚焦用户行为和业务分支，不测试 DOM 层级或 CSS。
  * 纯函数测试兜底格式化、payload mapping、cron/env/args parsing。

### 12. 依赖治理规范

* 需要明确新增依赖的门槛：是否已有本地模式、是否影响 bundle、是否跨 Web/Desktop 可用、是否有安全/维护风险。
* UI/状态/日期/表单/路由类依赖尤其要谨慎，避免多个解决同一问题的库并存。
* Tailwind/Radix/cva/shadcn 已作为明确技术迁移进入第一批落地范围，不允许在单个页面另起一套控件体系。

### 13. Web/Desktop 共用边界规范

* 现有前端服务 Web 和 Wails Desktop，两者应共用 UI。
* 需要补充：
  * Desktop 专属能力通过 hook/service 边界注入，提供 Web fallback。
  * 页面组件不直接依赖 Wails 全局对象。
  * `/api` 请求始终走普通网络路径，不假设 desktop resource server 能转发 body。
  * 桌面能力相关状态不进入普通业务 DTO。

### 14. 国际化与文案规范（可后置）

* 当前 UI 文案全中文，符合项目语言要求。
* 长期如果出现英文技术短语、错误码、runner 类型展示，需要规定：
  * 业务文案集中映射，例如状态/runner/trigger label。
  * API message 不作为 UI 稳定文案。
  * 错误码到用户文案的映射可逐步集中。

### 15. 观测与调试规范（可后置）

* API error 已携带 `traceId`，前端需要规范何时展示或复制 traceId。
* 前端不应随意 `console.log` 敏感数据；调试日志若需要，应集中封装并在生产关闭。
* 错误页面/错误状态可以提供 traceId 或刷新动作，但不暴露 token、密码、完整请求体。
