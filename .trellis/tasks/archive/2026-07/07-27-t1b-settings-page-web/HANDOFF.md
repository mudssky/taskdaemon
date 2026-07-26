# HANDOFF · T1b / W1 完整设置页（Web）

> 任务：`07-27-t1b-settings-page-web`  
> 日期：2026-07-27  
> 状态：**实现完成** — 待协调者验收；本 worker **不 merge / 不 archive**

## 结论

- 消费 **C-1 冻结** 的 `PUT /api/config/:section`（首版 `audio`）落地可写设置页。
- `settingsKeys` 命名空间已产出；路由 `/settings` 切换到 `features/settings/SettingsPage`。
- Token 不明文回显；重置后**仅内存一次显示 + 复制**；需重启字段有**持久 banner**（非 toast）。

## 交付物

### 代码（独占 `apps/web/src/features/settings/**`）

| 路径 | 说明 |
|---|---|
| `features/settings/SettingsPage.tsx` | 设置页壳：重启 banner + 音频表单 + 历史 + Desktop 面板 |
| `features/settings/AudioConfigForm.tsx` | RHF 可写表单、危险确认、字段级错误、reload 结果 |
| `features/settings/audio-config.schema.ts` | Zod / defaults / partial payload mapper |
| `features/settings/audio-config-errors.ts` | path 映射、fileOnly 只读项、token scratch、危险确认判定 |
| `features/settings/settings.queries.ts` | `settingsKeys` + GET/PUT hooks |
| `features/settings/*.test.ts` | schema / error helper 单测 |
| `lib/api/types.ts` | `fileOnly` + write DTO（严格对齐 C-1） |
| `lib/api/client.ts` | `putConfigSection` |
| `routes/router.tsx` | append：`SettingsPage` |
| `app/queryClient.ts` | re-export `settingsKeys` |
| `styles.css` | restart banner / token-once / save-result |

### 移除

- `features/audio/AudioSettingsPage.tsx`（只读页由 settings 替代；history 仍在 audio feature）

### 文档

- `design.md` / `implement.md` / PRD AC 已勾选

## 行为摘要

1. **热生效**：保存成功 → 「已生效」+ applied 列表 + subsystems  
2. **需重启**：`restartRequired` → 页顶持久 banner 累加字段路径  
3. **file_only**：只读状态文案（非 disabled input）  
4. **Token**：已配置/未配置；重置走 `inbound.token`；成功后 one-time 明文 + 复制  
5. **危险确认**（AlertDialog）：开启私网、重置 token  
6. **脏离开**：`beforeunload`  
7. **字段错误**：`details.fields[].path` → RHF field + 中文提示 + `traceId`

## 验证

```bash
pnpm typecheck   # 绿
pnpm lint        # web 绿；agent-web 仅有既有 warning（非本任务）
pnpm --filter @taskdaemon/web test   # 64 passed
```

## 残留 / 给协调者

| 项 | 说明 |
|---|---|
| 其它 section PUT | C-1 仅开放 `audio`；后续 section 放开后可加面板 |
| 真 daemon 浏览器冒烟 | 本 worker 以契约/单测验收；建议协调者在已合入 T1a 的环境点一次保存 |
| merge + archive | **协调者**执行；本 worker 不 merge |
| 路线图 §7.3 W1 | 可标 done（待合入后） |

## 危险操作清单（已实现确认）

1. 开启 `allowPrivateNetworks`  
2. 重新设置 inbound token  
