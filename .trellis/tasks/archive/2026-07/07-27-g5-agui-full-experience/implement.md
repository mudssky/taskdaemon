# G5 实现清单

## 顺序

1. [x] C-3 append-only：`protocol.ts` / `routes.ts` / `agui.ts` / `index.ts` + 契约文档 + 路线图 §9 一行
2. [x] 纯逻辑：`stream-reducer` 扩展（CUSTOM/HITL/file_change/去重/thinking 折叠默认）
3. [x] 纯逻辑：`session-filters.ts`（搜索/排序/归档过滤）+ 测试
4. [x] 纯逻辑：`hitl.ts` 状态机 + 测试
5. [x] `AgentApi` + mock：steer / reconnectStream / patchThread / forkThread / respondHitl / 事件缓冲
6. [x] `useRunStream`：自动重连、steer、HITL 响应、可见连接态
7. [x] UI：Timeline thinking 折叠、HITL 面板、steering Composer、WorkspacePanel、ThreadList 管理
8. [x] `AgentWorkspacePage` 接线
9. [x] 单测补齐 + `pnpm typecheck` / `lint` / `agent-web test`
10. [x] HANDOFF + 勾选 PRD AC

## 验收命令

```bash
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/agent-web test
```

## 禁止

- 不 merge
- 不按 runtimeId 硬编码能力
- 不伪造 thinking / tool args / file_change
- 不在本应用内做文件编辑
