# T1b 实现清单

## 顺序

1. [x] types + client：`fileOnly`、write DTO、`putConfigSection`
2. [x] `settingsKeys` + queries/mutation（失效 audio config）
3. [x] `audio-config.schema.ts` + path/error helpers + 单测
4. [x] `AudioConfigForm` + Restart banner + Token 一次显示
5. [x] `SettingsPage` 组装 + router 切换
6. [x] 迁移 `audio.queries` config key；保留 history
7. [x] client 测试补 PUT；样式补 banner
8. [x] `pnpm typecheck && pnpm lint && pnpm --filter @taskdaemon/web test`
9. [x] 勾选 PRD AC + HANDOFF

## 验证命令

```bash
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/web test
```

## 禁止

- 发明 DTO 字段
- token 进 query cache / localStorage
- `window.confirm` 作长期确认
- 直接 `fetch` 绕过 apiClient
