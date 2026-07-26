# G0 Agent 运行时技术 spike

> **轨**：Track G ｜ **依赖**：无（W1 波次，可立即开工）
> **性质**：研究型任务。**只写研究产物到 `research/`，不改仓库代码。**

## Goal

在 Agent 线投入实现之前，用实测回答五个决定后续任务规模的问题：

1. **Pi 与 OMP 各自提供了什么** —— 从而确定 gateway 的最小必要规模
2. **两者能力差异有多大** —— 决定 `AgentRuntime` 抽象是否可行、抽到哪一层
3. 各自的接入方式（SDK vs CLI spawn）与**冷启动延迟**实测
4. gateway 的持久化、runtime、工具链、部署形态怎么选
5. Pi WebUI 的核心功能清单与耦合点

**没有本任务的结论，G1 的契约无法冻结，G2/G3/G4 的规模无法判断。**

## Context

- 路线图 §5.2 已决策：**初版即 Pi + OMP 两个 adapter 并存**，不是「先做 Pi 以后再抽象」
- 路线图 §5.3 定义了两个正交维度：**接入方式**（延迟）× **Profile**（coding/general）
- 路线图 §5.4 定义了 `RuntimeCapabilities` 能力协商模型，本任务负责实测填表
- 路线图 §5.5 指出企业场景多数**非 coding**，coding 特化项必须可关闭
- 路线图 §5.8 列出「runtime 与外部网关都不做」的七项职责，本任务逐条实测
- 路线图 §5.9 已决策 Pi WebUI **参考重写并简化**，本任务只产出核心功能清单

## Requirements

### R1 Pi 与 OMP 能力盘点（**首要产出**）

**对 Pi 和 OMP 各做一份**，每项给出「提供 / 不提供 / 部分提供」的**实测结论**与验证方式：

| 能力 | 关键问题 |
|---|---|
| agent loop | 是否完全由 runtime 承担，gateway 是否需要介入 |
| MCP 工具接入 | 配置方式、能否运行时增删、能否按会话隔离 |
| **会话持久化** | 状态存在哪里？进程重启后能否恢复？跨机器能否迁移 |
| **多会话并发** | 一进程多会话还是一会话一进程 |
| 模型切换 | 运行时可切吗？粒度是会话级还是请求级 |
| steering / abort | 语义是什么？abort 后会话是否可继续 |
| 消息历史 | 能否分页读取？格式是什么 |
| **工具调用可见性** | 事件流里能拿到 tool 名称、入参、结果吗（AG-UI 时间线的前提） |
| **thinking / 中间态** | 是否暴露推理过程事件 |
| **非 coding 场景可用性** | 不绑定 workspace/cwd 时能否正常工作 |
| 错误与重试 | 模型报错、工具失败的事件形态 |

**产出**：`research/pi-capability-matrix.md`、`research/omp-capability-matrix.md`

### R2 OMP 基础核实

- 全称、仓库地址、许可证、维护活跃度
- 是否提供非交互 / RPC 模式（决定能否被程序驱动）
- 是否提供 SDK（决定 `attachMode` 与 `coldStartCost`）
- 事件流形态与 Pi 的差异

**产出**：并入 `research/omp-capability-matrix.md`

### R3 能力差异与抽象可行性分析（**本任务的核心判断**）

- 把 Pi 与 OMP 的能力矩阵**并排对照**，逐项标注：一致 / 语义不同 / 一方缺失
- 对每项差异回答：**能否被 `RuntimeCapabilities` 声明抹平？** 还是需要抽象层特殊处理？
- 填出两份 `RuntimeCapabilities`（路线图 §5.4 的字段）
- 识别**抽象层的危险信号**：是否存在某项能力，两者语义差异大到无法用同一接口表达？若有，给出处置建议（缩小接口 / 拆分方法 / 该能力不纳入抽象）
- 明确回答：**当前的 `AgentRuntime` 接口草案（路线图 §5.2）是否够用？需要增删什么？**

**产出**：`research/runtime-abstraction-feasibility.md`

> 这份文档是 G1 定稿接口的**直接依据**，也是本任务相对早期版本新增的最重要产出。

### R4 接入方式与延迟实测

对每个 runtime 的每种可用接入方式：

- 是否可用、如何启动、版本
- **冷启动延迟实测**：从发起创建会话到可接受第一个 prompt 的耗时（多次取分布，不是单次）
- **首字节延迟**：发出 prompt 到收到第一个事件的耗时
- 资源占用：单会话内存、进程数
- 协议异常处理实测：**半包、粘包、超长行、非法 JSON 帧、进程中途退出**

结论必须支撑 `coldStartCost: 'low' | 'high'` 的判定，并给出**池化策略建议**（是否需要预热、空闲保留多久）。

**产出**：`research/runtime-attach-and-latency.md` + 各路径的最小可运行验证脚本（`research/scripts/`）

### R5 gateway 最小规模判定（R1+R3 的推论）

基于能力盘点，对路线图 §5.8 的**七项职责**逐条判定：

- 该职责 runtime 是否已覆盖 → 覆盖则 gateway **直接透传，不封装**
- 未覆盖 → 说明 gateway 需要做什么，工作量量级（S / M / L）

特别需要明确回答：

- **gateway 是否需要自己的 thread/run 存储？**（两个 runtime 的持久化能力可能不同，取交集还是按 adapter 差异化处理？）
- **进程池/会话路由的复杂度**，含并发上限、超时回收、崩溃恢复，且需按 `coldStartCost` 区分策略
- **多 adapter 抽象层**本身的工作量

**产出**：`research/gateway-scope-decision.md`，含「G2 做什么 / 不做什么」表

### R6 运行时与工具链选型

- Hono 跑 Node 还是 Bun：结合现有 pnpm 11 / TypeScript 6 / Biome / vitest 工具链
- 若某个 runtime 的 SDK 只支持特定 JS runtime，这会约束选型 —— 必须核实
- 能否复用现有 lint / typecheck / test 命令
- 与 `pnpm-workspace.yaml` 的集成方式

**产出**：`research/runtime-toolchain.md`

### R7 持久化选型

- 复用 taskdaemon 的 PostgreSQL/SQLite？独立库？还是不需要（取决于 R5）
- 若需要：查询层选型、migration 策略、与 Go 侧 Ent 的边界
- 数据是否需要按 `X-Tenant-Id` 分区（C-4 的输入）

**产出**：`research/persistence-decision.md`

### R8 部署形态

- agent-gateway 是独立进程、被 taskdaemon 拉起、还是完全独立部署
- **多 runtime 并存的部署影响**：两个 runtime 的二进制/依赖如何分发，是否都必须存在
- Desktop 模式下的形态
- 开发期启动方式，与现有 `pnpm dev:*` 脚本的关系
- 生产期与外部网关的拓扑（呼应路线图 §4.3）

**产出**：`research/deployment-shape.md`

### R9 Pi WebUI 核心功能拆解

**前提**：路线图 §5.9 已决策**参考重写并简化**，本项不重开选型。

- 核实开源 Pi WebUI 的实际仓库地址与许可证
- 拆解核心交互形态：会话列表、消息流、工具调用展示、输入区、模型选择、中断控制等
- 每项标注「G4 必要 / G5 增强 / 本项目不需要」
- **识别它与 Pi 的耦合点**：用了哪些 Pi 私有接口 —— 这些正是 gateway 需要在标准协议下补齐的部分
- **额外**：识别它作为**单 runtime 单 profile UI** 的假设 —— 哪些地方硬编码了「一定有 workspace」「一定支持 steering」之类，这些正是我们必须做成能力驱动的地方
- 记录值得借鉴的交互细节与明确不采纳的复杂度

**产出**：`research/pi-webui-teardown.md`，含「核心功能 → G4/G5/不做」分级表 + Pi 耦合点清单 + 单 runtime 假设清单

### R10 协议版本锚点

- 实测时点的 AG-UI、Agent Protocol、MCP、Pi、OMP 版本号与快照日期
- 回写路线图 §5.1 版本锚点表的 Pi 与 OMP 两行（其余由 G1 填）

## Acceptance Criteria

### 能力盘点

- [ ] `research/pi-capability-matrix.md` 覆盖 R1 全部能力项，每项有实测结论与验证方式，**无「推测」「文档上说」字样**
- [ ] `research/omp-capability-matrix.md` 同上
- [ ] OMP 的全称、仓库、许可证、维护活跃度已核实
- [ ] OMP 是否提供非交互/RPC 模式已确认
- [ ] 两者的「非 coding 场景可用性」均已实测

### 抽象可行性（核心）

- [ ] `research/runtime-abstraction-feasibility.md` 有 Pi/OMP 并排对照表，逐项标注一致/语义不同/缺失
- [ ] 每项差异都回答了「能否被 capabilities 声明抹平」
- [ ] 两份 `RuntimeCapabilities` 已填出完整值
- [ ] 识别出的抽象危险信号有处置建议
- [ ] **明确回答了 `AgentRuntime` 接口草案是否够用、需要增删什么**

### 接入与延迟

- [ ] `research/runtime-attach-and-latency.md` 覆盖每个 runtime 的每种可用接入方式
- [ ] **冷启动延迟有多次测量的分布数据**，非单次
- [ ] 首字节延迟有数据
- [ ] `coldStartCost` 判定有数据支撑
- [ ] 给出了池化策略建议（是否预热、空闲保留时长）
- [ ] 协议异常（半包/粘包/超长行/非法帧/进程退出）均已实测并记录现象
- [ ] `research/scripts/` 有各路径的最小可运行验证脚本

### 规模与选型

- [ ] `research/gateway-scope-decision.md` 对**七项职责**逐条给出透传/自建判定与工作量量级
- [ ] 明确回答「gateway 是否需要自己的 thread/run 存储」及两 runtime 差异如何处理
- [ ] 明确回答会话↔进程编排复杂度，且按 `coldStartCost` 区分策略
- [ ] 多 adapter 抽象层的工作量已估计
- [ ] `research/runtime-toolchain.md` 给出 Node/Bun 结论，且已核实是否受 SDK 约束
- [ ] `research/persistence-decision.md` 给出结论（含「不需要」也是合法结论）
- [ ] `research/deployment-shape.md` 覆盖开发期、生产期、Desktop 三种形态，含多 runtime 分发问题

### WebUI 与收尾

- [ ] `research/pi-webui-teardown.md` 有功能分级表、Pi 耦合点清单、**单 runtime 假设清单**
- [ ] 路线图 §5.1 的 Pi 与 OMP 两行版本锚点已填
- [ ] 若任一假设被证伪，路线图对应章节已修订并在 §16 追加修订记录
- [ ] `research/samples/` 有两个 runtime 的原始事件流存档（→ G3 的测试 fixture）
- [ ] **仓库代码零改动**（`git status` 除 `research/` 与路线图外无变更）

## Out of Scope

- 任何 `services/agent-gateway` 或 `apps/agent-web` 的实现代码
- 契约文档的正式冻结（归 G1）
- Pi WebUI 的 fork 或依赖引入（路线图 §5.9 已排除）
- 企业鉴权/审计方案调研（路线图 §6 已裁定移出仓库）
- 第三个 runtime 的调研（路线图 §12）

## Definition of Done

- 上述 Acceptance Criteria 全部勾选
- G1 可以基于本任务结论直接开始契约冻结，无需回头补测
- G3 可以直接使用 `research/samples/` 与 `research/scripts/`
- 若结论推翻了路线图假设，路线图已同步修订

## Notes

- **本任务允许得出「某能力不需要做」「某抽象不可行」的结论**，这正是它的价值
- **两个 runtime 的差异是资产不是负担**：差异越大，越能暴露抽象设计的问题，越早发现越便宜
- 验证脚本可以粗糙，但结论必须是实测的
- 遇到文档与实测不符时以实测为准并记录差异
- 延迟测量要多次取分布 —— 单次数据在冷启动场景下毫无意义
