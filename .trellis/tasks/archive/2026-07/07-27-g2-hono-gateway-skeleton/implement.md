# G2 实施清单

## 前置

- [x] 读 C-3/C-4 与 G0 gateway-scope / latency / toolchain
- [x] design.md 记录薄索引与 reject-on-overlimit

## 步骤

1. 脚手架 `services/agent-gateway`（package/tsconfig/biome/vitest）
2. 根 package.json append typecheck/lint/test；lint-staged 追加 gateway
3. config + errors + trust（resolver/emitter/middleware）
4. policy（allowlist + sandbox）
5. mock adapters（high/low）+ registry + pool + orchestrator
6. memory store + AG-UI mapper + SSE
7. routes：health / runtimes / threads / runs（含 stream/cancel/steer/follow-up）
8. index 启动优雅关闭
9. 测试与 `pnpm typecheck && pnpm lint && pnpm --filter @taskdaemon/agent-gateway test`
10. HANDOFF.md

## 验收命令

```bash
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/agent-gateway test
```

## 回滚

删除 `services/agent-gateway` 与根 scripts 中 gateway 追加行即可；不碰 agent-protocol 冻结面。
