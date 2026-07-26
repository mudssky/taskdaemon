# G3 Runtime Adapter 层与 Pi/OMP 实现

> **轨**：Track G ｜ **依赖**：**C-3 冻结**（G1 的 `AgentRuntime` 接口 + `RuntimeCapabilities`）
> **并行**：与 G2 / G4 同时开工，联调在 G2 骨架就绪后进行。

## Goal

实现 `AgentRuntime` 抽象层，并**同时交付 Pi 与 OMP 两个 adapter**，让 gateway 与前端都不依赖任何具体 agent 的细节。

**为什么初版就要两个**（路线图 §5.2）：只有一个实现的接口必然长成那个实现的形状。两个实现从第一天并存，抽象才经得起检验。这条撤销了路线图早期版本「第二 adapter 后置」的判断。

## Context

- 接口、能力协商模型、事件映射全部来自 **C-3**
- 各 runtime 的实际能力、接入方式、事件形态来自 **G0 的实测结论**
- 路线图 §5.8：runtime 已提供的能力**直接透传**，adapter 只做协议翻译与能力声明
- 独占路径：`services/agent-gateway/src/runtime/**`（adapter 层 + `pi/` + `omp/`）
- 错误码前缀：`AGENT_*`（与 G2 共用）

## Requirements

### R1 Adapter 层（抽象本体）

- 实现 C-3 定稿的 `AgentRuntime` 接口定义与注册机制
- adapter 按 `id` 注册，gateway 通过 id 选择，新增 adapter 不改 gateway 代码
- **能力协商**：每个 adapter 声明 `RuntimeCapabilities`（路线图 §5.4），gateway 透出给前端
- 可选方法（`steer` / `listModels`）的缺失必须通过 capabilities 可发现；调用未声明的能力返回明确错误而非崩溃
- 抽象层不含任何 Pi 或 OMP 特有的概念

### R2 两个正交维度的建模（路线图 §5.3）

- **接入方式** `attachMode`：`cli-spawn` / `sdk-inprocess`，由 adapter 声明
- **冷启动代价** `coldStartCost`：`low` / `high`，**必须暴露给 G2 的池化策略**（冷启动贵的需预热与更长空闲保留）
- **Profile** `profile`：`coding` / `general`，或两者都支持
- 同一 adapter 若同时提供 CLI 与 SDK 两种接入方式，二者可配置切换，能力声明随之变化

### R3 Pi Adapter

- 按 G0 结论实现（SDK 进程内优先，CLI spawn 作为隔离场景备选）
- JSONL 帧解析：**半包、粘包、超长行、非法 JSON 帧、进程中途退出**都有处理，不崩溃
- 会话生命周期：创建、就绪判定、关闭、资源完全释放（无残留进程与句柄）
- `abort` 必须**真正中断 Pi 侧工作**，不只是断开监听（用 CPU / token 消耗验证）
- 模型切换按 G0 确认的粒度实现
- 能力声明与 G0 的实测能力矩阵一致

### R4 OMP Adapter

- 按 G0 核实的接入方式实现
- 同样覆盖协议解析异常、生命周期、资源释放、abort 语义
- 能力声明与 G0 实测一致；**Pi 有而 OMP 没有的能力必须如实声明为不支持，不做模拟**
- OMP 与 Pi 的差异越大越好 —— 差异是对抽象中立性的检验

### R5 非 coding 场景支持（路线图 §5.5）

- `workspaceBinding: false` 的会话必须能正常创建与运行
- general profile 下不强制要求 cwd，不因缺少工作区而失败
- 至少一个 adapter 验证 general profile 端到端可用

### R6 事件映射

- 各 adapter 的原生事件按 C-3 映射表转换为 `RuntimeEvent`
- G0 中标注「runtime 不提供」的事件按 C-3 的处置方式实现（降级/不支持），**禁止伪造数据**
- 流在会话结束、abort、进程退出三种情况下都正确终止，不悬挂

### R7 错误与可观测

- runtime 侧错误（模型报错、工具失败、进程崩溃）翻译成结构化 `AGENT_*` 错误
- traceId 透传到 runtime 侧日志（C-4 约定）
- 日志脱敏，不打印 prompt 全文敏感内容与凭据

### R8 可替换性验证（硬指标）

- gateway 层代码中**不出现任何 Pi 或 OMP 特有类型**
- 用类型检查或测试证明这一点
- 两个 adapter 可通过配置互换，gateway 与前端代码零改动

### R9 测试

- 抽象层：注册、选择、能力协商、未声明能力的调用行为
- 协议解析：半包、粘包、超长行、非法帧（两个 adapter 各一套）
- 生命周期：创建、关闭、异常退出、资源释放
- 事件映射：各类原生事件到 `RuntimeEvent` 的转换
- **跨 adapter 一致性测试**：同一份测试用例跑两个 adapter，验证对外行为一致（差异只应体现在 capabilities 声明上）
- 默认测试**不依赖真实 runtime 进程**（用 G0 的 `research/samples/` 事件流作为 fixture）
- 保留需要真实 runtime 的集成测试，可单独运行，不进默认 CI

## Acceptance Criteria

### 抽象层

- [ ] `AgentRuntime` 接口与注册机制实现完成
- [ ] adapter 按 id 注册，新增 adapter 不改 gateway 代码（用测试证明）
- [ ] `RuntimeCapabilities` 声明与透出可用
- [ ] 调用未声明的可选能力返回明确错误，不崩溃
- [ ] 抽象层不含 Pi/OMP 特有概念
- [ ] `attachMode`、`coldStartCost`、`profile` 三项正确声明并可被 G2 读取

### Pi Adapter

- [ ] 按 G0 结论的接入方式实现
- [ ] JSONL 半包、粘包、超长行、非法帧、进程中途退出均有处理（用测试证明）
- [ ] 会话创建、就绪判定、关闭正确
- [ ] 关闭后无残留进程与文件句柄（用测试证明）
- [ ] **`abort` 真正中断工作**，非仅断开监听（用 CPU/token 消耗验证）
- [ ] 模型切换按 G0 粒度可用，失败有明确错误不静默降级
- [ ] 能力声明与 G0 能力矩阵一致

### OMP Adapter

- [ ] 按 G0 核实的接入方式实现
- [ ] 协议解析异常处理完整
- [ ] 生命周期与资源释放正确
- [ ] `abort` 语义正确
- [ ] 能力声明与 G0 实测一致
- [ ] **Pi 有而 OMP 没有的能力如实声明为不支持，未做模拟**

### 通用

- [ ] `workspaceBinding: false` 的会话可正常创建与运行
- [ ] general profile 端到端可用（至少一个 adapter 验证）
- [ ] 事件映射符合 C-3
- [ ] runtime 不提供的事件按 C-3 处置，**未伪造数据**
- [ ] 流在会话结束/abort/进程退出三种情况下都正确终止
- [ ] runtime 错误翻译为 `AGENT_*` 结构化错误
- [ ] traceId 透传到 runtime 侧日志
- [ ] 日志脱敏
- [ ] **gateway 代码中无 Pi/OMP 特有类型**（用类型检查证明）
- [ ] **两个 adapter 可配置互换，gateway 与前端零改动**（用测试证明）
- [ ] 跨 adapter 一致性测试通过：同一用例跑两个 adapter，对外行为一致
- [ ] 默认测试不依赖真实 runtime 进程
- [ ] 真实 runtime 集成测试可单独运行
- [ ] `pnpm typecheck`、`pnpm lint`、`pnpm --filter @taskdaemon/agent-gateway test` 全绿
- [ ] 与 G2 联调通过

## Out of Scope

- gateway 端点与编排（归 G2，但本任务需向其暴露 `coldStartCost`）
- 前端（归 G4/G5）
- MCP server 配置管理（归 G6）
- 第三个及以后的 adapter（路线图 §12）
- Pi 或 OMP 本身的功能增强或 fork

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**消费** | **C-3**（`AgentRuntime` 接口 + `RuntimeCapabilities` + `RuntimeEvent` + 事件映射表）、G0 的两份能力盘点与接入方式结论、G0 的 `research/samples/` 事件流 fixture |
| 本任务**产出** | adapter 层与两个实现；向 G2 暴露 `coldStartCost` 供池化策略使用 |
| 下游 | G2（替换假实现）、G6（MCP 工具经由 adapter 执行） |

**并行**：与 G2 同时开工。G2 用 `AgentRuntime` 假实现自测，本任务用 G0 的事件流 fixture 自测，双方就绪后联调。

## Design / Implement 补齐条件

`design.md` 与 `implement.md` **待 C-3 冻结后补**，且 design 必须引用 G0 的 `pi-integration.md` 与 OMP 调研结论确定两个 adapter 各自的接入方式。原因：SDK 与 CLI spawn 两种方式的设计完全不同，两个 adapter 可能走不同路径。

## Definition of Done

- Acceptance Criteria 全部勾选
- 与 G2 联调通过，假实现可无缝替换
- **两个 adapter 互换验证通过**（这是本任务的核心价值证明）
- 路线图 §7.5 状态已回写

## Notes

- **本任务的核心价值不是「接了两个 agent」，而是「证明抽象是中立的」**。互换验证是最关键的验收项
- 抽象层设计时反复问：这个方法/字段是通用概念，还是某个 runtime 的特性？后者一律下沉到 adapter 内部或 capabilities 声明
- **JSONL 帧解析是最容易出低级错误的地方**，半包粘包在真实场景一定会出现
- **abort 语义要验证到底**：很多实现只是断开了监听，runtime 侧还在烧 token
- 建议实现顺序：抽象层 → 能力较全的 adapter → 能力较少的 adapter。第二个 adapter 暴露的抽象缺陷，**回头修抽象层，不要在 adapter 里打补丁**
