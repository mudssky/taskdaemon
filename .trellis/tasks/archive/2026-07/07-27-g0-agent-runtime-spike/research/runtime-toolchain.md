# 运行时与工具链选型

## 结论

| 项 | 决策 |
|---|---|
| JS runtime（gateway） | **Node 22+ 优先**（与现有 web/工具链一致）；Bun 作为可选运行时验证通过 |
| HTTP 框架 | **Hono**（Node `@hono/node-server` 与 Bun 原生均实测 `/health` ok） |
| 包管理 | **pnpm workspace** 新增 `services/agent-gateway`（或 `packages/agent-gateway`） |
| Lint | 复用根 **Biome 2.4.x** |
| Typecheck | 复用 **TypeScript 6** 工程配置 |
| Test | **vitest**（与 apps/web 一致） |
| SDK 约束 | Pi SDK 为标准 npm ESM，**不强制 Bun**；OMP 包 exports 偏 TS 源，**CLI RPC 路径避免绑死 OMP SDK 运行时** |

## 实测

### Hono

- Node v24.14.1 + `hono` + `@hono/node-server`：监听随机端口，`GET /health` → `{ok:true,runtime:"node"}`  
- Bun 1.3.14 + `hono`：`{ok:true,runtime:"bun"}`  

### 现有 monorepo

- `pnpm-workspace.yaml`：`apps/*`、`packages/*`、`services/*`  
- 根脚本：`typecheck`/`lint` 目前 filter 到 web；G2 需 **append** gateway 的 typecheck/lint/test 脚本（共享文件约定）  
- 根已有 `vitest`、`@biomejs/biome`、`typescript`

### Runtime SDK 对工具链的约束

| 包 | 约束 |
|---|---|
| `@earendil-works/pi-coding-agent` | dist JS + types，Node 可 import；适合可选 in-process adapter |
| `@oh-my-pi/pi-coding-agent` | exports 多指向 `src/*.ts`，CLI 二进制已编译；**gateway 不要把 OMP 源码编译进生产路径**，用 spawn `omp` |

## 建议落地

```text
services/agent-gateway/   # 或 packages/ — 与路线图 G2 独占路径对齐
  package.json            # type: module, hono, vitest
  src/
  vitest.config.ts
```

开发：`pnpm --filter @taskdaemon/agent-gateway dev`  
不强制全仓改 Bun；CI 以 Node 为准即可。
