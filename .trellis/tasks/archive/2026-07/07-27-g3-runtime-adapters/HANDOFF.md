# HANDOFF — G3 Runtime Adapters

> taskId: `07-27-g3-runtime-adapters` · Orca dispatch `task_fd813818ff32`  
> 日期: 2026-07-27  
> 状态: **实现完成（默认测试绿）**；G2 联调待 G2 合入

## 交付摘要

在 **`services/agent-gateway/src/runtime/**`** 落地 AgentRuntime adapter 层 + **Pi / OMP 两个 CLI adapter**（G2 骨架尚未合入，按 PRD 独占路径新建包，避免另起 `agent-runtime-adapters`）。

| 模块 | 路径 | 说明 |
|---|---|---|
| 注册表 | `src/runtime/registry.ts` | 按 id 注册/解析；`assertOptionalMethod` → `AGENT_CAPABILITY_UNSUPPORTED` |
| 默认装载 | `createDefaultRuntimeRegistry()` | 预装 `pi` + `omp` |
| JSONL 分帧 | `jsonl-framer.ts` | 半包/粘包/超长/非法帧 |
| RPC | `rpc-client.ts` + `process-transport.ts` + `FakeTransport` | 真实 spawn / 测试注入 |
| 事件映射 | `event-mapper.ts` | 原生 → `RuntimeEvent`；UI/ready/settled 丢弃；不伪造 |
| 密钥消毒 | `sanitize.ts` | 出口强制 strip Authorization/headers/apiKey |
| 池策略 | `pool.ts` | 按 `coldStartCost`/`memoryClass` 导出 minIdle/maxConcurrency/idleTtl（供 G2） |
| Pi | `src/runtime/pi/` | id=`pi`，默认 `PI_CLI_CAPABILITIES`；可选 `attachMode: sdk-inprocess` 声明 `PI_SDK_CAPABILITIES` |
| OMP | `src/runtime/omp/` | id=`omp`，`OMP_CLI_CAPABILITIES`；history=`messages`；heavy；general 裁剪 |
| 包入口 | `@taskdaemon/agent-gateway` / `./runtime` | G2 只应 `import type { AgentRuntime }` + registry |

## 验证

```bash
pnpm --filter @taskdaemon/agent-gateway typecheck   # 绿
pnpm --filter @taskdaemon/agent-gateway lint        # 绿
pnpm --filter @taskdaemon/agent-gateway test        # 34 tests 绿
```

根脚本已纳入 gateway：`pnpm typecheck` / `pnpm lint` 会跑 agent-gateway。

默认测试 **不依赖** 真实 `pi`/`omp` 进程（FakeTransport + fixture 剧本）。  
真实进程集成：`pnpm --filter @taskdaemon/agent-gateway test:integration`（脚本已留，`AGENT_RUNTIME_INTEGRATION=1`；用例目录可后续补）。

## 与 G2 联调点

1. **包**：`services/agent-gateway` 已创建；G2 在同包增加 Hono 路由，**不要**再新建 runtime 目录。
2. **消费方式**：
   ```ts
   import {
     createDefaultRuntimeRegistry,
     derivePoolPolicies,
     type AgentRuntime,
   } from "@taskdaemon/agent-gateway/runtime";
   // 或 package root
   const registry = createDefaultRuntimeRegistry({ traceId });
   const runtime = registry.resolve(thread.runtimeId); // AgentRuntime only
   const pool = derivePoolPolicies(registry.listCatalog(), { hostMemoryGiB: 16 });
   ```
3. **假实现替换**：G2 若已有 fake runtime，改为 `registry.resolve(id)`；capabilities 从 `runtime.capabilities()` 或 `listCatalog()` 透出。
4. **池化**：CLI 均为 `coldStartCost: high` → `warmup=true, minIdle=1, idleTtlMs=10min`；OMP `maxConcurrency` 上限 3，Pi 上限 8（16GiB 主机默认）。
5. **错误码**：adapter 抛 `AgentRuntimeError.code`（`AGENT_*`），HTTP 映射可复用 `AGENT_ERROR_HTTP_STATUS`。
6. **路径边界**：G2 独占 HTTP/编排；G3 独占 `src/runtime/**`。若 G2 已写过 runtime 假实现文件，合并时以本实现替换。

## 能力诚实性（不模拟）

| 字段 | Pi CLI | OMP CLI |
|---|---|---|
| coldStartCost | high | high |
| memoryClass | light | heavy |
| history | messages+entries | **messages only** |
| attachMode | cli-spawn（可声明 sdk） | cli-spawn |

## 残留 / 非本任务

- [ ] 与 G2 HTTP 端到端联调（G2 未合入）
- [ ] 真实 pi/omp 集成测试用例补全（脚本入口已留）
- [ ] Pi 真 SDK `createAgentSession` 接入（capabilities 位已支持；CLI 为 MVP 主路径，对齐 G0）
- [ ] 路线图 §7.5 状态回写（协调者 archive 时）
- [ ] **不要 merge**（监督任务约束）

## 关键文件清单

- `services/agent-gateway/package.json`
- `services/agent-gateway/src/runtime/**`
- `services/agent-gateway/test/**`
- 根 `package.json`（typecheck/lint filter）
- `pnpm-lock.yaml`
