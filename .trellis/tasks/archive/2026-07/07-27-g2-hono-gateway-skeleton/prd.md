# G2 Hono Agent Gateway 骨架

> **轨**：Track G ｜ **依赖**：**C-3 / C-4 冻结**（G1）
> **⚠️ 本任务的规模由 G0 的 `gateway-scope-decision.md` 判定**，下述需求中标注「条件性」的部分可能被 G0 结论裁掉。

## Goal

实现 agent-gateway 的服务骨架：对外提供 C-3 定义的 Agent Protocol 子集与 AG-UI 流，对内通过 `AgentRuntime` 接口连接 runtime，并产出 C-4 定义的信任边界钩子与事件。

## Context

**规模决策规则（路线图 §5.8）**：runtime 已原生提供且质量达标的能力，gateway **一律直接透传，不做二次封装**。本任务只做 runtime 与外部网关都做不到的七项：

1. 会话↔进程编排（**含多 adapter 路由与按冷启动代价差异化的池化**）
2. 非交互调用入口
3. thread/run 持久化与恢复（**条件性** —— 若 G0 证明 runtime 持久化足够，本项省略，G2 退化为纯代理）
4. 对外协议转换
5. **多 adapter 抽象与能力协商透出**（抽象层实现归 G3，本任务负责透出与路由）
6. 策略执行（tool allowlist、workspace 沙箱根）
7. usage / audit 事件产出

- 错误码前缀：`AGENT_*`（路线图 §9.1 已分配）
- 独占路径：`services/agent-gateway/**`（新建）
- 运行时、持久化、部署形态选型全部来自 G0

## Requirements

### R1 服务骨架

- Hono 服务落地，runtime 与工具链按 G0 结论（Node 或 Bun）
- 接入现有 pnpm workspace，可被根 `pnpm typecheck` / `pnpm lint` 覆盖
- 有独立的 `test` script，可被路线图 §11 的验收基线调用
- 启动/关闭有优雅处理，不留孤儿进程

### R2 Agent Protocol 子集实现

- 严格按 C-3 实现端点，**不多做、不少做**
- 请求校验、响应形态、错误码（`AGENT_*`）与 C-3 一致
- 统一 envelope 与 traceId，与 taskdaemon 的既有约定保持风格一致

### R3 AG-UI 流

- SSE（或 C-3 确定的传输方式）流式输出
- 事件转换严格按 C-3 的映射表
- 客户端断开时正确清理资源，不泄漏订阅
- cancel 语义正确：取消后 runtime 收到 abort，流正确终止

### R4 会话↔进程编排（**核心，runtime 不做**）

- **adapter 选择**：会话创建时按配置或请求指定 adapter（`pi` / `omp`），未指定用默认
- 会话到 runtime 实例的路由
- **按 `coldStartCost` 差异化池化策略**（路线图 §5.3）：
  - `coldStartCost: 'high'` 的 adapter：维护预热池，空闲保留时间更长
  - `coldStartCost: 'low'` 的 adapter：按需起、快速回收
  - 池化参数可配置，默认值来自 G0 的延迟实测建议
- 并发上限可配置，**可按 adapter 分别设置**，超限时行为明确（排队 or 拒绝，二选一并文档化）
- 空闲会话超时回收，超时时长可配置
- runtime 进程异常退出时的检测与清理，不留僵尸进程
- 编排状态可观测：**按 adapter 分组的**活跃会话数、进程数、预热池状态

### R4.5 能力协商透出（**本轮新增**）

- 提供端点让前端查询：可用 adapter 列表及其 `RuntimeCapabilities`
- 提供端点让前端查询：当前会话所用 adapter 的 capabilities
- capabilities 由 adapter 声明，gateway **只透传不改写**
- 调用某 adapter 未声明的能力时，返回明确错误而非静默失败或崩溃

### R4.6 非 coding 场景支持（路线图 §5.5）

- `workspaceBinding: false` 的会话可正常创建，不强制 cwd
- general profile 会话不因缺少工作区而失败
- 沙箱策略（R7）在无 workspace 时正确降级，不误拒

### R5 持久化（**条件性 —— 由 G0 判定**）

若 G0 结论为「runtime 持久化不足」：

- 按 G0 的持久化选型实现 thread/run 存储
- 支持刷新、断线、重启后的恢复
- 数据按 `X-Tenant-Id` 分区（C-4 约定）

若 G0 结论为「runtime 持久化足够」：

- **本项省略**，gateway 不维护自己的存储，直接透传
- 在 design 阶段记录该决策

### R6 信任边界钩子（C-4，约占本任务 5% 工作量）

- **`PrincipalResolver`** 可插拔，默认 `dev-single-principal` 实现（固定 subject/tenant，自生成 traceId）
- 接受上游注入的 `X-Auth-Subject` / `X-Tenant-Id` / `X-Trace-Id`（或 C-4 确定的命名），缺失时按 C-4 定义的行为处理
- 生产部署未显式切换 resolver 时，启动日志给出**显眼告警**
- **audit 事件产出**：谁、何时、对哪个 workspace、调了哪个 tool、结果如何
- **usage 事件产出**：token 数、模型、cost 字段、归属主体
- 事件只**产出**，不做聚合、不做存储策略、不做配额执行（路线图 §6）

### R7 策略执行（**runtime 与外部网关都做不到**）

- **tool allowlist**：按会话/租户配置允许的 tool 集合，越界返回 `AGENT_*` 错误
- **workspace 沙箱根**：会话 cwd 限制在配置的根目录内，路径穿越被拒绝
- 策略配置可读，默认保守
- 策略框架可扩展（G6 接 MCP 目录时不需要重构）

### R8 安全

- 不实现密钥管理（路线图 §5.6：由外部网关或统一 LLM 出口承担）
- 日志脱敏：不打印 prompt 全文中的敏感内容、不打印凭据
- 假定自己在外部网关之后，但**不因此省略输入校验**

### R9 测试

- 端点契约测试：覆盖 C-3 全部端点的成功路径与主要失败路径
- 编排逻辑测试：并发上限、超时回收、异常退出清理
- 策略测试：tool allowlist 越界、workspace 路径穿越
- 测试不依赖真实 runtime 进程（用 `AgentRuntime` 的假实现）

## Acceptance Criteria

- [ ] Hono 服务落地，runtime 与工具链符合 G0 结论
- [ ] 接入 pnpm workspace，被根 `pnpm typecheck` / `pnpm lint` 覆盖
- [ ] 有独立 `test` script
- [ ] 启动/关闭优雅，不留孤儿进程
- [ ] C-3 全部端点实现，无多余端点
- [ ] 请求校验、响应形态、`AGENT_*` 错误码与 C-3 一致
- [ ] 统一 envelope 与 traceId
- [ ] AG-UI 流式输出可用，事件转换符合 C-3 映射表
- [ ] 客户端断开时资源正确清理，无订阅泄漏（用测试证明）
- [ ] cancel 后 runtime 收到 abort，流正确终止
- [ ] 会话到 runtime 实例的路由可用
- [ ] **adapter 可按配置或请求指定**，未指定用默认
- [ ] **池化策略按 `coldStartCost` 差异化**：high 有预热池，low 按需起（用测试证明）
- [ ] 池化参数可配置，默认值来自 G0 延迟实测
- [ ] 并发上限可配置且**可按 adapter 分别设置**，超限行为明确且已文档化
- [ ] 空闲会话超时回收生效，时长可配置
- [ ] runtime 异常退出被检测并清理，无僵尸进程（用测试证明）
- [ ] **按 adapter 分组的**活跃会话数、进程数、预热池状态可观测
- [ ] 可用 adapter 列表及其 capabilities 可查询
- [ ] 当前会话的 capabilities 可查询
- [ ] capabilities 只透传不改写（用测试证明）
- [ ] 调用未声明的能力返回明确错误，不静默失败、不崩溃
- [ ] `workspaceBinding: false` 会话可正常创建，不强制 cwd
- [ ] general profile 会话不因缺少工作区而失败
- [ ] 沙箱策略在无 workspace 时正确降级，不误拒
- [ ] 持久化决策已在 design 中记录（实现或明确省略）
- [ ] 若实现持久化：刷新/断线/重启后可恢复，数据按租户分区
- [ ] `PrincipalResolver` 可插拔，默认 dev 实现可用
- [ ] 注入头被正确接受与透传，缺失时行为符合 C-4
- [ ] 生产未切换 resolver 时启动日志有显眼告警
- [ ] audit 事件产出，字段符合 C-4 schema
- [ ] usage 事件产出，字段符合 C-4 schema
- [ ] 未实现聚合/存储策略/配额执行（保持职责边界）
- [ ] tool allowlist 生效，越界返回明确错误（用测试证明）
- [ ] workspace 沙箱根生效，路径穿越被拒绝（用测试证明）
- [ ] 策略配置默认保守
- [ ] 未实现密钥管理
- [ ] 日志脱敏，不含 prompt 敏感内容与凭据
- [ ] 端点契约测试覆盖成功与主要失败路径
- [ ] 编排、策略逻辑有测试
- [ ] 测试不依赖真实 runtime 进程
- [ ] `pnpm typecheck`、`pnpm lint`、`pnpm --filter @taskdaemon/agent-gateway test` 全绿

## Out of Scope

- Runtime Adapter 层与各 adapter 实现（归 G3）
- 前端（归 G4/G5）
- MCP server 目录管理（归 G6）
- SSO / 多租户管理 / 审计存储 / 配额执行 / 计费（路线图 §6，归外部网关）
- **重做 runtime 已有能力**：agent loop、MCP 工具执行、会话内状态机、模型切换（路线图 §12）
- **adapter 实现本身**（归 G3；本任务只消费 `AgentRuntime` 接口与 capabilities）
- A2A

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**消费** | **C-3**（对外契约 + `AgentRuntime` 接口）、**C-4**（信任边界）、G0 的规模判定与选型结论 |
| 本任务**产出** | gateway 运行时；无契约冻结职责 |
| 下游 | G3（提供 adapter 实现）、G4（联调）、G6（策略框架扩展点） |

**并行**：与 G3 / G4 同时开工，三方共用 C-3 契约，联调在各自骨架就绪后进行。

## Design / Implement 补齐条件

`design.md` 与 `implement.md` **待 C-3 / C-4 冻结后补**，且 design 必须先引用 G0 的 `gateway-scope-decision.md` 明确本任务的实际范围。原因：本任务的规模是变量，不是常量。

## Definition of Done

- Acceptance Criteria 全部勾选（条件性项按 G0 结论取舍并记录）
- 路线图 §7.5 状态已回写
- G3 的假实现可替换为真实 adapter，无需改 gateway 代码

## Notes

- **本任务最大的风险是做多了**：每写一个功能前先问「runtime 是不是已经做了」，答案是则透传
- 会话↔进程编排是本任务不可替代的核心价值，值得花最多时间
- **池化策略必须由 `coldStartCost` 驱动**，不要对两个 adapter 用同一套参数 —— 这正是 G3 暴露该字段的原因
- capabilities **只透传不改写**：一旦 gateway 开始「修正」adapter 的声明，中立性就破了
- 信任边界钩子只占 5% 工作量，但决定了未来能不能平滑接入 Java 网关，不能省
- 测试用 `AgentRuntime` 假实现，让 G2 与 G3 真正解耦。**假实现应覆盖两种 `coldStartCost`**，以验证池化分支
