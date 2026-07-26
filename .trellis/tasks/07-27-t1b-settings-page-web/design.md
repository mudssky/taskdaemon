# T1b 完整设置页 · 技术设计

> 依赖：**C-1 已冻结**（T1a）。禁止发明 DTO 字段。

## 1. 边界

| 项 | 决策 |
|---|---|
| 独占路径 | `apps/web/src/features/settings/**` |
| 共享 append-only | `routes/router.tsx`、`app/queryClient.ts`、`lib/api/{types,client}.ts`、`styles.css` |
| 首版可写 section | **仅 `audio`**（`PUT /api/config/audio`） |
| 其它 section | C-1 未开放 GET/PUT → 本任务不伪造面板数据；Desktop 能力面板沿用现有组件 |
| 历史记录 | 继续用 `features/audio` 的 query + `AudioHistoryTable` |

## 2. 页面结构

```
SettingsPage
├── RestartRequiredBanner        # session 内持久可见（非 toast）
├── AudioConfigForm panel        # 可写：hot + restart 字段
│   ├── 热生效字段区
│   ├── Token 区（已配置/未配置 + 重新设置 + 一次明文）
│   ├── 需重启字段区（transcodeTimeoutSeconds）
│   ├── 仅配置文件只读区（pathConfigured / probePathConfigured）
│   ├── 保存结果：applied + reload.subsystems + traceId on error
│   └── 危险操作 AlertDialog
├── Audio inbound 文档 + curl 示例（保留 T0 信息）
├── AudioHistoryTable            # 复用 audio feature
└── Desktop* panels              # 既有；浏览器禁用态由 D1 负责
```

面板独立保存：仅 audio 表单有自己的 RHF + mutation；Desktop 面板无配置写入。

## 3. 数据流

```
GET /api/config/audio
  → settingsKeys.config("audio")
  → defaultAudioConfigFormValues(dto)
  → RHF

submit (dirty only)
  → toAudioConfigWritePayload(values, dirtyFields)
  → PUT /api/config/audio
  → ConfigSectionWriteResponse
  → setQueryData(settingsKeys.config("audio"), data.config)
  → 若 restartRequired 非空 → 写入 RestartBanner 状态
  → 若 payload 含 inbound.token → 组件 state 一次显示明文（不进 query cache / localStorage）
```

## 4. API 类型（严格对齐 C-1）

扩展 `AudioConfig.configuration.fileOnly: string[]`。

新增：

- `AudioConfigWriteRequest`（可选嵌套，字段同 Go `audioConfigWriteRequest`）
- `ConfigSectionWriteResponse` / `ConfigSectionReloadResponse` / `ConfigSubsystemStatus`
- `ConfigFieldError`：`{ path, reason, code }`

Client：`apiClient.putConfigSection(section, body)` → `PUT /api/config/:section`。

## 5. 表单分层

| 文件 | 职责 |
|---|---|
| `audio-config.schema.ts` | Zod schema、defaults、payload mapper |
| `audio-config-errors.ts` | `details.fields[].path` → RHF field name；reason 中文映射 |
| `AudioConfigForm.tsx` | RHF UI + mutation + 确认框 |
| `settings.queries.ts` | `settingsKeys` + query/mutation |

校验不宽于服务端：

- target ∈ {backend, frontend}
- queueLimit ≥ 0；history.limit ≥ 0；maxRedirects ≥ 0
- maxBytes / downloadTimeoutSeconds / transcodeTimeoutSeconds > 0
- allowedSchemes 仅 http/https，合并后非空

## 6. 敏感值

- 永不从 GET 回显 token；只展示 `tokenConfigured`
- 「重新设置」展开输入框；提交 `inbound.token` 明文
- 成功后在内存 state 显示一次 + 复制按钮；随后清空输入框与 dirty
- 明文不写 `queryClient`、不写 `localStorage`

## 7. 危险操作清单

| 操作 | 风险 | 确认方式 |
|---|---|---|
| 开启 `allowPrivateNetworks` | 允许私网 URL 拉取，扩大 SSRF 面 | 提交前 AlertDialog |
| 重新设置 inbound token | 旧 Bearer 立即失效 | 提交前 AlertDialog |

## 8. 需重启提示

- 保存响应 `restartRequired` 非空 → `RestartRequiredBanner` 展示字段列表与说明（落盘已完成，重启前进程仍用旧值）
- Banner 状态存在 SettingsPage 级 state（session 内持久，非 toast）
- 热生效成功：表单内「已生效」文案 + applied 列表

## 9. 未保存离开

- `window.beforeunload` 在 `formState.isDirty` 时提示
- SPA 内无官方 Blocker 依赖时，以 beforeunload 满足 AC；导航侧不强制 window.confirm 干扰

## 10. Query key

```ts
export const settingsKeys = {
  all: ["settings"] as const,
  config: (section: string) => ["settings", "config", section] as const,
};
```

`audioKeys.config` 迁移为 `settingsKeys.config("audio")`，避免双 key。`audioKeys.history` 不变。

## 11. 测试

- `audio-config.schema.test.ts`：defaults、校验边界、payload 仅含 dirty、token 不进 payload 除非 reset
- `audio-config-errors.test.ts`：path 映射与敏感值不缓存 helper
- `client.test.ts`：PUT 路径与 envelope
- 不测 CSS / 静态结构

## 12. 回滚

删除 `features/settings/**`，还原 router/types/client/audio.queries 引用即可。
