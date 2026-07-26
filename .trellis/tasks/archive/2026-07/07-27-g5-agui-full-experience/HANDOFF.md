# G5 HANDOFF — AG-UI 完整体验（Codex 式）

## 状态

**done（未 merge）**  
协调者验收后 merge + archive + 清 worktree。

## 交付摘要

在 G4 `apps/agent-web` 骨架上扩展完整 Codex 式体验：

1. **thinking**：`REASONING_*` 可折叠，默认收起；`capabilities.thinking=false` 不渲染、不伪造。
2. **HITL**：`CUSTOM hitl_request` + REST `POST .../hitl`；批准/拒绝/修改后继续；等待态显眼；超时标记。
3. **Steering**：运行中注入，`capabilities.steering=false` 隐藏入口；时间线 `kind:"steer"` 可区分。
4. **工作区（coding）**：`WorkspacePanel` 展示 cwd / file_change / diff；越界 `outsideSandbox` 明示；只读；`workspaceBinding=false` 整块不渲染。
5. **断线续传**：`Last-Event-ID`/`cursor` 重连；`seenEventIds` 去重；游标失效 → 刷新路径；重连过程可见。
6. **会话管理**：重命名、搜索（标题+内容）、fork（源不变）、归档/恢复、列表排序与筛选。
7. **C-3 append-only**：`packages/agent-protocol` + `docs/agent-contracts` + 路线图 §9.2.1。

## 产物路径

| 类别 | 路径 |
|---|---|
| 流归约 | `apps/agent-web/src/features/agent/stream-reducer.ts` |
| HITL / 会话过滤 | `hitl.ts` / `session-filters.ts` |
| 流 hook | `useRunStream.ts` |
| UI | `Timeline.tsx` / `Composer.tsx` / `ThreadList.tsx` / `WorkspacePanel.tsx` / `AgentWorkspacePage.tsx` |
| API | `lib/api/client.ts` / `mock-gateway.ts` |
| 契约 | `packages/agent-protocol/**`、`docs/agent-contracts/**` |
| 路线图 | `docs/roadmap-post-mvp-and-agent.md` §9.2.1 |

## 演示（mock）

```bash
pnpm dev:agent-web
# http://127.0.0.1:9246
```

- `thr_demo_pi`：全能力；输入 `hitl` / `file` / `tool` / `code` 触发对应演示。
- `thr_demo_omp`：无 thinking/steering/workspace；UI 分支降级无崩溃。
- 侧栏：搜索、重命名、归档、分叉、排序。

## 验收（本机已绿）

```bash
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/agent-web test
# 38 tests passed
```

## PRD AC 要点

- [x] thinking 可折叠默认收起 + capabilities 门控
- [x] HITL 决策上下文 / 批准拒绝修改 / 等待态 / 超时
- [x] steering 注入 + 能力隐藏
- [x] workspaceBinding=false 隐藏工作区；coding 可见 cwd/变更/diff/越界
- [x] 双 adapter mock 分支
- [x] 断线自动续传 + 去重（单测）+ 可见重连 + 游标失效恢复
- [x] 会话重命名/搜索/fork/归档/筛选排序
- [x] 无自造 SSE type；CUSTOM 仅 file_change/usage/hitl_request
- [x] 契约扩展回写 §9
- [x] 单元测试覆盖 HITL/steering/去重/fork/搜索
- [x] typecheck / lint / agent-web test 绿
- [ ] 与真实 G2+G3 gateway 全量联调（PATCH/fork/hitl 网关实现仍缺；HTTP client 已对接；steer/stream reconnect 网关已有）

## 风险

1. **gateway 未实现** `PATCH /threads`、`POST .../fork`、`POST .../hitl`：HTTP 模式这些按钮会 404；默认 mock 完整。
2. 真 runtime HITL 依赖 adapter 发出 `hitl_request`；当前 mock 关键词演示。
3. mock 重连只重放缓冲，不挂 live subscriber；真实 gateway SSE buffer 语义见 C-3。

## 不要

- 不要 merge（协调者）
- 不要在本应用内加文件编辑

## filesModified（主要）

- `packages/agent-protocol/src/{protocol,routes,agui,index}.ts`
- `docs/agent-contracts/*`、`docs/roadmap-post-mvp-and-agent.md`
- `apps/agent-web/src/**`（G5 扩展）
- `.trellis/tasks/07-27-g5-agui-full-experience/{design,implement,HANDOFF}.md`
