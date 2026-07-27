# AgentRuntime 接口冻结（C-3）

> 类型源：`packages/agent-protocol/src/runtime.ts`、`runtime-types.ts`  
> **定稿依据**：G0 `research/runtime-abstraction-feasibility.md`  
> 路线图草案：§5.2（本文件为最终 SoT）

## 1. 接口签名（定稿）

```ts
interface AgentRuntime {
  readonly id: string
  capabilities(): RuntimeCapabilities
  createSession(opts: SessionOpts): Promise<SessionHandle>
  prompt(sessionId: string, input: PromptInput): AsyncIterable<RuntimeEvent>
  steer?(sessionId: string, input: PromptInput): Promise<void>
  followUp?(sessionId: string, input: PromptInput): Promise<void>
  abort(sessionId: string): Promise<void>
  getState(sessionId: string): Promise<RuntimeState>
  getMessages(sessionId: string, page?: HistoryPage): Promise<MessagePage>
  setModel?(sessionId: string, model: ModelRef): Promise<void>
  listModels?(sessionId: string): Promise<ModelInfo[]>
  disposeSession(sessionId: string): Promise<void>
}
```

### 1.1 相对路线图 §5.2 草案的变更

| 成员 | 决策 | 依据 |
|---|---|---|
| `followUp?` | **采纳** | 两 runtime 均有独立语义，与 steer 不同 |
| `getMessages` | **采纳**（必选） | 历史是一等能力；比塞进 getState 清晰 |
| `setModel?` | **采纳** | 与 listModels 对称；两 runtime 均有会话级切换 |
| `disposeSession` | **采纳**（必选） | CLI 必须杀进程；SDK 必须释放 |
| secret sanitize | **不变量**（非方法） | OMP get_state 可泄漏 Authorization；不能当可选能力 |
| `memoryClass` 等 | 进 **RuntimeCapabilities** | 见 runtime-capabilities.md |
| OMP task/hub/subagent | **否决**纳入 MVP | adapter 内部/后置 |
| Pi WebUI 文件浏览器 | **否决** | 属 G4 UI |

## 2. 必选 vs 可选

| 方法 | 级别 | 缺失时 capabilities |
|---|---|---|
| capabilities / createSession / prompt / abort / getState / getMessages / disposeSession | **必选** | — |
| steer | 可选 | `steering: false` |
| followUp | 可选 | `followUp: false` |
| setModel | 可选 | `modelSwitch: 'none'` |
| listModels | 可选 | 可与 modelSwitch=none 同时 |

调用可选方法前必须检查 capabilities 或 `typeof runtime.steer === "function"`。  
gateway 对前端：capabilities 不支持时返回 `AGENT_CAPABILITY_UNSUPPORTED`，不抛 adapter 内部异常到浏览器。

## 3. 相关类型

详见包内 JSDoc：

- `SessionOpts`：`cwd?`、`profile`、`workspaceBinding?`、`tools`、`sessionPersistence`、`model`、`thinkingLevel`、`systemPrompt`、`appendSystemPrompt`
- `PromptInput`：`text?` / `messages?`
- `RuntimeState`：**禁止** raw provider headers
- `RuntimeEvent`：中立事件联合类型（见 agui-event-mapping 映射表）
- `SessionHandle`：可含 `sessionFile` 供 gateway 薄索引

## 4. 中立性核对

| 检查项 | 结果 |
|---|---|
| 接口名含 pi/omp/rpc/jsonl？ | 否 |
| 事件名使用 runtime 私有 `tool_execution_*`？ | 否 → 归一 `tool_call_*` |
| SessionOpts 绑定某 CLI flag 名？ | 否 → 中立字段，adapter 翻译 |
| capabilities 用自由字符串厂商特性？ | 否 → 封闭枚举 |
| 密钥当作「能力差异」？ | 否 → 强制 sanitizer |

## 5. 「换 adapter 不改前端协议」论证

1. 浏览器只认 `/v1/*` + AG-UI 事件（本契约）。  
2. gateway 只依赖 `AgentRuntime`，不 import 具体 adapter 类型。  
3. 能力差异经 `GET .../capabilities` 声明，UI 按声明降级（与 C-5 同模式）。  
4. Pi/OMP 事件超集在 adapter 内归一；未知事件丢弃。  
5. 因此替换 `runtimeId` 实现只需注册新 adapter + 报告 capabilities，**不改** REST/AG-UI 字段名。

## 6. Secret sanitize 不变量

所有 adapter 在以下出口**必须**剥离敏感字段（含子对象）：

- `Authorization` / `apiKey` / `api_key` / `token` / `secret`  
- provider `headers` 整段（若无法白名单）  
- 环境变量回显

违规视为 adapter bug，不是 capabilities 协商项。
