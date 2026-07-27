# RuntimeCapabilities 能力协商（C-3）

> 类型源：`packages/agent-protocol/src/capabilities.ts`  
> 路线图：§5.4  
> **与 C-5 同构**：Desktop `CapabilityState` = 声明 + 前端降级；此处 `RuntimeCapabilities` = 同一心智。

## 1. 共用词汇（C-5 ↔ C-3）

| 概念 | Desktop C-5 | Agent C-3 |
|---|---|---|
| 能力声明 | `DesktopCapability` 常量 / list | `RuntimeCapabilities` 对象 |
| 发现入口 | `useDesktopCapability` / list API | `GET /v1/threads/{id}/capabilities` |
| 不可用时 | `status: unavailable` + reason | 字段 false / enum none + UI 禁用 |
| 前端纪律 | 不硬编码 Wails | 不硬编码 runtimeId 能力 |
| 展示 | 禁用态 + 原因，不隐藏不伪造 | 同：禁用 steering/thinking 控件并说明 |

## 2. 字段清单

| 字段 | 类型 | 默认（文档语义） | 前端降级 |
|---|---|---|---|
| `profile` | `AgentProfile` \| 数组 | `coding` | general 隐藏 diff/workspace UI |
| `attachMode` | `cli-spawn` \| `sdk-inprocess` | — | 一般不直接展示；编排用 |
| `coldStartCost` | `low` \| `high` | — | 可提示「准备中」；池化归 G2 |
| `steering` | boolean | false | false → 禁用 steer 输入 |
| `followUp` | boolean | false | false → 禁用 follow-up |
| `thinking` | boolean | false | false → 不渲染 reasoning 面板 |
| `toolCallDetail` | `full` \| `name-only` \| `none` | `none` | none 隐藏工具时间线；name-only 无参数展开 |
| `modelSwitch` | `session` \| `request` \| `none` | `none` | none 禁用模型选择器 |
| `workspaceBinding` | boolean | true | false 隐藏文件树/diff |
| `sessionPersistence` | `runtime` \| `gateway` \| `none` | `none` | none 提示刷新可能丢会话 |
| `abortContinuesSession` | boolean | true | false 时 cancel 后需新 thread |
| `history` | `messages` \| `messages+entries` \| `none` | `messages` | none 禁用历史拉取 |
| `memoryClass` | `light` \| `heavy` | `light` | 不直接 UI；运维指标 |

**不进入 capabilities**：`sanitizesSecrets`（硬不变量）。

## 3. 对外透出

| 方式 | 路径 | 时机 |
|---|---|---|
| **主** | `GET /v1/threads/{threadId}/capabilities` | 会话页加载 |
| 可选 | 创建 thread 响应附带 `capabilities` | 减少 RTT（G2 可做，契约允许） |
| 禁止 | 前端写死 `if (runtimeId === 'omp')` | 违反中立性 |

全局「运行时目录」（未建 thread 前选 pi/omp）由 G2 提供只读配置端点时，返回各 runtime 的**静态** capabilities 模板（即下方实例值）；会话级以 thread 端点为准。

## 4. 参考实例值

### 4.1 Pi CLI（`PI_CLI_CAPABILITIES`）

```ts
{
  profile: ['coding', 'general'],
  attachMode: 'cli-spawn',
  coldStartCost: 'high',
  steering: true,
  followUp: true,
  thinking: true,
  toolCallDetail: 'full',
  modelSwitch: 'session',
  workspaceBinding: true,
  sessionPersistence: 'runtime',
  abortContinuesSession: true,
  history: 'messages+entries',
  memoryClass: 'light',
}
```

### 4.2 Pi SDK（`PI_SDK_CAPABILITIES`）

同 CLI，但 `attachMode: 'sdk-inprocess'`，`coldStartCost: 'low'`。

### 4.3 OMP CLI（`OMP_CLI_CAPABILITIES`）

```ts
{
  profile: ['coding', 'general'],
  attachMode: 'cli-spawn',
  coldStartCost: 'high',
  steering: true,
  followUp: true,
  thinking: true,
  toolCallDetail: 'full',
  modelSwitch: 'session',
  workspaceBinding: true,
  sessionPersistence: 'runtime',
  abortContinuesSession: true,
  history: 'messages', // entries 不可靠
  memoryClass: 'heavy',
}
```

注：OMP 的 general **非开箱干净**——adapter 必须用 SessionOpts（tools/systemPrompt/profile）强制裁剪；capabilities 仍可声明支持 general。

## 5. G0 增补字段说明

相对路线图 §5.4 初稿，G1 **增加**：

- `followUp`
- `abortContinuesSession`
- `history`
- `memoryClass`

**保留** `coldStartCost: high` 字段（G0 确认 CLI 均 high，字段仍必需，供未来 SDK low 区分）。
