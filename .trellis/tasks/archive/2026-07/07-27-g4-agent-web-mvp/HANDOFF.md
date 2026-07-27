# G4 HANDOFF — 最小 agent-web MVP

## 状态

**done（未 merge）**  
协调验收后由协调者 archive / 合入。

## 交付摘要

1. 新建独立应用 `apps/agent-web`（`@taskdaemon/agent-web`）：Vite + React + TS + TanStack Query/Router + Tailwind，对齐 `apps/web` 栈。
2. **聊天核心**：会话列表（展示/创建/切换/删除确认）、消息流、工具时间线、发送与中断、loading/empty/error（含 traceId）、断连提示与手动重载入口。
3. **能力驱动渲染**（不硬编码 runtimeId）：`steering` / `thinking` / `toolCallDetail` / `modelSwitch` / `workspaceBinding` 纯函数分支 + 测试。
4. **AG-UI 事件流归约** `stream-reducer.ts`：纯函数 + 充分单测（文本流、工具三态、name-only/none 降级、thinking 丢弃、cancel/error）。
5. **Mock gateway**（默认 `VITE_AGENT_API_MODE=mock`）：进程内双 runtime 模板（`pi` 全能力 / `omp` 降级），G2 未就绪可独立开发；`http` 模式走 `/v1`。
6. 根 workspace：`typecheck` / `lint` 覆盖 agent-web；`dev:agent-web`；lint-staged 追加。

## 产物路径

| 类别 | 路径 |
|---|---|
| 应用 | `apps/agent-web/**` |
| query keys | `apps/agent-web/src/app/queryClient.ts` → `agentKeys` |
| 流归约 | `apps/agent-web/src/features/agent/stream-reducer.ts` |
| 能力 UI | `apps/agent-web/src/features/agent/capability-ui.ts` |
| Mock | `apps/agent-web/src/lib/api/mock-gateway.ts` |
| HTTP client | `apps/agent-web/src/lib/api/client.ts` |
| 根脚本 | `package.json`（append-only） |
| lockfile | `pnpm-lock.yaml` |

## 使用方式

```bash
# 默认 mock
pnpm dev:agent-web
# → http://127.0.0.1:9246

# 接真实 G2 gateway
VITE_AGENT_API_MODE=http VITE_AGENT_API_ORIGIN=http://127.0.0.1:<gateway> pnpm dev:agent-web
```

Mock 演示：
- 预置 `thr_demo_pi`（full）与 `thr_demo_omp`（name-only / 无 thinking / 无 steering / 无 workspace）
- 输入含 `tool` 触发工具事件；含 `code` 触发 fenced code 渲染

## 验收命令（本机已绿）

```bash
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/agent-web test
# 22 tests passed
```

## PRD AC 自检（要点）

- [x] `apps/agent-web` 脚手架 + workspace 接入 + 独立 `test` script
- [x] 会话列表 / 创建 / 切换 / 删除确认 / offset 分页约定（client+mock）
- [x] 消息历史 + 流式发送 + 代码块/换行
- [x] 工具时间线三态 + 长结果折叠 + name-only/none 不伪造入参
- [x] cancel 回落 + 运行状态可见
- [x] 断连提示与重载入口（无自动续传）
- [x] 错误含 traceId
- [x] capabilities 驱动四分支 + workspaceBinding=false
- [x] 无 Pi WebUI 依赖
- [x] 双 adapter 模板（mock pi/omp）能力分支测试
- [x] typecheck / lint / test 绿
- [ ] 与 G2+G3 真联调（G2/G3 未交付；mock 替代）

## 未做（故意 / 越界禁止）

- thinking level 调节、HITL、steering 实际输入框交互（入口按能力隐藏；完整体验归 G5）
- 文件树 / fork / 断线自动续传 / 会话搜索
- 真实 gateway 集成测试
- merge 到 dev

## 风险与后续

1. G2 上线后将 `VITE_AGENT_API_MODE=http`，核对 SSE 帧 `data:` 与 `Content-Location` 是否与 client 一致。
2. mock 的 omp 降级值是**演示用**有意裁剪，不等于生产 OMP_CLI_CAPABILITIES 全量字段。
3. 滚动策略：距底部 <80px 才 stick；流式用 timelineTail 签名触发。

## filesModified（主要）

- `apps/agent-web/**`（新建）
- `package.json`
- `pnpm-lock.yaml`
- `.trellis/tasks/07-27-g4-agent-web-mvp/HANDOFF.md`（本文件）
