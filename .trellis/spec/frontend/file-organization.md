# 前端文件组织与拆分

> 前端文件按职责增长，而不是按页面一次性堆叠。

---

## 概览

`apps/web` 当前体量还小，但任务表单、状态概览、任务表格、路由测试和全局样式会随着功能增长迅速变长。拆分目标是让页面主线清晰、业务转换可测试、UI section 可复用，并减少未来改动冲突。

拆分不以行数为唯一标准；行数只是提醒。真正信号是同一文件里出现多个职责簇，或者一次小改动需要阅读大量无关逻辑。

---

## 拆分信号

优先考虑拆分的情况：

* 单个页面或组件超过约 180 行，并且同时包含数据读取、业务计算、复杂 JSX 和多个交互分支。
* 单个测试文件超过约 200 行，并覆盖多个独立用户流程。
* 表单文件同时包含 schema、默认值、DTO 映射、字段 section 和提交副作用。
* 表格文件同时包含列定义、行操作、empty/loading/error 状态和复杂格式化。
* 全局 `styles.css` 中某一类组件样式开始跨越多个不相邻段落，或同类 class 出现复制变体。

暂不拆分的情况：

* 文件虽接近阈值，但只表达一个线性流程，读者能快速扫完。
* 拆分会迫使状态在多个子组件间来回传递，反而增加理解成本。
* 只为一次性功能临时存在，且近期会删除。

---

## 页面与路由

* `routes` 文件只声明 route tree、loader/action 绑定和页面入口。
* 受保护入口、认证分流和 app shell 可以集中在路由根组件；具体页面 UI 下沉到 feature。
* 页面组件只组合 query/mutation hook、route params、feature 组件和少量页面状态。
* 创建/编辑/详情等复杂流程使用独立路由，不嵌进列表页。
* route tree 增长后，按业务域拆成 `routes/<domain>.routes.tsx` 或相近模式，保持根路由文件可扫描。

---

## Feature 组件

* feature 组件按业务意图命名，例如 `TaskForm`、`TaskTable`、`RunHistoryTable`。
* 页面级 feature 超过阈值时，优先拆成同目录私有 section，例如 `TaskRunnerSection`、`TaskCronSection`。
* 复杂计算下沉到纯函数，例如 `selectFocusTask`、`buildStatusMetrics`、`toTaskPayload`，并在有业务分支时测试。
* 子组件 props 用业务名词，避免 `data`、`item`、`config` 这类宽泛名称。
* 子组件不直接读取 query cache 或调用 mutation；动作由父级 feature/page 注入。

---

## 表单

复杂表单按四层组织：

* `*.schema.ts`：Zod schema、表单值类型、DTO -> form 默认值、form -> API payload。
* `TaskForm.tsx` 这类主表单：创建 RHF form、提交、组合 section。
* `*Section.tsx`：字段分组，只绑定字段和展示错误。
* feature 工具函数：解析 env/args、格式化 label、软警告展示。

表单拆分时，form state 和提交边界保持单一来源；不要让多个 section 各自维护重复状态。

---

## API、Query 与类型

* `src/lib/api` 是 HTTP、DTO、envelope 和错误映射边界。
* `*.queries.ts` 放 feature query key、query hook 和 mutation hook。
* DTO 类型集中在 `src/lib/api/types.ts` 或未来生成目录；feature 专属 view model 放在 feature 内。
* API payload 映射放在 schema/mapper 中，组件不直接拼请求体。
* API/query 细节遵守 [API 与 Query 契约](./api-query-contracts.md)。

---

## 测试文件

* 测试与被测文件相邻；跨测试共享工具放入 `src/test`。
* router 测试增长后按行为拆分，例如 auth gate、sidebar navigation、task routes。
* API client、schema、format/helper、query invalidation 和组件交互分别测试，不用一个集成测试覆盖所有风险。
* 不为了降低行数拆出无意义 helper；拆分后的测试名应能说明用户行为或业务分支。

---

## 样式文件

当前项目已引入 Tailwind CSS v4 与 shadcn/ui 本地组件模式，`styles.css` 同时承载 Tailwind 入口、theme variables、基础布局和过渡期全局 class。继续增长时按以下顺序整理：

1. 先把颜色、间距、radius、字号等重复值提升为 CSS variables 或 Tailwind theme token。
2. 再按 base/layout/components/utilities/media 分段整理过渡期全局 CSS。
3. 当某类控件有稳定复用模式时，抽成 `components/ui` 本地组件，而不是继续复制 class。
4. shadcn CLI 新增组件后必须检查生成代码是否符合本项目命名、可访问性和依赖边界。

---

## 当前关注候选

* `features/status/StatusOverviewPage.tsx`：状态指标、关注任务、最近执行摘要可按 section 拆分。
* `features/tasks/TaskForm.tsx`：runner、cron、基础信息可按 section 拆分。
* `features/tasks/TaskTable.tsx`：行操作和状态 badge 可在行为增加后拆分。
* `routes/router.test.tsx`：auth gate、导航、任务 route flow 可分文件。
* `styles.css`：继续增长时优先 token 化和分段整理。

---

## 评审清单

* 文件名是否表达业务职责。
* 页面主线是否仍能一屏看懂。
* 状态是否被不必要地拆散到多个子组件。
* API payload、query key、DTO 类型是否仍在规定边界内。
* 测试拆分后是否更容易定位失败原因。
