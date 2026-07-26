# G1 Agent 对外契约冻结与共享包落地

> **轨**：Track G ｜ **依赖**：**G0**（技术 spike 结论）
> **契约冻结点**：**C-3 Agent 对外契约** + **C-4 信任边界契约**。G2、G3、G4、G5、G6 全部消费本任务产物。

## Goal

把 G0 的实测结论固化为**可被多方消费的契约**：对外 REST 子集、AG-UI 事件映射、`AgentRuntime` 接口、信任边界头与事件 schema，并落地共享 TS 类型包。

**本任务不写运行时实现。** 交付的是类型、文档与边界定义。

## Context

- 依据：G0 的 `research/` 全部产物，特别是 `runtime-abstraction-feasibility.md`、两份能力矩阵与 `gateway-scope-decision.md`
- 路线图 §5.8 已定规模决策规则：**runtime 已提供的一律透传，不做二次封装**
- 路线图 §6 已裁定企业能力归属：仓库内只留外部网关做不到的钩子
- 独占路径：`packages/**`（新建）、`docs/agent-contracts/**`（新建）
- `pnpm-workspace.yaml` 已声明 `packages/*` glob，目录不存在（路线图 §10 的 ROI 拐点在此触发）

## Requirements

### R1 Agent Protocol 子集冻结（C-3 之一）

- 明确实现哪些资源与操作：先 threads / runs / stream / cancel / messages，其余不做
- 每个端点的路径、方法、请求体、响应体、错误形态确定
- **对齐 G0 结论**：runtime 已原生提供的能力直接透传，不在协议层重新建模
- 分页、排序、过滤的约定统一
- 明确不实现的部分（Store 全量、全部 stream 原语）并说明原因

### R2 AG-UI 事件映射表（C-3 之二）

- 列出对外发出的全部 AG-UI 事件类型
- 每个事件对应的 `RuntimeEvent` 来源与转换规则
- **标注 G0 实测中任一 runtime 不提供的事件**（如 thinking、tool 入参），说明是降级、模拟还是不支持
- 事件顺序与生命周期约定：一次 run 的事件序列长什么样
- 断线重连的事件补发语义（是否有游标、能否续传）

### R3 `AgentRuntime` 接口冻结（C-3 之三）

- 接口方法签名最终确定（路线图 §5.2 是草案，本任务定稿）
- **定稿依据是 G0 的 `runtime-abstraction-feasibility.md`**，不是凭空设计
- `RuntimeEvent`、`SessionOpts`、`RuntimeState`、`PromptInput` 等类型定义完整
- **中立性硬要求**：接口中不得出现任何 Pi 或 OMP 特有的概念、命名、语义
- 可选方法与必选方法明确区分，缺失必须可通过 capabilities 发现
- 接口设计必须让「换掉任一 adapter 不改前端协议」成立

### R3.5 `RuntimeCapabilities` 能力协商冻结（C-3 之四，**本轮新增**）

路线图 §5.4 定义了能力协商模型，本任务定稿：

- 字段清单与取值枚举定稿（`profile` / `attachMode` / `coldStartCost` / `steering` / `thinking` / `toolCallDetail` / `modelSwitch` / `workspaceBinding` / `sessionPersistence`，按 G0 结论增删）
- 每个字段的语义、默认值、前端应如何降级，逐条文档化
- **对外透出方式**：前端通过哪个端点拿到当前会话的 capabilities
- **两份实例值**：Pi 与 OMP 各自的 capabilities 作为契约的参考实现写入文档
- **同构说明**：与 Track D 的 C-5 Desktop capability 是同一模式，文档中应指出这一点并共用词汇

### R3.6 非 coding 场景契约（路线图 §5.5）

- `workspaceBinding: false` 的会话在协议层如何表达（`SessionOpts` 中 workspace 是可选的）
- coding 特化的事件（文件变更、diff）在 general profile 下的语义：不发送，而非发空
- 前端按 profile 分支渲染的约定

### R4 信任边界契约（C-4）

- **注入头**：名称、语义、缺失时的行为（`X-Auth-Subject` / `X-Tenant-Id` / `X-Trace-Id` 或 G0 确定的其他命名）
- **`PrincipalResolver` 接口**：签名、默认 dev 实现的行为、生产切换方式
- **audit 事件 schema**：谁、何时、对哪个 workspace、调了哪个 tool、结果如何
- **usage 事件 schema**：token 数、模型、cost 相关字段、归属主体
- 明确写出：哪些由外部网关承担（SSO、多租户管理、审计存储、配额执行、计费），本仓库只产出事件不做聚合
- 无外部网关时的开发期默认行为与启动告警要求

### R5 共享包落地（路线图 §10）

- 创建 `packages/agent-protocol/`：Agent Protocol DTO + AG-UI 事件类型 + `AgentRuntime` 接口
- **types only，零运行时依赖**
- 接入现有工具链：可被 `pnpm typecheck`、`pnpm lint` 覆盖
- 三方（agent-gateway / agent-web / 未来消费者）能正确 import
- **不做**：不拆 `packages/ui`、`packages/utils`

### R6 版本锚点回填

- 回写路线图 §5.1 的 AG-UI / Agent Protocol / MCP 三行：参考版本、快照日期、实现子集
- Pi 与 OMP 两行由 G0 填，本任务核对一致性

### R7 契约文档

- `docs/agent-contracts/` 下形成可被外部团队（含未来 Java 网关）阅读的文档
- 至少包含：REST 子集说明、AG-UI 事件映射表、`trust-boundary.md`
- 冻结完成后回写路线图 §9 表格的产物路径

## Acceptance Criteria

- [ ] Agent Protocol 子集端点清单确定，每个端点的请求/响应/错误形态完整
- [ ] 明确列出不实现的部分及原因
- [ ] AG-UI 事件映射表完整，每个事件有来源与转换规则
- [ ] **Pi 与 OMP 各自不提供的事件均已标注处置方式**（降级/模拟/不支持）
- [ ] 一次 run 的事件序列有明确定义
- [ ] 断线重连的补发语义已定义
- [ ] `AgentRuntime` 接口签名定稿，相关类型定义完整
- [ ] **接口定稿依据 G0 的 `runtime-abstraction-feasibility.md`**，采纳/否决其建议均有说明
- [ ] **接口中无 Pi 或 OMP 特有概念**（逐项核对）
- [ ] 可选方法与必选方法明确区分，缺失可通过 capabilities 发现
- [ ] 「换掉任一 adapter 不改前端协议」的成立性有论证
- [ ] `RuntimeCapabilities` 字段清单与取值枚举定稿
- [ ] 每个 capability 字段有语义、默认值、前端降级方式的说明
- [ ] capabilities 的对外透出端点已定义
- [ ] **Pi 与 OMP 两份 capabilities 实例值写入文档**
- [ ] 文档中指出与 C-5 Desktop capability 的同构关系并共用词汇
- [ ] `workspaceBinding: false` 会话在协议层可表达
- [ ] general profile 下 coding 特化事件的语义已定义（不发送，非发空）
- [ ] 前端按 profile 分支渲染的约定已写入契约
- [ ] 注入头名称与语义确定，缺失时行为明确
- [ ] `PrincipalResolver` 接口定稿，dev 默认实现行为明确
- [ ] audit 事件 schema 完整
- [ ] usage 事件 schema 完整
- [ ] 外部网关职责与本仓库职责的分界写入 `trust-boundary.md`
- [ ] 开发期默认行为与启动告警要求已定义
- [ ] `packages/agent-protocol/` 落地，types only 零运行时依赖
- [ ] 共享包被 `pnpm typecheck`、`pnpm lint` 覆盖
- [ ] 至少一个消费方能正确 import（用最小验证证明）
- [ ] 路线图 §5.1 版本锚点表全部填写
- [ ] `docs/agent-contracts/` 文档完成，含 REST 子集、事件映射表、`trust-boundary.md`
- [ ] 路线图 §9 的 C-3 / C-4 产物路径已回写
- [ ] 契约与 G0 结论一致，runtime 已提供的能力未被二次封装
- [ ] `pnpm typecheck`、`pnpm lint` 全绿

## Out of Scope

- gateway 运行时实现（归 G2）
- Runtime Adapter 层与各 adapter 实现（归 G3）
- 前端实现（归 G4/G5）
- A2A / Agent Card（路线图 §12 已排除）
- SSO / 多租户 / 审计存储 / 配额执行（路线图 §6 已移出仓库）
- 拆 `packages/ui`、`packages/utils`

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**消费** | G0 全部研究产物 |
| 本任务**产出** | **C-3**（Agent 对外契约）、**C-4**（信任边界契约） |
| 下游 | G2、G3、G4、G5、G6，以及未来的外部 Java 网关 |
| 共享文件 | `pnpm-workspace.yaml`（glob 已覆盖，通常无需改） |

**冻结即解锁**：C-3 写入后 G2 / G3 / G4 可三线并行开工。

## Design / Implement 补齐条件

`design.md` 与 `implement.md` **待 G0 结论产出后补**。原因：契约的规模与形状完全取决于 G0 对两个 runtime 的能力盘点与抽象可行性结论；G0 之前写设计是凭空想象。

## Definition of Done

- Acceptance Criteria 全部勾选
- 路线图 §5.1、§7.5、§9 三处已回写
- G2 / G3 / G4 可仅凭契约文档与类型包开始并行开发

## Notes

- 本任务是 **Track G 的总闸门**，契约质量直接决定后面三个任务能否真正并行
- **克制是关键**：能透传就不封装，能少一个端点就少一个。协议子集越小，实现和维护成本越低
- **中立性是第二关键**：每定一个字段就问「这是通用概念还是某个 runtime 的特性」。后者归 capabilities 或 adapter 内部
- `RuntimeCapabilities` 与 Track D 的 `C-5` 是同一个模式 —— 复用词汇能显著降低团队认知负担
- `trust-boundary.md` 要写给「未来接 Java 网关的人」看，假设读者不了解本仓库
- 若 G0 结论与路线图假设冲突，先改路线图再冻结契约，不要让两份文档打架
