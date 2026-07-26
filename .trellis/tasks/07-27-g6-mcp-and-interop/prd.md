# G6 MCP 目录与双引擎打通

> **轨**：Track G ｜ **依赖**：**G3**（Runtime Adapter 层）+ **C-2 冻结**（T2a 事件契约）
> **性质**：本任务是**唯一**让 taskdaemon 与 agent-gateway 互相感知的任务。在此之前两个引擎互不知道对方存在（路线图 §4.1）。

## Goal

两件事：

1. **MCP 目录**：让 MCP server 可配置，并让 G2 的 tool 策略在真实工具上生效
2. **双引擎打通**：taskdaemon 可定时触发 agent run；agent 可通过 API 触发 taskdaemon 任务

## Context

- 架构图（路线图 §4.1）中 `taskdaemon-go ┈┈▶ agent-gateway` 这条连线**仅在本任务之后存在**
- G2 已建立 tool allowlist 策略框架，本任务接入真实 MCP 目录，**不重构策略框架**
- MCP 工具的实际执行由各 runtime 承担（路线图 §5.8），gateway 只做配置与策略
- 错误码前缀：`MCP_*`（路线图 §9.1 已分配）
- 独占路径：`services/agent-gateway/src/mcp/**`、`internal/httpapi/agent_bridge_*`（新建）

## Requirements

### R1 MCP server 目录

- MCP server 可配置：标识、连接方式、启用状态
- 按会话或租户维度决定可用的 server 集合
- 配置变更后的生效方式明确（热生效或需重启，二选一并文档化）
- server 不可用时的降级行为明确，不让整个会话失败

### R2 tool 策略生效

- 复用 G2 的 tool allowlist 框架，**不重构**
- 策略粒度至少到 tool 级别（可按 server + tool 名允许/拒绝）
- 越界调用返回 `MCP_TOOL_NOT_ALLOWED`
- 默认策略保守：未显式允许的 tool 不可用
- 策略拦截点明确：在 gateway 侧拦截，不依赖 runtime 自觉

### R3 工具可见性与审计

- 当前会话可用的 tool 清单可查询（供前端展示）
- tool 调用产生 C-4 定义的 audit 事件，含 server、tool 名、入参摘要、结果状态
- 审计事件不含敏感入参明文（脱敏）

### R4 taskdaemon → agent（定时触发 agent run）

- taskdaemon 侧新增一种触发方式：定时调用 agent-gateway 创建并执行 run
- 复用既有任务调度模型（cron、超时、取消、执行历史），**不另起一套调度**
- run 结果回写到 taskdaemon 的执行历史，成功/失败可见
- agent-gateway 不可用时任务失败并产生 C-2 通知事件，不静默
- **traceId 透传**：taskdaemon 的 traceId 传给 gateway，不重新生成（路线图 §9.3）

### R5 agent → taskdaemon（触发任务与入站能力）

- agent 可通过 API 或 MCP tool 触发 taskdaemon 任务
- 可查询任务运行状态与结果
- 可调用既有入站音频能力（T0 已交付）
- **认证方式**：使用既有机器认证机制（入站 Bearer Token 模型），不给 agent 管理员 cookie
- 调用受策略约束：哪些任务允许被 agent 触发是可配置的，默认保守

### R6 安全（重点）

- **双向调用都不使用管理员 cookie**
- agent 触发 taskdaemon 任务的权限默认最小：需显式允许，不是默认全开
- 防止循环触发：agent 触发的任务不应能再触发同一个 agent run 造成无限循环，需有环路检测或深度限制
- 双向调用的凭据不进日志

### R7 配置

- MCP 目录配置、双向触发的允许列表按 C-1 契约分级
- taskdaemon 侧配置追加到 `internal/config/types.go` 尾部（append-only）

### R8 测试

- MCP 目录配置与策略拦截测试
- 双向触发的成功与失败路径测试
- 环路检测测试
- 测试不依赖真实 MCP server（用桩）

## Acceptance Criteria

- [ ] MCP server 可配置：标识、连接方式、启用状态
- [ ] 可用 server 集合可按会话或租户决定
- [ ] 配置生效方式已文档化
- [ ] server 不可用时会话降级而非整体失败
- [ ] 复用 G2 的 allowlist 框架，未重构
- [ ] 策略粒度到 tool 级别
- [ ] 越界调用返回 `MCP_TOOL_NOT_ALLOWED`
- [ ] 默认策略保守，未显式允许的 tool 不可用（用测试证明）
- [ ] 策略在 gateway 侧拦截，不依赖 runtime 自觉（用测试证明）
- [ ] 会话可用 tool 清单可查询
- [ ] tool 调用产生符合 C-4 的 audit 事件
- [ ] audit 事件中敏感入参已脱敏
- [ ] taskdaemon 可定时触发 agent run
- [ ] 复用既有调度模型（cron/超时/取消/历史），未另起调度
- [ ] run 结果回写 taskdaemon 执行历史
- [ ] gateway 不可用时任务失败并产生 C-2 通知事件
- [ ] traceId 从 taskdaemon 透传到 gateway，未重新生成（用测试证明）
- [ ] agent 可触发 taskdaemon 任务并查询状态与结果
- [ ] agent 可调用既有入站音频能力
- [ ] 双向调用均使用机器认证，**未使用管理员 cookie**（用测试证明）
- [ ] agent 可触发的任务需显式允许，默认拒绝
- [ ] 环路检测或深度限制生效（用测试证明）
- [ ] 双向调用凭据不进日志
- [ ] 配置项按 C-1 分级
- [ ] MCP 目录、策略拦截、双向触发、环路检测均有测试
- [ ] 测试不依赖真实 MCP server
- [ ] `pnpm typecheck`、`pnpm lint`、`pnpm test:go`、`pnpm vet:go`、`pnpm --filter @taskdaemon/agent-gateway test` 全绿

## Out of Scope

- MCP server 的编写与托管（只做接入与策略）
- MCP Resources / Prompts（先做 Tools）
- A2A / Agent Card（路线图 §12 已排除）
- agent 对 taskdaemon 的完整管理权限（只开放受限的触发与查询）
- MCP 目录的图形化管理界面（配置驱动即可，UI 有需求时另立项）

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**消费** | **G3**（Runtime Adapter 层，MCP 工具经由 adapter 执行）、**C-2**（通知事件，用于失败告警）、**C-4**（audit 事件 schema）、**C-1**（配置分级）、G2 的策略框架 |
| 本任务**产出** | 无契约冻结职责 |
| 共享文件 | `internal/httpapi/router.go`、`internal/config/types.go` / `defaults.go`（均 append-only） |

**注意**：本任务是**唯一跨两个引擎**的任务，同时改 Go 与 TS 两侧，集成风险最高。建议拆成「MCP 目录」与「双引擎打通」两批分别合并。

## Design / Implement 补齐条件

`design.md` 与 `implement.md` **待 G3 交付 + C-2 冻结后补**。原因：MCP 工具执行路径依赖 G3 的实际形态，失败告警依赖 C-2 事件模型。

## Definition of Done

- Acceptance Criteria 全部勾选
- 路线图 §4.1 架构图的 `┈┈` 连线可改为实线并标注本任务
- 路线图 §7.5 状态已回写

## Notes

- **循环触发是本任务最危险的失败模式**：agent 触发任务 → 任务触发 agent → 无限循环烧 token。环路检测必须在设计阶段定死
- 「默认保守」在本任务出现三次（MCP tool、agent 可触发的任务、双向认证），都不要为了演示方便放宽
- 本任务同时改两个语言栈的代码，**分批合并**能显著降低集成痛苦
- 打通之后两个引擎不再完全独立，路线图 §7.8 的「Track G 暂停时自洽」结论对本任务不再适用，需在文档中注明
