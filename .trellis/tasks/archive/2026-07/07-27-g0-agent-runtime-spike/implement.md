# G0 Agent 运行时技术 spike — 执行计划

## 前置

- [ ] 读路线图 §5 全节（本任务负责证伪或确认其中假设）
- [ ] 读路线图 §6（企业能力已裁定移出仓库，不在调研范围）
- [ ] 创建 `research/`、`research/samples/{pi,omp}/`、`research/scripts/`
- [ ] 从 `dev` 切分支 `feat/g0-agent-runtime-spike`

> ⚠️ **硬约束**：全程只写 `research/` 与路线图。任何 `services/` 或 `apps/` 的改动都是范围失控。

## 阶段 0 · OMP 基础核实（**先做，因为它可能否定整个计划**）

- [ ] 核实 OMP 全称、仓库地址、许可证、维护活跃度
- [ ] **是否提供非交互 / RPC 模式？** 能否被程序驱动？
- [ ] 是否提供 SDK？
- [ ] 装起来，跑通一次最简单的交互

**🚨 门（阻塞性）**：若 OMP **无法被程序驱动**，立即停下来回路线图重估「初版两个 adapter」的决策 —— 要么换一个第二 adapter，要么退回单 adapter 方案。**不要带着这个问题继续往下做。**

## 阶段 1 · 接入方式与延迟（R4）

对每个 runtime 的每种可用接入方式：

- [ ] Pi SDK 路径：安装 `@earendil-works/pi-coding-agent`，记录版本与 `AgentSession` 实际签名
- [ ] Pi CLI 路径：`pi --mode rpc`，记录 JSONL 帧格式
- [ ] OMP 可用路径（按阶段 0 结论）
- [ ] 各路径最小可运行脚本 → `research/scripts/`
- [ ] **冷启动延迟**：每条路径重复 ≥10 次，记录 min / median / p95
- [ ] 分别测「已预热」与「完全冷启动」
- [ ] **首字节延迟**：prompt → 第一个事件
- [ ] 资源占用：单会话内存、进程数
- [ ] 协议异常实测（CLI 路径），每种记录观察到的现象：
  - [ ] 半包（分两次写入一帧）
  - [ ] 粘包（一次写入多帧）
  - [ ] 超长行
  - [ ] 非法 JSON 帧
  - [ ] 进程中途被杀
- [ ] 写 `research/runtime-attach-and-latency.md`，含 `coldStartCost` 判定与**池化策略建议**

**门**：延迟数据必须是分布，不是单次。

## 阶段 2 · 能力盘点（R1、R2，首要产出）

按 design §3.1 的实验表，**对 Pi 和 OMP 各做一遍**。每项存原始事件流。

Pi：

- [ ] 会话持久化（起→发→杀→重启→恢复，记录状态存储位置）
- [ ] 多会话并发（一进程多会话 vs N 进程？内存曲线）
- [ ] 工具调用可见性（tool 名/入参/结果）
- [ ] thinking 事件
- [ ] **abort 语义**（真停 vs 只断监听，看 CPU/token）
- [ ] 模型切换（粒度、是否需重建、历史保留）
- [ ] 消息历史（分页？格式？）
- [ ] **非 coding 可用性**（不给 workspace 起会话）
- [ ] 错误形态（模型报错、工具失败）
- [ ] agent loop / MCP 接入覆盖度
- [ ] 事件流存档 → `research/samples/pi/*.jsonl`
- [ ] 写 `research/pi-capability-matrix.md`

OMP：

- [ ] 同上全部项目
- [ ] 事件流存档 → `research/samples/omp/*.jsonl`
- [ ] 写 `research/omp-capability-matrix.md`（含阶段 0 的基础核实结果）

**门**：两份表都无「推测」「文档上说」字样。有则回去做实验。

## 阶段 3 · 抽象可行性分析（R3，★ 核心）

- [ ] 填 design §3.2 的**并排对照表**，逐项标注一致 / 语义不同 / 一方缺失
- [ ] 对每项差异回答三问（capabilities 能否抹平 / 可选方法能否处理 / 是否危险信号）
- [ ] 填出**两份完整的 `RuntimeCapabilities`**（路线图 §5.4 字段）
- [ ] 对每个危险信号给出处置建议（缩小接口 / 拆分方法 / 不纳入抽象）
- [ ] **明确回答**：路线图 §5.2 的 `AgentRuntime` 接口草案够用吗？需增删什么？
- [ ] 写 `research/runtime-abstraction-feasibility.md`

**门**：G1 能否仅凭这份文档定稿接口？不能就继续挖。

## 阶段 4 · gateway 规模判定（R5）

- [ ] 填 design §3.4 的**七项职责**判定表，每格必填
- [ ] 明确回答：**gateway 是否需要自己的 thread/run 存储？** 两 runtime 持久化能力不同时如何处理（交集 / 差异化 / 补齐弱方）
- [ ] 明确回答：会话↔进程编排复杂度，**按 `coldStartCost` 区分池化策略**
- [ ] 估计多 adapter 抽象层本身的工作量
- [ ] 对 runtime 已覆盖的能力标注「**直接透传，不封装**」
- [ ] 写 `research/gateway-scope-decision.md`，含「G2 做什么 / 不做什么」表

**门**：G2 的 PRD 中标注「条件性」的需求，本阶段必须给出取舍结论。

## 阶段 5 · 运行时与工具链（R6）

- [ ] Hono 在 Node 与 Bun 上各起最小服务
- [ ] **核实是否受 SDK 约束**（若某 runtime 的 SDK 只支持特定 JS runtime，这决定选型）
- [ ] 验证复用现有工具链：`pnpm typecheck`、`pnpm lint`（Biome）、`vitest`
- [ ] 验证 pnpm workspace 集成方式
- [ ] 写 `research/runtime-toolchain.md`

## 阶段 6 · 持久化（R7）

- [ ] 基于阶段 4 结论决定是否需要
- [ ] 若需要：候选方案对比、查询层选型、migration 策略、与 Go 侧 Ent 的边界
- [ ] 若需要：确认能否按 `X-Tenant-Id` 分区
- [ ] 写 `research/persistence-decision.md`

> **「不需要持久化」是完全合法且最省事的结论。**

## 阶段 7 · 部署形态（R8）

- [ ] 独立进程 vs 被 taskdaemon 拉起 vs 完全独立，各自代价
- [ ] **多 runtime 并存的分发问题**：两个 runtime 的二进制/依赖如何分发？是否都必须存在？缺一个时能否降级运行
- [ ] Desktop 模式下的形态
- [ ] 开发期启动方式，与 `pnpm dev:*` 的关系
- [ ] 生产期与外部网关的拓扑
- [ ] 写 `research/deployment-shape.md`

## 阶段 8 · Pi WebUI 拆解（R9）

- [ ] 核实仓库地址与**许可证**
- [ ] 跑起来，按**用户旅程**记录交互形态
- [ ] 逐个功能点回答四问（解决什么问题 / 我们有吗 / 依赖哪个私有接口 / **它假设了什么**）
- [ ] 输出**功能分级表**：`G4 必要` / `G5 增强` / `不需要`
- [ ] 输出**Pi 耦合点清单** → G1 的协议补齐清单
- [ ] 输出**单 runtime 假设清单** → G4 必须做成能力驱动的地方
- [ ] 记录值得借鉴的交互细节 + 明确不采纳的复杂度
- [ ] 写 `research/pi-webui-teardown.md`

## 阶段 9 · 版本锚点与路线图回写（R10）

- [ ] 记录 AG-UI、Agent Protocol、MCP、Pi、OMP 版本与快照日期
- [ ] 回写路线图 §5.1 的 **Pi 与 OMP 两行**（其余由 G1 填）
- [ ] **逐条核对路线图 §5.2 ~ §5.9 的假设**，被证伪的修订并在 §16 追加记录
- [ ] 回写路线图 §7.5 状态为 done

## 全量验收

```bash
# 硬约束验证：除 research/ 与路线图外无变更
git status --porcelain | grep -v "^.. .trellis/tasks/07-27-g0-agent-runtime-spike/" | grep -v "^.. docs/roadmap-post-mvp-and-agent.md"
# 期望：无输出
```

- [ ] 上述命令无输出（零代码污染）
- [ ] 九份 `research/*.md` 全部完成
- [ ] `samples/pi/` 与 `samples/omp/` 均有事件流存档
- [ ] `scripts/` 有可运行验证脚本
- [ ] `prd.md` 全部 Acceptance Criteria 勾选

## 评审门

| 门 | 时机 | 检查 |
|---|---|---|
| **G-0** | 阶段 0 结束 | 🚨 **OMP 能否被程序驱动？** 不能则停下重估方案，不要继续 |
| G-1 | 阶段 1 结束 | 延迟数据是否为分布；接入方式结论能否支撑 G3 开工 |
| G-2 | 阶段 2 结束 | **两份能力矩阵是否全部实测**，无「文档上说」 |
| G-3 | 阶段 3 结束 | ★ **G1 能否仅凭可行性分析定稿接口** |
| G-4 | 阶段 4 结束 | 七项职责判定表是否每格都填；G2 范围是否清晰到可写 design |
| G-5 | 阶段 9 前 | G1/G3/G4 能否仅凭这些产出开工，无需回头补测 |

## 回滚点

本任务无代码副作用，任何阶段可直接停止。已完成的 `research/` 产物即使不完整也有价值，保留即可。

**阶段 0 的门是唯一可能导致方案重估的点**，越早触发越便宜。

## 与其他任务的协调

- **G1**：本任务全部产出是 G1 的唯一输入，尤其 `runtime-abstraction-feasibility.md`。阶段 9 完成后通知开工
- **G2**：`gateway-scope-decision.md` 界定其范围；`runtime-attach-and-latency.md` 的 `coldStartCost` 驱动其池化策略
- **G3**：`samples/` 与 `scripts/` 直接可用，交付时明确告知
- **G4**：`pi-webui-teardown.md` 的三份清单界定其范围与能力驱动要求
- **路线图**：假设被证伪时**先改路线图再往下走**，不要让两份文档打架
