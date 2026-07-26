# T2b 通知中心 Web — 实现清单

## 顺序

1. DTO + `apiClient` 方法 + client 测试
2. `notification.schema` / `notification-format` / `notifications.queries` + 单元测试
3. `NotificationBell` / `NotificationTable` / `NotificationListPage`
4. AppShell 铃铛与侧栏、router `/notifications`、样式
5. 路由测试 mock 兼容未读接口
6. `pnpm typecheck && pnpm lint && pnpm --filter @taskdaemon/web test`
7. HANDOFF

## 验收命令

```bash
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/web test
```

## 回滚

删除 `apps/web/src/features/notifications/**`，并回退 AppShell/router/api/styles 的 append 改动。
