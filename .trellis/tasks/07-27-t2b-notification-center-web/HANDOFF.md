# HANDOFF: T2b 通知中心 Web（C-2 消费）

## Status

完成（mock 对齐 C-2；与真实 T2a 联调需后端可用时验证）

## Summary

落地站内通知中心：导航铃铛/未读角标/快览、`/notifications` 列表（筛选+分页进 URL）、已读/删除/清空、关联跳转禁用策略、Desktop 能力提示。query 前缀 `notificationKeys`，mutation 统一失效未读/列表/快览。

## Files

### 新建

- `apps/web/src/features/notifications/notification.schema.ts`
- `apps/web/src/features/notifications/notification.schema.test.ts`
- `apps/web/src/features/notifications/notification-format.ts`
- `apps/web/src/features/notifications/notification-format.test.ts`
- `apps/web/src/features/notifications/notifications.queries.ts`
- `apps/web/src/features/notifications/notifications.queries.test.ts`
- `apps/web/src/features/notifications/NotificationBell.tsx`
- `apps/web/src/features/notifications/NotificationListPage.tsx`
- `apps/web/src/features/notifications/NotificationTable.tsx`
- `.trellis/tasks/07-27-t2b-notification-center-web/design.md`
- `.trellis/tasks/07-27-t2b-notification-center-web/implement.md`
- `.trellis/tasks/07-27-t2b-notification-center-web/HANDOFF.md`

### 修改

- `apps/web/src/lib/api/types.ts` — Notification DTO / severity / subject
- `apps/web/src/lib/api/client.ts` — list/unread/mark/delete/clear
- `apps/web/src/lib/api/client.test.ts`
- `apps/web/src/components/layout/AppShell.tsx` — 侧栏 + 铃铛
- `apps/web/src/routes/router.tsx` — `/notifications` + validateSearch
- `apps/web/src/routes/router.test.tsx` — mock notification endpoints
- `apps/web/src/styles.css` — 铃铛/快览样式
- `.trellis/tasks/07-27-t2b-notification-center-web/task.json` — status in_progress

## API 对齐（C-2）

| Method | Path | 用途 |
|---|---|---|
| GET | `/api/notifications` | `page,pageSize,read,severity` |
| GET | `/api/notifications/unread-count` | `{ count }` |
| POST | `/api/notifications/:id/read` | 单条已读 |
| POST | `/api/notifications/read` | `{ ids }` 批量已读 |
| POST | `/api/notifications/read-all` | 全部已读 |
| DELETE | `/api/notifications/:id` | 删除 |
| DELETE | `/api/notifications/read` | 清空已读 |

## Verification

```text
pnpm typecheck  # pass
pnpm lint       # pass
pnpm --filter @taskdaemon/web test  # 14 files / 50 tests pass
```

业务单测覆盖：URL 非法参数兜底、角标 0/99+、空状态 never/filtered、subject 删除禁用、轮询可见性停止、`notificationKeys` 前缀、API client 路径。

## Residual / Follow-ups

1. **与 T2a 真后端联调**：需 daemon 运行态；本任务以 mock/契约为准。
2. **Desktop 原生通知开关（D2）**：本页仅展示 capability 状态，不实现系统通知。
3. **路线图 §7.3 状态回写**：由协调端归档时处理。
4. **未 merge**：按派遣要求保留分支，协调端 merge+archive+清 worktree。

## Notes for coordinator

- 分支：`mudssky/t2b-notification-center-web`
- 不要 worker merge
- 建议 commit message：`feat(web): 落地通知中心铃铛与列表（C-2）`
