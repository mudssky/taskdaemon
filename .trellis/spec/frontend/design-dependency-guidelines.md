# 设计系统与依赖治理规范

> taskdaemon 是工具型管理台，UI 体系要稳定、克制、可复用；依赖新增要证明长期收益。

---

## 适用范围

本规范适用于 `apps/web` 的设计 token、`src/styles.css`、`components/ui`、shadcn/ui 本地组件、图标与常用 UI 模式，以及前端运行依赖和开发依赖的新增决策。

某次视觉调整、单个页面控件清单、营销页风格和完整设计稿不写入本长期规范。

---

## 视觉定位

前端保持专业、安静、高信息密度的运维/开发工具体验。优先让任务列表、运行状态、执行历史和表单在长时间使用中可扫描、可比较、可恢复。

* 不创建营销式 landing page、巨大 hero 或装饰性低信息密度布局。
* 页面区块使用清晰布局和节奏，不靠多层浮动卡片制造层次。
* 表格、状态 badge、表单 section、确认对话框和 empty/error state 是核心组件模式。
* 默认浅色管理台；深色模式需要整体设计后再启用，不能为单个组件临时补一套暗色。

---

## Token 与全局样式边界

`src/styles.css` 的长期职责：

* Tailwind v4 入口和 shadcn 所需 import。
* `:root`、`.dark` 和 `@theme inline` 中的颜色、radius、语义状态 token。
* 全局 base 样式，例如字体、box sizing、body、基础链接和原生控件 reset。
* app shell、sidebar、content 这类跨页面布局。
* 迁移期保留的旧全局 class，例如 `.panel`、`.empty-state`、`.button`、`.badge`。

`src/styles.css` 不应继续承载新增的可复用控件逻辑。出现以下情况时，应改为沉淀到 `components/ui` 或 feature 组件：

* 同一 class 被多个页面当作可复用控件使用。
* 需要 variant、size、disabled、selected、destructive 等状态组合。
* 需要可访问性属性、键盘交互或 Radix primitive。
* 需要和 shadcn token、`cn`、`cva` 组合维护。

重复色值、间距、radius、字号、阴影和状态色必须优先沉淀为 CSS variable、Tailwind theme token 或组件 variant。禁止在多个页面散落新的十六进制色值；如果只是短期修补，必须先确认是否已有语义 token 可用。

---

## shadcn/ui 与本地组件

项目采用 shadcn/ui 本地组件模式，配置文件位于 `apps/web/components.json`，组件别名使用 `@/components/ui`，工具函数使用 `@/lib/utils`。

新增或迁移可复用控件时：

* 优先使用已有 `components/ui` 组件，例如 `Button`、`Badge`、`Table`、`Input`、`Textarea`、`Select`、`Checkbox`、`Label`、`AlertDialog`。
* 本地 UI 组件使用 Radix primitive、`cva` variant、`cn` 合并 class，并保留标准 HTML props。
* variant/size 扩展集中在对应 UI 组件内，不在单个页面复制一套 helper。
* 图标按钮使用 `lucide-react`，按钮内图标使用 `data-icon`，由组件样式控制尺寸。
* 可访问性要求随组件一起封装，例如 dialog title/description、字段 label、表格语义和图标按钮 `aria-label`。

过渡期允许旧全局 class 与 shadcn 组件共存，但新增代码优先组件化。迁移旧页面时按“重复最多、状态最多、维护成本最高”的顺序替换，不为了清理 CSS 做无业务收益的大改。

---

## 常用 UI 模式归属

* Button：通用命令、链接按钮、图标按钮进入 `components/ui/button.tsx` 的 variant/size。
* Badge：任务状态、执行状态、危险/警告/成功短文本进入 `components/ui/badge.tsx` 的 variant。
* Table：任务列表和执行历史使用 `components/ui/table.tsx`，保持横向滚动容器，避免页面级横向滚动。
* AlertDialog：删除、取消运行、高风险提交等确认操作使用 `components/ui/alert-dialog.tsx`。
* Form controls：Input、Textarea、Select、Checkbox、Label 优先使用 `components/ui`，复杂表单状态仍归 feature/schema 管理。
* Empty/Error state：迁移期可继续使用 `.empty-state`；当多个页面需要图标、标题、描述和操作组合时，应沉淀为本地 `Empty` 或 `Alert` 组件。
* Panel/Layout：`.panel`、`.dashboard-grid`、`.overview-grid` 属于过渡期布局 class；若未来多个页面出现相同结构，再抽成 layout 组件。

单页面不得临时创造第二套 `.button-*`、`.badge-*`、`.table-*` 或新的 variant helper。确实只有该 feature 使用的展示结构留在 feature 内，不上移到全局。

---

## 依赖新增门槛

新增前端依赖前必须回答：

* 现有 React、TanStack、React Hook Form、Zod、dayjs、Tailwind、shadcn/Radix、lucide 是否已经能覆盖需求。
* 依赖是否跨 Web/Desktop 可用，不依赖不可控浏览器扩展或 Node-only API。
* 是否会引入第二套 UI、表单、路由、状态、日期或样式方案。
* 是否显著增加 bundle、运行时复杂度或测试维护成本。
* 是否只是单页面临时需求；如果是，优先本地实现或延迟引入。
* 是否需要配套类型、mock、测试工具或安全审查。

运行依赖放在 `apps/web/package.json`；workspace 共享开发/测试工具放在根 `devDependencies`。不要把应用运行依赖放到根目录，也不要在子包重复声明 Biome、Vitest、Testing Library、TypeScript 这类共享工具。

引入重型依赖必须有明确收益和替代方案比较。当前默认禁止为单个页面引入图表库、Monaco Editor、Framer Motion、终端模拟器、moment 或第二套 UI 组件库。

---

## 禁止模式

* 在单个页面新增一套与 shadcn/ui 并行的按钮、badge、表格或表单控件体系。
* 在多个文件散落新的十六进制色值、radius、阴影或状态色。
* 把可复用控件继续堆进 `styles.css`。
* 为短期页面效果引入重型依赖或第二套解决方案。
* 为 Desktop 单独复制一套 UI。
* 用 emoji、手写 SVG 图标或非统一图标库替代 `lucide-react`。
* 为纯视觉微调编写脆弱 DOM 结构测试。
