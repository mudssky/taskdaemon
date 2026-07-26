# G1 HANDOFF — C-3 / C-4 契约冻结

## 状态

**done（契约已冻结，未 merge）**  
分支：`mudssky/g1-agent-contract-freeze`  
协调验收后由协调者合入并清 worktree。

## 交付摘要

1. **C-3** Agent 对外契约冻结：Agent Protocol REST 子集、AG-UI 映射、`AgentRuntime`、`RuntimeCapabilities`、非 coding profile。  
2. **C-4** 信任边界冻结：注入头、`PrincipalResolver`、audit/usage schema。  
3. **共享包** `packages/agent-protocol`（`@taskdaemon/agent-protocol`，**types only，零运行时依赖**）。  
4. **文档** `docs/agent-contracts/**`。  
5. **路线图回写** §5.1 / §5.2 / §5.4 / §7.5 G1=done / §9 C-3·C-4 产物路径 / §16 v5.3。

## 产物路径

| 类别 | 路径 |
|---|---|
| 类型包 | `packages/agent-protocol/` |
| 契约索引 | `docs/agent-contracts/README.md` |
| REST 子集 | `docs/agent-contracts/agent-protocol-subset.md` |
| AG-UI 映射 | `docs/agent-contracts/agui-event-mapping.md` |
| AgentRuntime | `docs/agent-contracts/agent-runtime.md` |
| Capabilities | `docs/agent-contracts/runtime-capabilities.md` |
| 非 coding | `docs/agent-contracts/non-coding-profile.md` |
| 信任边界 C-4 | `docs/agent-contracts/trust-boundary.md` |
| 路线图 | `docs/roadmap-post-mvp-and-agent.md` |
| 根脚本 | `package.json`（typecheck/lint 纳入 agent-protocol） |
| lockfile | `pnpm-lock.yaml`（workspace 接入） |

## G0 采纳说明

| G0 建议 | 决策 |
|---|---|
| followUp / getMessages / setModel / disposeSession | **采纳**进 `AgentRuntime` |
| secret sanitize 不变量 | **采纳**（硬约束，非 capabilities 位） |
| coldStartCost high 字段保留 | **采纳**（CLI high；SDK 实例 low） |
| gateway 薄索引 | 契约层确认：messages 透传 runtime，不强制全文双写 |
| runtime 已有能力透传 | REST 映射表一一对应 runtime 方法，不重做 loop |
| capabilities 增补 followUp / abortContinuesSession / history / memoryClass | **采纳** |
| OMP task/hub、Pi WebUI 进接口 | **否决**（MVP） |
| Pi 0.82.0 / OMP 17.1.3 | 与 G0 锚点一致 |

## 验收

```bash
pnpm typecheck   # web + @taskdaemon/agent-protocol
pnpm lint        # 同上
```

本机结果：**全绿**（2026-07-27）。

消费方 import 证明：

- 包 `src/index.ts` 导出全部公共类型与常量；`pnpm --filter @taskdaemon/agent-protocol typecheck` 解析通过。  
- 实例常量：`PI_CLI_CAPABILITIES` / `OMP_CLI_CAPABILITIES` / `AgentRoutes` / `TRUST_HEADERS`。  
- G2/G3/G4 应：`import type { AgentRuntime, AguiEvent, Thread } from "@taskdaemon/agent-protocol"`。

## PRD AC 自检

- [x] Agent Protocol 子集端点清单 + 请求/响应/错误完整  
- [x] 不实现部分及原因（Store/Agents/全量 stream 等）  
- [x] AG-UI 映射表 + 来源转换规则  
- [x] Pi/OMP 不提供事件处置（丢弃/不支持/不模拟）  
- [x] 一次 run 事件序列  
- [x] 断线重连游标语义  
- [x] AgentRuntime 定稿 + 完整类型  
- [x] 依据 G0 feasibility，采纳/否决有说明  
- [x] 无 Pi/OMP 专有概念  
- [x] 可选/必选方法 + capabilities 发现  
- [x] 换 adapter 不改前端协议论证  
- [x] RuntimeCapabilities 字段与枚举  
- [x] 字段语义/默认/前端降级  
- [x] capabilities 透出端点  
- [x] Pi/OMP 实例值入文档与包常量  
- [x] 与 C-5 同构词汇  
- [x] workspaceBinding:false 协议表达  
- [x] general 下 coding 事件不发送  
- [x] 前端 profile 分支约定  
- [x] 注入头 + 缺失行为  
- [x] PrincipalResolver + dev 默认  
- [x] audit / usage schema  
- [x] 外部 vs 本仓职责分界  
- [x] 开发期默认与启动告警  
- [x] packages/agent-protocol types only  
- [x] typecheck/lint 覆盖  
- [x] 消费方可 import（类型包导出 + tsc）  
- [x] §5.1 版本锚点  
- [x] docs/agent-contracts 完成  
- [x] §9 C-3/C-4 路径回写  
- [x] 与 G0 一致、无二次封装  
- [x] pnpm typecheck && pnpm lint 全绿  

## 未做（故意）

- gateway / adapter **实现代码**（G2/G3）  
- agent-web（G4/G5）  
- merge 到 dev  

## 解锁

**G2 / G3 / G4 可仅凭本契约与类型包并行开工。**

## filesModified（主要）

- `packages/agent-protocol/**`（新建）
- `docs/agent-contracts/**`（新建）
- `docs/roadmap-post-mvp-and-agent.md`
- `package.json`
- `pnpm-lock.yaml`
- `.trellis/tasks/07-27-g1-agent-contract-freeze/HANDOFF.md`（本文件）
