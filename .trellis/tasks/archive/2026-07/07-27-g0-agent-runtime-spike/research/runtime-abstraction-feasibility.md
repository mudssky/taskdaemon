# 运行时抽象可行性分析（★ 核心）

> 输入：`pi-capability-matrix.md` + `omp-capability-matrix.md` + 延迟实测  
> 目的：回答 G1 能否冻结 `AgentRuntime` / `RuntimeCapabilities`

## 1. 并排对照表

| 能力 | Pi | OMP | 差异性质 |
|---|---|---|---|
| 程序可驱动（RPC） | 有 | 有 | **一致** |
| agent loop 位置 | runtime 内 | runtime 内 | **一致** |
| 事件骨架 | agent/turn/message/* | 同 + ready/commands | **语义相同，OMP 超集** |
| tool 事件 | tool_execution_* full | 同 | **一致** |
| thinking | message 内 thinking 块 | 同 | **一致** |
| steer/follow_up/abort | 有；abort 真停可续 | 同 | **一致** |
| 会话持久化 | runtime jsonl | runtime jsonl | **一致** |
| 多会话 | CLI=1 进程 1 会话 | 同 | **一致** |
| 模型切换 | 会话级 set_model | 同 | **一致** |
| 历史读取 | get_messages/entries | get_messages 稳；entries 本次超时 | **语义相同，可靠性不同** |
| 非 coding | flag 可收敛到干净 | 默认可对话但全局人格/MCP 仍注入 | **语义不同（收敛成本）** |
| 错误形态 | 多为事件/响应错误 | 更易进程 exit | **语义不同** |
| 资源 | ~150MB | ~490MB | **量级不同** |
| 密钥表面 | 相对干净 | get_state 可能含 Authorization | **危险差异** |
| SDK | 成熟 createAgentSession | 有但分发形态偏 TS 源 | **接入路径不同** |
| 冷启动 CLI | high ~3.1s med | high ~3.8s med | **一致（都 high）** |

## 2. 差异三问

| 差异 | capabilities 抹平？ | 可选方法？ | 危险信号？ | 处置 |
|---|---|---|---|---|
| 事件超集 | 是（忽略未知事件） | — | 否 | adapter 归一化到 RuntimeEvent |
| entries 超时 | 是（historyPagination 能力位） | getHistory 可选 | 否 | 降级只保证 get_messages |
| general 收敛 | 是（workspaceBinding + toolPolicy） | createSession opts | 否 | OMP adapter 强制 profile/system/tools |
| 进程 exit vs 事件错 | 是（errorMode 提示） | — | 弱 | 编排层统一当 run failed + 换进程 |
| 密钥进 state | **否，不能当能力差异忽略** | getState 必须经 sanitizer | **是（安全）** | **不纳入抽象差异，强制 adapter 出口消毒** |
| SDK vs CLI | attachMode + coldStartCost | 多 attach 实现 | 否 | 接口不绑定传输 |
| 内存量级 | coldStartCost 不够，需 resourceClass？ | — | 弱 | 建议 capabilities 增 `memoryClass: 'light'|'heavy'` 或由 G2 配置表硬编码 |

## 3. 两份 RuntimeCapabilities（定稿建议）

### Pi

```ts
{
  id: 'pi',
  profile: ['coding', 'general'],
  attachMode: 'cli-spawn', // 另实现可报 sdk-inprocess
  coldStartCost: 'high',
  steering: true,
  thinking: true,
  toolCallDetail: 'full',
  modelSwitch: 'session',
  workspaceBinding: true,
  sessionPersistence: 'runtime',
  // 建议 G1 增补：
  // abortContinuesSession: true,
  // history: 'messages+entries',
  // memoryClass: 'light',
  // sanitizesSecrets: true
}
```

### OMP

```ts
{
  id: 'omp',
  profile: ['coding', 'general'],
  attachMode: 'cli-spawn',
  coldStartCost: 'high',
  steering: true,
  thinking: true,
  toolCallDetail: 'full',
  modelSwitch: 'session',
  workspaceBinding: true,
  sessionPersistence: 'runtime',
  // abortContinuesSession: true,
  // history: 'messages', // entries 不可靠
  // memoryClass: 'heavy',
  // sanitizesSecrets: false // adapter 必须自己 strip
}
```

## 4. 路线图 §5.2 `AgentRuntime` 草案是否够用？

**结论：大体够用，建议小增补，不必推倒。**

### 保持

```ts
interface AgentRuntime {
  readonly id: string
  capabilities(): RuntimeCapabilities
  createSession(opts: SessionOpts): Promise<SessionHandle>
  prompt(sessionId: string, input: PromptInput): AsyncIterable<RuntimeEvent>
  steer?(sessionId: string, input: PromptInput): Promise<void>
  abort(sessionId: string): Promise<void>
  getState(sessionId: string): Promise<RuntimeState>
  listModels?(sessionId: string): Promise<ModelInfo[]>
}
```

### 建议增加

| 成员 | 原因（实测） |
|---|---|
| `followUp?(…)` | 两 runtime 都有独立语义，与 steer 不同 |
| `getMessages(sessionId, page?)` | 历史是一等能力；比塞进 getState 清晰 |
| `setModel?(sessionId, model)` | 两 runtime 都有；比 listModels 单独存在更对称 |
| `disposeSession(sessionId)` | CLI 必须杀进程；SDK 必须释放 |

### 建议 SessionOpts 明确字段

- `cwd?`, `profile: 'coding'|'general'`
- `tools: 'none'|string[]`
- `sessionPersistence: 'ephemeral'|'runtime-file'`
- `model`, `thinkingLevel`
- `systemPrompt?` / `appendSystemPrompt?`（OMP general 收敛关键）

### 建议 RuntimeEvent 归一化原则

- 透传映射：agent_start/end、message_update(text/thinking)、tool_execution_*  
- 丢弃或降级：extension_ui_request（无 host UI 时）、available_commands_update（可转 capabilities 刷新）  
- **禁止**把 raw `model.headers` 放进对外 state

### 不建议纳入抽象

- OMP 特有 task/hub/subagent 拓扑：作为 adapter 扩展事件或后续 profile，不进 MVP 接口  
- Pi WebUI 的文件浏览器/skills 商店：属 G4 UI，不是 runtime 接口

## 5. 危险信号总结

1. **密钥泄漏（OMP state）** → 强制 sanitizer，不是可选 capabilities。  
2. **OMP 默认不是干净 general** → 抽象仍可行，但 SessionOpts 必须表达裁剪，否则 UI 会误展示 coding 能力。  
3. **未发现「无法用同一接口表达」的核心循环语义**（prompt/steer/abort/tools/thinking/history 均可对齐）。

## 6. 给 G1 的一句话

**可以按 §5.2 草案冻结接口；补 followUp/getMessages/setModel/disposeSession 与 secret-sanitize 不变量即可开工，无需因 OMP 重估双 adapter。**
