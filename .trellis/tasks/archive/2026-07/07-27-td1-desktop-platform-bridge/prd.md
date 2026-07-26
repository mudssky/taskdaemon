# TD1 Desktop 平台能力边界层

> **轨**：Track D ｜ **依赖**：无（W1 波次，可立即开工）
> **契约冻结点**：**C-5 Desktop capability 契约**。D2、D3 与全部 Track W 任务消费本任务产物。

## Goal

落地 `apps/web/src/lib/desktop` 能力边界层，让业务组件以统一方式消费 Desktop 原生能力，并在浏览器环境优雅降级。

**这是「桌面端 = Web 端套壳 + 一些功能」这一定位的技术承载点**：有了边界层，Web 前端任务不需要懂 Wails，Desktop 任务不需要改业务组件。

## Context

`.trellis/spec/frontend/routing-platform-guidelines.md` 已经定义了这套边界层的**规范**：

> Desktop 专属能力必须通过边界 hook、service 或 `src/lib/desktop` 一类模块封装，业务组件不直接读取 Wails 全局对象或调用 runtime API。

但 **`apps/web/src/lib/desktop` 目录从未创建**（当前 `src/lib/` 只有 `api/`、`dayjs.ts`、`utils.ts`）。规范存在而实现缺失，导致后续每个 Desktop 能力都会各自发明接入方式。

- 现有 Wails 侧代码：`services/taskdaemon-go/internal/desktop/`
- 错误码前缀：`DESKTOP_*`（路线图 §9.1 已分配）
- query key 前缀：`desktopKeys`（路线图 §9.2 已分配）

## Requirements

### R1 Capability 模型（契约核心）

- capability 以**具名常量**标识，不用自由字符串
- 每个 capability 的返回形状统一：至少包含「是否可用」「不可用原因」「调用方法」
- 「不可用」区分至少两种原因：非 Desktop 环境、Desktop 环境但该能力不支持（如某平台无托盘）
- capability 检测结果可缓存，但平台能力变化时可失效

### R2 Web fallback 语义

- 浏览器环境下每个 capability 必须返回明确的不可用状态，**不抛异常、不崩溃**
- 提供 noop 实现，调用不可用能力时返回可解释的失败结果而非静默成功
- 业务组件只根据返回状态渲染禁用态或隐藏入口，不做平台判断

### R3 边界 hook

- 提供 `useDesktopCapability(name)` 一类的 React hook 作为业务组件的唯一入口
- hook 返回值形状稳定，供 D2/D3/W 轨任务直接消费
- 提供环境判断的单一来源（是否运行在 Wails 壳内），业务代码不重复判断

### R4 Wails binding 注册模式（Go 侧）

- 定义 Go 侧能力注册的统一模式：新增一个 Desktop 能力需要做哪几步
- 注册点集中，D2/D3 只追加自己的注册，不改注册框架
- Go 侧能力不可用时返回结构化错误（`DESKTOP_*` 前缀），不 panic

### R5 首个样板能力

- 用一个**最小真实能力**验证整条链路端到端可用（建议：查询 Desktop 环境信息，如平台、版本、壳能力清单）
- 该能力同时验证：Go binding → 前端 capability → hook → 组件消费 → 浏览器降级
- 样板代码结构会被 D2/D3 直接模仿，必须可读

### R6 契约文档化（C-5 冻结产物）

- capability 命名规则、返回形状、fallback 语义、Go 侧注册步骤写入 `.trellis/spec/frontend/routing-platform-guidelines.md`
- 冻结完成后回写路线图 §9 表格的产物路径

### R7 测试要求

- capability 逻辑、fallback 分支、hook 返回形状有单元测试
- 按 `.trellis/spec/frontend/quality-guidelines.md`：测业务逻辑与通用函数，不测页面结构与样式

## Acceptance Criteria

- [ ] `apps/web/src/lib/desktop/` 目录落地，导出稳定的 capability 接口
- [ ] capability 以具名常量标识，非法名称在类型层被拒绝
- [ ] 每个 capability 返回形状统一，含可用性与不可用原因
- [ ] 不可用原因至少区分「非 Desktop 环境」与「平台不支持」
- [ ] 浏览器环境下调用任意 capability 不抛异常、不崩溃
- [ ] `useDesktopCapability` hook 可用，返回形状稳定
- [ ] 环境判断只有单一来源，全仓库无第二处 Wails 全局对象读取
- [ ] Go 侧 binding 注册模式定义完成，新增能力步骤写入文档
- [ ] Go 侧能力错误返回 `DESKTOP_*` 结构化错误，不 panic
- [ ] 样板能力端到端可用：Wails 壳内返回真实数据
- [ ] 样板能力在浏览器内优雅降级：UI 显示禁用态而非报错
- [ ] capability 逻辑与 fallback 分支有单元测试
- [ ] C-5 契约写入 `routing-platform-guidelines.md`，路线图 §9 产物路径已回写
- [ ] `pnpm typecheck`、`pnpm lint`、`pnpm --filter @taskdaemon/web test`、`pnpm test:go` 全绿

## Out of Scope

- Desktop 原生通知（归 D2）
- 托盘、自启动、单实例、窗口状态（归 D3）
- 拆分 `apps/desktop` 独立前端（路线图 §4.2 已明确不做）
- 任何业务功能

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**产出** | C-5 Desktop capability 契约 |
| 本任务**消费** | 无 |
| 下游 | D2（通知 sink）、D3（原生增量能力）、W1/W2/W3（需要禁用态渲染时） |

**冻结即解锁**：C-5 写入 spec 后 D2/D3 即可开工。

## Definition of Done

- Acceptance Criteria 全部勾选
- 路线图 §7.4 状态与 §9 产物路径已回写
- D2/D3 可以只读 spec 就知道如何新增一个 Desktop 能力

## Notes

- 本任务是**契约任务 + 样板任务**，接口稳定性与代码可读性优先于能力数量
- 严格遵守既有禁止模式：不为 Desktop 和 Web 分叉两套业务组件
- 样板能力选最小的那个，本任务的价值在边界层不在能力本身
