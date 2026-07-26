# T2b 通知中心 Web — 技术设计

## 边界

- 独占：`apps/web/src/features/notifications/**`
- append-only：`router.tsx`、`AppShell.tsx`、`lib/api/{types,client}.ts`、`styles.css`
- 消费 C-2：`/api/notifications*` DTO 与筛选语义，不发明字段

## 模块拆分

| 文件 | 职责 |
|---|---|
| `notification.schema.ts` | URL search Zod 校验、API list params 映射 |
| `notification-format.ts` | 角标、空状态、严重级别、关联跳转纯函数 |
| `notifications.queries.ts` | `notificationKeys`、轮询、mutation 失效 |
| `NotificationBell.tsx` | 铃铛 + 未读角标 + 快览 |
| `NotificationListPage.tsx` | 列表页筛选/分页/已读管理 |
| `NotificationTable.tsx` | 表格与行操作 |

## 数据流

1. 未读计数：`useUnreadCountQuery` → `GET /api/notifications/unread-count`，`refetchInterval=15s`，`document.visibilityState !== visible` 时停止。
2. 列表：URL search → `parseNotificationSearch` → `notificationKeys.list(params)`。
3. 快览：`page=1&pageSize=5`，独立 `preview` key。
4. 任一写操作成功后 `invalidateQueries({ queryKey: notificationKeys.all })`，覆盖未读/列表/快览。

## 关联跳转

- `task`：存在于 `useTasksQuery` → `/tasks/$taskId/edit`；否则禁用并说明「关联任务已删除」。
- `run`：`detail.taskId` 对应任务不存在则禁用；否则链到 `/runs`。
- `scheduler`：无实体跳转。

## Desktop

列表侧栏通过 `useDesktopCapability(DesktopCapability.Notification)` 展示可用性说明；不直接读 Wails。

## 测试

- 纯函数：筛选解析、角标上限、空状态、subject link、轮询停止条件、query key 前缀。
- API client 端点路径与 method。
- 不测 CSS/DOM 结构。
