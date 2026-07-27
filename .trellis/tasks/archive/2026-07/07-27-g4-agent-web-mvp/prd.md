# G4 最小 agent-web

> **轨**：Track G ｜ **依赖**：**C-3 冻结**（G1）
> **性质**：新建独立前端应用 `apps/agent-web`。**Track G 的重心在这里**（路线图 §7.5）。

## Goal

落地最小可用的 Agent 会话前端：会话列表、流式消息、工具调用时间线。

**设计来源**：参考开源 Pi WebUI 的核心功能，**简化重写**（路线图 §5.9 已决策）。

## Context

- 路线图 §5.9 已决策：**参考重写并简化，不 fork、不作为依赖**
  - 理由：fork 会继承全部上游复杂度与历史包袱；参考重写代码量小、可读性高、后续改造成本低
- G0 产出 `research/pi-webui-teardown.md`，含**核心功能分级表**（G4 必要 / G5 增强 / 不需要）、Pi 耦合点清单与**单 runtime 假设清单**
- 契约来自 C-3：Agent Protocol 子集 + AG-UI 事件类型 + `packages/agent-protocol` 类型包
- 判断标准（路线图 §5.9）：**宁可少一个功能，不要多一层看不懂的抽象**
- query key 前缀：`agentKeys`（路线图 §9.2 已分配）
- 独占路径：`apps/agent-web/**`（新建）

**与 `apps/web` 的关系**：两个独立应用，不共用业务代码。共享的只有 `packages/agent-protocol` 的类型。

## Requirements

### R1 应用脚手架

- 新建 `apps/agent-web`，技术栈与 `apps/web` 对齐（Vite + React + TypeScript）
- 接入 pnpm workspace，被根 `pnpm typecheck` / `pnpm lint` 覆盖
- 有独立 `test` script
- 遵守 `.trellis/spec/frontend/` 的目录结构、组件、状态管理规范
- **不搬运 Pi WebUI 的目录结构与状态管理方案**，除非本仓库规范已认可

### R2 会话列表

- 列出会话（thread），展示标题、最近活动时间、状态
- 创建新会话
- 切换会话
- 删除会话（需确认）
- 分页或滚动加载，按 C-3 的分页约定

### R3 消息流

- 展示会话历史消息，区分用户与 agent
- 发送消息，流式接收响应
- 流式渲染平滑，不因高频事件卡顿
- 消息内容的基础渲染（至少代码块与换行正确）
- 长会话滚动性能可接受，滚动位置行为合理（新消息到达时的自动滚动策略明确）

### R4 工具调用时间线

- 展示 agent 调用了哪些 tool、入参、结果
- 工具调用与消息在时间线上顺序正确
- 调用中/成功/失败三种状态可区分
- 长结果可折叠，不撑爆页面
- **依赖 C-3 的事件映射**：若 G0 证明某 runtime 不提供 tool 入参，按 C-3 的降级方式展示，不伪造数据

### R5 运行控制

- 发送中可中断（cancel）
- 中断后 UI 状态正确回落，不卡在「进行中」
- 运行状态可见（空闲 / 运行中 / 出错）

### R6 连接与错误

- 流式连接断开时有明确提示与重连入口
- **不做**自动断线续传（归 G5）
- 错误展示包含 traceId
- loading / empty / error 状态齐全

### R7 能力驱动渲染（**本轮新增，硬约束**）

前端**不得硬编码任何 adapter 的能力假设**（路线图 §5.4）：

- 从 gateway 读取当前会话的 `RuntimeCapabilities`，按声明渲染
- 能力不可用时展示**禁用态与原因**，不隐藏、不伪造
- 至少覆盖这些分支：`steering` 关闭时不显示 steering 入口、`thinking` 关闭时不显示推理区、`toolCallDetail: 'name-only'` 时只显示工具名不显示入参、`modelSwitch: 'none'` 时不显示模型选择器
- **profile 分支**（路线图 §5.5）：`workspaceBinding: false` 时不显示工作区相关 UI，会话仍能正常工作
- 可选：会话创建时可选择 adapter（若 gateway 暴露多个）

> **同构提示**：这与 Track D 的 `useDesktopCapability` 是同一模式。参考其词汇与心智模型。

> **G0 的「单 runtime 假设清单」是本项的直接检查表** —— Pi WebUI 硬编码的每一处假设，都是这里必须做成能力驱动的地方。

### R8 简化纪律（本任务的核心约束）

- 每实现一个功能前，对照 G0 的功能分级表确认它是「G4 必要」
- 标为「G5 增强」或「不需要」的功能**本任务不做**
- 不引入 Pi WebUI 作为依赖
- 不复制看不懂的抽象层；宁可写直白的代码

### R9 测试

- 测业务逻辑：事件流到 UI 状态的归约、时间线排序、状态机转换、query 缓存失效
- **能力驱动分支有测试**：喂不同的 capabilities，断言 UI 分支正确
- **不测**页面结构与 CSS
- 事件流归约逻辑是本应用最核心的逻辑，测试要充分

## Acceptance Criteria

- [ ] `apps/agent-web` 落地，技术栈与 `apps/web` 对齐
- [ ] 接入 pnpm workspace，被根 `pnpm typecheck` / `pnpm lint` 覆盖
- [ ] 有独立 `test` script
- [ ] 遵守 `.trellis/spec/frontend/` 规范，未搬运上游目录结构
- [ ] 会话列表可用：展示、创建、切换、删除（删除有确认）
- [ ] 分页或滚动加载符合 C-3 约定
- [ ] 消息历史正确展示，用户与 agent 可区分
- [ ] 发送消息后流式接收响应
- [ ] 流式渲染平滑，高频事件不卡顿
- [ ] 代码块与换行渲染正确
- [ ] 长会话滚动性能可接受，自动滚动策略明确
- [ ] 工具调用时间线展示 tool 名称、入参、结果
- [ ] 工具调用与消息顺序正确
- [ ] 调用中/成功/失败三态可区分
- [ ] 长结果可折叠
- [ ] runtime 不提供的字段按 C-3 降级展示，**未伪造数据**
- [ ] **UI 按 `RuntimeCapabilities` 渲染，无硬编码能力假设**（用测试证明）
- [ ] `steering` / `thinking` / `toolCallDetail` / `modelSwitch` 四个分支均正确处理
- [ ] 能力不可用时展示禁用态与原因，未隐藏、未伪造
- [ ] `workspaceBinding: false` 时不显示工作区 UI，会话仍正常工作
- [ ] **G0 的「单 runtime 假设清单」逐项已处理为能力驱动**
- [ ] 两个 adapter 各跑一遍，UI 均正确（差异只体现在能力分支上）
- [ ] 发送中可中断，中断后 UI 状态正确回落
- [ ] 运行状态可见
- [ ] 断连有提示与重连入口
- [ ] 错误展示包含 traceId
- [ ] loading / empty / error 状态齐全
- [ ] **实现范围与 G0 功能分级表的「G4 必要」项一致**，无越界实现
- [ ] `package.json` 中无 Pi WebUI 依赖
- [ ] 事件流归约逻辑有充分单元测试
- [ ] 能力驱动分支有单元测试（喂不同 capabilities 断言 UI 分支）
- [ ] `pnpm typecheck`、`pnpm lint`、`pnpm --filter @taskdaemon/agent-web test` 全绿
- [ ] 与 G2 + G3 联调通过

## Out of Scope

- thinking 展示、HITL、steering（归 G5）
- 工作区绑定、文件/diff 预览（归 G5）
- 断线自动续传（归 G5）
- 会话命名、搜索、fork、归档策略（归 G5）
- 与 `apps/web` 的功能整合
- 移动端适配
- 主题定制

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**消费** | **C-3**（Agent Protocol 子集 + AG-UI 事件类型 + `packages/agent-protocol`）、G0 的 `pi-webui-teardown.md` 功能分级表 |
| 本任务**产出** | `agentKeys` query key 命名空间；agent-web 应用骨架（G5 在此之上扩展） |
| 共享文件 | 根 `package.json`（追加 script）、`pnpm-workspace.yaml`（glob 已覆盖） |

**并行**：与 G2 / G3 同时开工，用 mock 事件流开发，三方就绪后联调。

## Design / Implement 补齐条件

`design.md` 与 `implement.md` **待 C-3 冻结 + G0 功能分级表产出后补**。原因：实现范围由分级表界定，事件类型由 C-3 界定，两者缺一都会做错范围。

## Definition of Done

- Acceptance Criteria 全部勾选
- 与 G2 + G3 联调通过
- 路线图 §7.5 状态已回写
- G5 可在本应用基础上扩展，无需重构

## Notes

- **简化是本任务的第一原则**：路线图 §5.9 明确「宁可少一个功能，不要多一层看不懂的抽象」
- **能力驱动是第二原则**：Pi WebUI 是单 runtime 单 profile 的 UI，它的硬编码假设**不能照搬**。G0 的「单 runtime 假设清单」就是这里的检查表
- 参考 Pi WebUI 时，看的是**交互形态**，不是代码结构。它的抽象服务于它的场景，不一定适合这里
- **事件流到 UI 状态的归约**是本应用的技术核心，值得单独设计并充分测试
- 不要伪造 runtime 不提供的数据（如假的 tool 入参），宁可显示「不可用」
- 能力协商与 Track D 的 `useDesktopCapability` 同构，复用其词汇能降低认知负担
