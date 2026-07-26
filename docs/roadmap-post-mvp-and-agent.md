# taskdaemon 后 MVP 与通用 Agent 路线图

> 状态：**并行开发施工图**（任务树已创建）
> 日期：2026-07-27（v4）
> 性质：**不写业务代码**；作为后续所有子任务的单一入口与跨任务契约来源。
> 父任务：`.trellis/tasks/07-27-post-mvp-and-agent-roadmap/`
> 关联：主 PRD 第二轮见 `.trellis/tasks/archive/2026-05/05-14-cross-platform-scheduler-daemon/prd.md`

---

## 1. 文档目的

把当前仓库从「一期调度核心可用」切到「产品深度 + 通用 Agent 后端」，并且**支撑多任务并行开发**。为此本文必须回答四件事：

1. 已交付 vs 缺口（避免重复立项）
2. 四轨分期与**任务依赖图**
3. **跨任务契约冻结点**（并行的前提）
4. **文件所有权边界**（并行的安全带）

**本备忘不是实现清单。** 每个子任务仍须完成 `prd.md` /（复杂任务）`design.md` + `implement.md`，经批准后再 `task.py start`。

### 1.1 相对 v1 的变更

| # | 变更 | 原因 |
|---|---|---|
| 1 | Wave 数字/字母混编 → **四轨 T / W / D / G** | 原编号既不表时间也不表优先级，并行时无法定位 |
| 2 | 原 1b/1c/2a 三个通知任务 → **合并为事件总线 + 可并行 sink** | 三者本是同一事件总线的三个 sink，跨 Wave 拆开会让事件模型返工两次 |
| 3 | Wave C 企业能力 → **降维为信任边界钩子 + 外部网关契约文档** | 鉴权/审计/成本由外部网关（APISIX/Kong/Java BFF）承担 |
| 4 | 新增 **G0 技术 spike** | Pi SDK 可用性、gateway 持久化、runtime、部署形态四项硬前置未验证 |
| 5 | 新增 **§7 依赖图 / §8 文件所有权 / §9 契约冻结点** | v1 声称可并行但缺全部支撑要素 |
| 6 | 识别 **`packages/` ROI 拐点** | Track G 一开工，共享 TS 类型从「ROI 低」变「硬需求」 |
| 7 | 每条轨给出**可执行验收命令** | v1 的「成功标准」是一句话，不可验收 |
| 8 | 架构图标注生效阶段 | v1 图里画了 Track G 后期才存在的连线 |
| 9 | **前后端按层拆任务**（T1→T1a/T1b、T2→T2a/T2b、T7→T7a/T7b） | v1 把前后端塞进同一任务，前端无法与后端并行 |
| 10 | 新增 **Track D（Desktop）**，含 TD1 平台边界层 | 规范定义了 `src/lib/desktop` 边界层但**目录从未落地**；Desktop 增量能力无归属 |
| 11 | **撤销「第二 Runtime Adapter 后置」**，初版即 Pi + OMP 并存 | 只有一个实现的接口必然长成那个实现的形状；两个并存抽象才经得起检验 |
| 12 | 新增 §5.3 两个正交维度、§5.4 能力协商、§5.5 非 coding 场景 | 接入方式（延迟）与 profile（能力/UI）是独立维度；企业场景多数非 coding |

---

## 2. 原则

1. **双引擎松耦合**：Go `taskdaemon` 专精本机调度与运维能力；Agent 会话与运行时走独立 Hono 网关。禁止在 Go 内嵌 agent loop。
2. **对外契约稳定、对内可换引擎**：浏览器/SDK 只认标准协议子集；Pi 只是默认 Runtime Adapter。
3. **不复活一期已交付项**：调度 CRUD、runner 白名单、单管理员认证、基础 Web/Desktop 只陈述，不重复立项。
4. **父任务只做地图与跨任务契约**，不写业务代码；子任务按轨可独立验收。
5. **KISS / YAGNI**：协议只钉 MVP 子集；A2A、完整 Agent Protocol Store、插件平台、分布式队列默认后置。
6. **企业能力在仓库外**：鉴权、租户、审计聚合、配额执行归外部网关；本仓库只留**外部做不到**的那部分（见 §6）。
7. **并行优先于顺序**：能拆成独立可验收单元的，先拆；串行只保留真实依赖。
8. **契约先于实现**：前后端拆任务的代价是必须先冻结接口。冻结点见 §9，冻结前下游不开工。

---

## 3. 现状分层

### 3.1 taskdaemon（已交付基线）

| 层 | 状态 | 代码锚点 |
|---|---|---|
| 调度核心（cron / 启停 / 触发 / 取消 / skip-overlap） | 已交付 | `internal/scheduler/` |
| Runner（shell/bash/pwsh/python/node/typescript） | 已交付 | `internal/runner/` |
| 单管理员认证 + session/CSRF | 已交付 | `internal/auth` + `/api/auth` |
| Web/Desktop 同 UI（总览 / 任务 / 历史） | 已交付 | `apps/web/src/features/*` |
| Ent + SQLite/PostgreSQL（自动 migration） | 已交付 | `internal/data/ent` |
| 进程日志轮转（lumberjack） | 已交付 | `internal/logging/file_handler.go` |
| 入站音频 API / 队列播放 / 历史重放 | **主体已交付** | `internal/audio` + `/api/inbound/*` |
| 音频设置页 | **只读展示，未写配置** | `AudioSettingsPage`；`GET /api/config/audio` only |
| Desktop 平台能力边界层 | **规范已定义，`src/lib/desktop` 目录不存在** | `.trellis/spec/frontend/routing-platform-guidelines.md` |
| monorepo `packages/` 共享包 | `pnpm-workspace.yaml` 已声明 glob，**目录不存在** | — |

> 注：pnpm 对不存在的 workspace glob 是容忍的，所以当前不报错。Track G 开工时必须落地（见 §10）。

### 3.2 任务状态

| 状态 | 任务 | 说明 |
|---|---|---|
| `in_progress` | `05-28-hermes-agent-audio-api` | **即 T0**，已挂载为本父任务子任务；不新建收口任务 | | **done**（写配置→T1a/T1b） |
| 已归档（2026-05） | 34 项 | 骨架、调度、UI、shadcn、Go 拆分、前端规范等一期关账 |

### 3.3 Agent 平台（尚未立项 → 现已建 Track G 任务）

| 能力 | v1 状态 | v3 归属 |
|---|---|---|
| Pi/Hono 技术前置验证 | 未识别 | **G0** |
| 对外契约 + 共享包 | 仅规划 | **G1** |
| Hono Agent Gateway | 未建 | **G2** |
| Pi Runtime Adapter | 未建 | **G3** |
| Codex 式前端 | 未建 | **G4 / G5** |
| 企业 SSO / 审计聚合 / 多租户 | 未建 | **移出仓库**（见 §6） |

---

## 4. 目标形态

### 4.1 双引擎

```text
┌─────────────────────────┐     ┌──────────────────────────────────┐
│  apps/web（管理台）      │     │  apps/agent-web（Codex 式 App）   │
│  Web + Desktop 共用      │     │  会话 / 流式 / 工具时间线 / HITL  │
└───────────┬─────────────┘     └────────────────┬─────────────────┘
            │ HTTP                                │ AG-UI (SSE/WS)
            ▼                                     ▼
┌─────────────────────────┐     ┌──────────────────────────────────┐
│  services/taskdaemon-go │┈┈┈┈▶│  services/agent-gateway (Hono)   │
│  调度 / runner / 音频    │ G6  │  Thread·Run·编排·策略·事件产出    │
│  CLI / Desktop 壳        │ only│  多 Adapter + 能力协商            │
└─────────────────────────┘     └────────────────┬─────────────────┘
                                                 │ AgentRuntime 接口
                              ┌──────────────────┴──────────────────┐
                              │                                     │
                              ▼                                     ▼
                   ┌────────────────────┐              ┌────────────────────┐
                   │  Pi Adapter        │              │  OMP Adapter       │
                   │  SDK 进程内 / CLI   │              │  CLI（待 G0 核实）  │
                   └────────────────────┘              └────────────────────┘
                              │                                     │
                              └──────────────┬──────────────────────┘
                                             ▼
                                      ┌────────────────┐
                                      │  MCP tools     │
                                      └────────────────┘
```

图例：`──` 从 G2 起存在；`┈┈` **仅 G6 之后存在**。

**边界：**

- 浏览器 **不** 直连任何 runtime 的私有协议。
- taskdaemon **不** 承载 agent 会话状态机。
- **初版即两个 adapter 并存**（Pi + OMP），保证 `AgentRuntime` 抽象经得起检验（§5.2）。
- 两个引擎在 G6 之前**互不知道对方存在**；Track T/W/D 与 Track G 因此可以完全并行。

### 4.2 Web 与 Desktop 的关系（架构决策）

**决策：不分叉两套业务代码。`apps/web` 保持单一 React 代码库，Desktop 差异全部收敛在边界层。**

```text
                      apps/web（唯一业务代码库）
      ┌──────────────────────────────────────────────────┐
      │  features/*  routes/*  components/*   ← 平台无关  │
      └───────────────────────┬──────────────────────────┘
                              │ 只依赖 capability 接口
                              ▼
      ┌──────────────────────────────────────────────────┐
      │  src/lib/desktop/  ← TD1 落地的能力边界层         │
      │  useDesktopCapability() → { available, invoke }   │
      └──────┬────────────────────────────┬──────────────┘
             │ Web：noop / disabled        │ Desktop：Wails binding
             ▼                             ▼
      浏览器（能力不可用，UI 显示禁用态）   internal/desktop/*（Go）
```

理由：

1. `.trellis/spec/frontend/routing-platform-guidelines.md` 已明确把「为 Desktop 和 Web 分叉两套业务组件」列为**禁止模式**。
2. 「桌面端就是 Web 端套壳加一些功能」这个定位本身就要求业务代码共用；分叉会让每个功能改两遍。
3. Desktop 的「一些功能」（通知、托盘、自启动、窗口状态）本质是**原生能力**，实现在 Go/Wails 侧，前端只消费 capability。

**「单开」体现在任务维度而非代码维度**：Track W（Web 前端）与 Track D（Desktop）是独立任务轨，做 Web 页面的人不需要懂 Wails，做 Desktop 能力的人不改业务组件。这正是 TD1 边界层的价值。

**重估触发条件**：若未来 Desktop 出现 3 个以上纯专属页面（Web 端完全不展示），再评估是否拆 `apps/desktop`。当前不做。

### 4.3 部署与信任边界（生产形态）

```text
[Internet] → [外部网关：APISIX / Kong / Java BFF]     ← 鉴权·SSO·配额执行·审计聚合·计费
                        │  注入 X-Auth-Subject / X-Tenant-Id / X-Trace-Id
                        ▼
             [agent-gateway (Hono)]                   ← 只做 agent 语义：tool allowlist·workspace 沙箱·usage 事件产出
                        ▼
                   [Pi Runtime]
```

`agent-gateway` **假定自己在外部网关之后**，不直接面向公网。G1 产出的《信任边界与外部网关契约》是这条假设的书面形式。

---

## 5. 协议栈

| 层 | 规范 | 用途 | 本项目策略 |
|---|---|---|---|
| Agent ↔ 用户 UI | [AG-UI](https://docs.ag-ui.com/introduction) | 流式事件、工具可视化、HITL、steering、共享状态 | **前端主契约** |
| Agent 生命周期 HTTP | [Agent Protocol](https://langchain-ai.github.io/agent-protocol/) | Thread / Run / Store / cancel / stream 重连 | **对外 REST 子集**（先 threads/runs/stream/cancel/messages） |
| Agent ↔ 工具 | [MCP](https://modelcontextprotocol.io/) | Tools / Resources | 工具面标准；gateway 侧 allowlist |
| Agent ↔ Agent | [A2A](https://a2a-protocol.org/) | Agent Card、跨 agent 协作 | **不建任务**，见 §12 |
| Generative UI | A2UI | 声明式组件树 | 可选增强，非 MVP |
| IDE 宿主 | ACP + 社区 `pi-acp` | 编辑器无关 JSON-RPC | 做 IDE 插件时再对齐 |
| Pi 嵌入 | [Pi RPC](https://pi.dev/docs/latest/rpc) | JSONL stdin/stdout | **Adapter 内部协议**，不对公网 |

### 5.1 版本锚点（G0 / G1 必须填）

上述规范都在演进。**冻结契约时必须在本表补录**：规范版本号 / commit / 快照日期，以及本仓库实现的是哪个子集。没有锚点就无法判断未来是否漂移。

| 规范 | 参考版本 | 快照日期 | 实现子集 |
|---|---|---|---|
| AG-UI | docs.ag-ui.com `concepts/events`（2026-07-27 快照；无独立 semver） | 2026-07-27 | Lifecycle + Text + Tool + Reasoning + 有限 State/Custom；映射见 `docs/agent-contracts/agui-event-mapping.md` |
| Agent Protocol | OpenAPI **0.1.6**（https://langchain-ai.github.io/agent-protocol/openapi.json） | 2026-07-27 | threads / runs / stream / cancel / messages；无 Store/Agents/全量 stream；见 `docs/agent-contracts/agent-protocol-subset.md` |
| MCP | 协议修订 **2025-03-26**（tools 面） | 2026-07-27 | 不在 gateway 重做 MCP transport；经 runtime 透传，G6 接目录/allowlist |
| Pi RPC / SDK | `@earendil-works/pi-coding-agent` **0.82.0**（CLI `pi`；RPC JSONL + `createAgentSession` SDK） | 2026-07-27 | Adapter 内部：CLI `--mode rpc` 与可选 sdk-inprocess；事件子集见 G0 `research/samples/pi/` |
| OMP | `@oh-my-pi/pi-coding-agent` / CLI `omp` **17.1.3**（Oh My Pi，MIT，https://github.com/can1357/oh-my-pi） | 2026-07-27 | Adapter 内部：CLI `--mode rpc`（与 Pi 同构 JSONL）；能力超集与裁剪见 G0 `research/omp-capability-matrix.md` |

### 5.2 对内 Runtime 抽象（G1 冻结）

gateway 只依赖 adapter，不依赖任何具体 agent 的细节。

**核心决策：初版即支持两个 adapter（Pi + OMP），不是「先做 Pi、以后再抽象」。**

理由：只有一个实现的接口必然长成那个实现的形状。两个实现从第一天并存，抽象才经得起检验。这条**撤销**了 v1/v2 中「第二 Runtime Adapter 后置」的判断。

```ts
interface AgentRuntime {
  readonly id: string
  capabilities(): RuntimeCapabilities
  createSession(opts: SessionOpts): Promise<SessionHandle>
  prompt(sessionId: string, input: PromptInput): AsyncIterable<RuntimeEvent>
  steer?(sessionId: string, input: PromptInput): Promise<void>
  followUp?(sessionId: string, input: PromptInput): Promise<void>
  abort(sessionId: string): Promise<void>
  getState(sessionId: string): Promise<RuntimeState> // 必须 secret-sanitize
  getMessages(sessionId: string, page?: HistoryPage): Promise<MessagePage>
  setModel?(sessionId: string, model: ModelRef): Promise<void>
  listModels?(sessionId: string): Promise<ModelInfo[]>
  disposeSession(sessionId: string): Promise<void>
}
```

> **G1 已冻结**：权威类型在 `packages/agent-protocol`（`@taskdaemon/agent-protocol`）；说明见 `docs/agent-contracts/agent-runtime.md`。

`RuntimeEvent` → AG-UI events；HTTP 资源 → Agent Protocol 子集。

### 5.3 两个正交维度

Agent 接入有两个**互相独立**的维度，不要混为一谈：

| 维度 | 取值 | 影响什么 |
|---|---|---|
| **接入方式** | `cli-spawn` / `sdk-inprocess` | **延迟与资源**。CLI 每会话冷启动开销大；SDK 进程内延迟低但隔离性弱 |
| **Agent Profile** | `coding` / `general` | **能力集与 UI**。非 coding 场景下 workspace 绑定、diff 预览无意义 |

二者自由组合。同一个 adapter 可能同时提供两种接入方式（Pi 假定有 SDK 与 CLI 两条路），也可能只有一种（OMP 待 G0 核实）。

**接入方式必须暴露给编排层**：G2 的进程池策略依赖它 —— 冷启动贵的 adapter 需要预热池与更长的空闲保留，冷启动便宜的可以按需起、快速回收。把这个特性藏在 adapter 内部会让编排层做出错误的池化决策。

**Profile 影响前端**：G4/G5 必须按 profile 调整 UI，而不是把 coding 特化的界面强加给所有场景。

### 5.4 能力协商（RuntimeCapabilities）

不同 adapter 支持的能力不同：有的没有 steering，有的不暴露 thinking，有的不支持运行时切模型。**前端不得硬编码任何 adapter 的能力假设。**

解法：每个 adapter 声明自己的能力，gateway 透出，前端按声明渲染。

```ts
interface RuntimeCapabilities {
  profile: 'coding' | 'general' | Array<'coding' | 'general'>
  attachMode: 'cli-spawn' | 'sdk-inprocess'
  coldStartCost: 'low' | 'high'      // 供 G2 的池化策略使用
  steering: boolean
  followUp: boolean
  thinking: boolean
  toolCallDetail: 'full' | 'name-only' | 'none'
  modelSwitch: 'session' | 'request' | 'none'
  workspaceBinding: boolean          // general profile 下通常为 false
  sessionPersistence: 'runtime' | 'gateway' | 'none'
  abortContinuesSession: boolean
  history: 'messages' | 'messages+entries' | 'none'
  memoryClass: 'light' | 'heavy'
}
```

> **G1 已冻结**：Pi/OMP 实例值见 `docs/agent-contracts/runtime-capabilities.md` 与包内 `PI_CLI_CAPABILITIES` / `OMP_CLI_CAPABILITIES`。

> **同构提示**：这与 Track D 的 `C-5 Desktop capability` 契约是**同一个模式** —— 能力声明 + 前端按声明降级。两处应共用词汇与心智模型，减少认知负担。能力不可用时前端展示禁用态并说明原因，不隐藏、不伪造。

**G0 必须实测填出 Pi 与 OMP 各自的能力表；G1 据此定稿接口。**

### 5.5 非 coding 场景

企业场景大部分不是 coding。但 coding agent 本质是「带工具的 LLM loop」，把工具换成 MCP 即可服务通用场景。真正要处理的是**coding 特化部分的可关闭性**：

| coding 特化项 | general profile 下 |
|---|---|
| workspace / cwd 绑定 | 关闭或降级为无文件系统会话 |
| 文件变更与 diff 预览 | 不展示 |
| 代码块特化渲染 | 保留（无害），但不作为主要形态 |
| 默认工具集 | 由 MCP 目录决定，不预设文件/命令工具 |

`workspaceBinding: false` 的会话必须能正常工作，这是 G2/G3 的硬要求，也是 G4/G5 的 UI 分支依据。

### 5.6 Pi 接入要点（**G0 已实测确认**）

- 启动：`pi --mode rpc`（JSONL，仅 `\n` 分帧）—— **确认**。
- TS 优先：`@earendil-works/pi-coding-agent` 的 `createAgentSession` / `AgentSession`（进程内、更低创建延迟）—— **确认可用**；隔离场景 spawn CLI —— **确认**，CLI 冷启动 median ~3.1s（`xh/grok-4.5`）。
- 参考形态：开源 Pi WebUI 主参考 **`agegr/pi-web`**（`@agegr/pi-web`，MIT）—— G0 已拆解；另有 `pi-for-vscode` 等。
- 密钥出口策略：由外部网关或统一 LLM 出口（如 Bifrost）承担，**不在 agent-gateway 内实现密钥管理**（不变）。

### 5.7 OMP 接入要点（**G0 已核实**）

- 全称 Oh My Pi；仓库 https://github.com/can1357/oh-my-pi；许可证 MIT；活跃维护（测时 17.1.3）。
- **提供**非交互/RPC：`omp --mode rpc` / `-p`；**可被程序驱动**（阶段 0 门通过）。
- SDK/RpcClient 存在；生产 adapter 建议 **CLI RPC**（包 exports 偏 TS 源；冷启动 high；RSS 显著高于 Pi）。
- 事件流与 Pi **同构超集**；差异可用 `RuntimeCapabilities` + 归一化抹平；须 **sanitize** `get_state` 中可能出现的密钥头。
- general profile：可对话，但默认仍易注入全局 agent 配置/MCP —— adapter 必须强制裁剪。

### 5.8 gateway 的最小必要规模（关键判断）

**Pi / OMP 已经提供了大部分「agent 功能」，gateway 不应重做。** 明确不重做：agent loop、LLM 调用、MCP 工具执行、会话内状态机、模型切换、steering/abort 语义。

**但以下七项 runtime 与外部网关都不做，gateway 无法省略：**

| # | 职责 | 为什么 runtime 做不了 |
|---|---|---|
| 1 | **会话↔进程编排** | runtime 是单会话模型；进程池、并发上限、僵尸回收、会话路由无人负责 |
| 2 | **非交互调用入口** | runtime 的 WebUI/CLI 是交互式的；定时触发 / Webhook / taskdaemon 调用（G6）需要程序 API |
| 3 | **thread/run 持久化与恢复** | 刷新、断线、重启后的可恢复性取决于各 runtime 自身边界 —— **G0 必测** |
| 4 | **对外协议转换** | 各 runtime 协议私有；对外要给 AG-UI / Agent Protocol |
| 5 | **多 adapter 抽象与能力协商** | **本节新增**。runtime 不会为你抹平彼此差异 |
| 6 | **策略执行** | tool allowlist、workspace 沙箱根、egress —— runtime 不知道租户策略 |
| 7 | **usage / audit 事件产出** | 见 §6.2 |

**规模决策规则**：G0 实测后，凡 runtime 已原生提供且质量达标的能力，gateway **一律直接透传，不做二次封装**。G2 的最终规模由 G0 结论确定。若 G0 证明持久化足够，G2 可省去自己的 thread/run 存储。

### 5.9 Pi WebUI 的处置（已决策）

**决策：参考重写并简化，不 fork、不直接依赖。**

| 选项 | 是否采纳 | 理由 |
|---|---|---|
| 直接使用 | ✗ | 无法定制，无法与 taskdaemon 体系融合，且它只服务 Pi 一家 |
| fork 后改 | ✗ | 继承全部上游复杂度与历史包袱；跟上游同步长期成本高 |
| **参考核心功能，简化重写** | **✓** | 只取真正需要的交互形态，代码量小、可读性高，后续改造成本低 |

执行方式：

- **G0 产出**：Pi WebUI 的**核心功能清单**与交互形态拆解，以及它与 Pi 的耦合点（用了哪些 Pi 私有接口 —— 这些正是 gateway 需要在标准协议下补齐的部分）。
- **G4 消费**：按清单实现最小可用版本，代码结构服从本仓库 `.trellis/spec/frontend/` 规范，不搬运上游目录结构。
- **额外约束**：Pi WebUI 是单 runtime 单 profile 的 UI，**不能直接照搬** —— 我们的 UI 必须按 §5.4 的能力声明与 §5.5 的 profile 分支渲染。
- **判断标准**：宁可少一个功能，不要多一层看不懂的抽象。功能缺口留给 G5。

**不做**：不把 Pi WebUI 作为依赖引入 `package.json`；不复制其状态管理方案除非本仓库规范已认可。

---

## 6. 企业能力的归属（v1 Wave C 的重构）

### 6.1 移出本仓库（外部网关承担）

| 能力 | 承担方 | 本仓库动作 |
|---|---|---|
| SSO / OIDC / API Key 签发轮换 | 外部网关 / Java BFF | 不实现；只消费注入头 |
| 多租户隔离与租户管理 | 外部网关 | 不实现；只透传 `X-Tenant-Id` 并用作数据分区键 |
| 审计**存储、检索、留存策略** | 外部审计平台 | 不实现；只产出事件 |
| 配额**执行**、限流、计费 | 外部网关 | 不实现；只产出 usage 事件 |
| 完整可观测平台 | 外部 APM | 不实现；只保证 traceId 透传 |

### 6.2 必须留在 agent-gateway（外部网关物理上做不到）

| 能力 | 为什么外部做不到 | 归属任务 |
|---|---|---|
| **tool allowlist** | L7 网关只见到 `POST /runs`，看不懂 agent 要调哪个 tool | G2 定框架，G6 接 MCP 目录 |
| **workspace 沙箱根** | 会话 cwd 是运行时概念，不在 HTTP 语义里 | G2 |
| **usage / token 事件产出** | 外部只能数 HTTP 次数，数不了 token 与 cost | G2 |
| **可插拔 `PrincipalResolver`** | 需要默认 dev 单主体实现，否则本地无法开发 | G2 |
| **结构化 audit 事件产出** | 「谁对哪个 workspace 调了哪个 tool」只有运行时知道 | G2 |

**总量估计**：约占 G2 工作量的 5%，外加 G1 的一份契约文档。这不是「实现企业级平台」，是「不把钩子焊死」。

### 6.3 开发期默认

无外部网关时，`PrincipalResolver` 使用 `dev-single-principal` 实现：固定 subject、固定 tenant、自生成 traceId。**生产部署必须显式切换**，配置缺失时启动日志给出显眼告警。

---

## 7. 四轨分期与依赖图

### 7.1 轨道定义

| 轨 | 范围 | 主要技术面 |
|---|---|---|
| **T** | taskdaemon 后端 | Go / Ent / CLI |
| **W** | Web 前端 | `apps/web` React（平台无关部分） |
| **D** | Desktop | Wails + Go 原生能力 + `src/lib/desktop` 边界层 |
| **G** | Agent Gateway 及其前端 | Hono / TS / Pi / `apps/agent-web` |

### 7.2 Track T · taskdaemon 后端

| ID | 任务目录 | 主题 | 依赖 | 状态 |
|---|---|---|---|---|
| T0 | `05-28-hermes-agent-audio-api` | 音频收口（**现有任务，不新建**；跨前后端） | — | **done**（写配置→T1a/T1b） |
| T1a | `07-27-t1a-config-write-api` | 配置写入 API + **契约 C-1 冻结** | T0 | **done（C-1 已冻结，待合入）** |
| T2a | `07-27-t2a-notification-event-bus` | 通知事件总线 + 站内 sink + **契约 C-2 冻结** | — | **done（C-2 已冻结）** |
| T4 | `07-27-t4-outbound-notification-sink` | Webhook / 邮件出站 sink | T2a | **done** |
| T5 | `07-27-t5-service-installer` | 系统服务安装器（CLI，纯后端） | — | **done**（真实三平台装机为残留） |
| T6 | `07-27-t6-run-log-archive` | Run 完整日志归档 | — | **done（待合入）** |
| T7a | `07-27-t7a-backup-template-api` | 备份模板后端 API | T1a | **done（待合入）** |
| T8 | `07-27-t8-db-migration-tool` | SQLite ↔ PostgreSQL 迁移工具（CLI，纯后端） | T1a | |
| T7a | `07-27-t7a-backup-template-api` | 备份模板后端 API | T1a | |
| T8 | `07-27-t8-db-migration-tool` | SQLite ↔ PostgreSQL 迁移工具（CLI，纯后端） | T1a | **done（待协调者 merge）** |

### 7.3 Track W · Web 前端

| ID | 任务目录 | 主题 | 依赖 | 状态 |
|---|---|---|---|---|
| W1 | `07-27-t1b-settings-page-web` | 完整设置页 | **C-1 冻结**（非 T1a 完整交付） | **done（待合入）** |
| W2 | `07-27-t2b-notification-center-web` | 通知中心 | **C-2 冻结** | pending |
| W3 | `07-27-t7b-backup-template-wizard-web` | 备份模板向导 | T7a 契约 | pending |

> **关键**：Track W 依赖的是**契约冻结**，不是后端交付完成。契约冻结后前端可用 MSW / 手写 mock 并行开发，联调在后端交付后进行。这是前后端拆任务的全部意义。

### 7.4 Track D · Desktop

| ID | 任务目录 | 主题 | 依赖 | 状态 |
|---|---|---|---|---|
| D1 | `07-27-td1-desktop-platform-bridge` | **平台能力边界层落地**（`src/lib/desktop`） | — | **done**（C-5 已冻结） |
| D2 | `07-27-t3-desktop-notification-sink` | Desktop 原生通知 sink | D1 + C-2 冻结 | pending |
| D3 | `07-27-td2-desktop-native-features` | 托盘 / 自启动 / 单实例 / 窗口状态 | D1 | pending |

### 7.5 Track G · Agent Gateway

| ID | 任务目录 | 主题 | 依赖 | 状态 |
|---|---|---|---|---|
| G0 | `07-27-g0-agent-runtime-spike` | 技术 spike + **Pi/OMP 能力盘点与 gateway 规模判定** | — | **done**（见任务 `research/`） |
| G1 | `07-27-g1-agent-contract-freeze` | **契约 C-3 / C-4 冻结** + `packages/` 落地 | G0 | **done**（C-3/C-4 已冻结） |
| G2 | `07-27-g2-hono-gateway-skeleton` | Hono gateway 骨架（**规模由 G0 判定**）+ 多 adapter 编排 + 信任边界钩子 | G1 | pending |
| G3 | `07-27-g3-runtime-adapters` | **Runtime Adapter 层 + Pi/OMP 两个实现** | G1（与 G2 并行；联调需 G2） | pending |
| G4 | `07-27-g4-agent-web-mvp` | 最小 agent-web（**参考 Pi WebUI 简化重写**，见 §5.9；按能力声明与 profile 渲染） | G1（与 G2/G3 并行；联调需 G2+G3） | pending |
| G5 | `07-27-g5-agui-full-experience` | AG-UI 完整体验（Codex 式） | G4 | pending |
| G6 | `07-27-g6-mcp-and-interop` | MCP 目录 + 双引擎打通 | G3、T2a | pending |

> **Track G 的重心在前端。** G2/G3 的后端工作量取决于 G0 对 runtime 原生能力的盘点结果 —— runtime 已提供的一律透传（见 §5.8）。真正需要从零定制的是 G4/G5 的产品体验层，以及 G3 的**多 adapter 抽象**。

### 7.6 依赖图

```text
Track T（Go 后端）             Track W（Web 前端）       Track D（Desktop）     Track G（Agent）
─────────────────────────      ────────────────────      ─────────────────      ──────────────────────
T0 ──▶ T1a ═[C-1]═══════════════════▶ W1                 D1 ──┬──▶ D3          G0 ──▶ G1 ═[C-3/C-4]═┐
        │                                                     │                                     │
        ├──▶ T7a ────────────────────▶ W3                     │                        ┌──▶ G2 ──┐  │
        └──▶ T8                                               │                        ├──▶ G3 ──┼──┤
                                                              │                        └──▶ G4 ──┘  │
T2a ═[C-2]═══╤═══════════════════════▶ W2                     │                                  ▼  │
             ├──▶ T4                                          │                             （联调）◀┘
             └──────────────────────────────────────▶ D2 ◀────┘                                  │
                                                                                                  ▼
T5（独立）   T6（独立）                                                                     G5    G6
                                                                                                  ▲
T2a ──────────────────────────────────────────────────────────────────────────────────────────────┘
```

`═[C-x]═` 表示**契约冻结边**：上游冻结契约后下游即可开工，无需等上游完整交付。

### 7.7 并行波次

| 波次 | 可同时开工 | 条数 | 说明 |
|---|---|---|---|
| **W1** | T0、T2a、T5、T6、D1、G0 | 6 | 全部零依赖；T0 已在跑 |
| **W2** | T1a、W2、T4、D2、G1 | 5 | C-2 冻结后 W2/T4/D2 解锁；T1a 待 T0 收口 |
| **W3** | W1(设置页)、T7a、T8、G2、G3、G4、D3 | 7 | C-1 冻结后前端解锁；C-3 冻结后 G 轨三线并行 |
| **W4** | W3(向导)、G5、G6 | 3 | 需前序联调通过 |

**并行度上限建议 3–4 条线**。不是技术限制，是评审与集成带宽限制。理论上限见上表，实际按人手取子集。

### 7.8 Track G 暂停时的自洽性

**如果 Agent 线整体暂停，T/W/D 三轨完全自洽**：不依赖 Track G 任何产出，taskdaemon 仍是功能完整的调度守护进程。反之亦然（G6 除外）。这是双引擎松耦合的直接收益，也降低了 Track G 的承诺风险。

---

## 8. 文件所有权边界（并行安全带）

> **规则**：任务只能自由修改「独占」列的路径。「共享」列的文件采用 **append-only** 约定：只追加自己的行/字段/注册项，不重排、不重构他人代码。发生冲突时以本表判定归属。

### 8.1 独占路径

| 任务 | 独占路径 |
|---|---|
| T0 | `internal/audio/**`、`internal/httpapi/audio_*`、`apps/web/src/features/audio/**` |
| T1a | `internal/httpapi/config_routes.go`、`config_dto.go`、`runtime_config.go` |
| T2a | `internal/notify/**`（新建）、`internal/data/ent/schema/notification*.go` |
| T4 | `internal/notify/sink_webhook*.go`、`internal/notify/sink_email*.go` |
| T5 | `internal/cli/service_commands.go`（新建）、`internal/service/**`（新建） |
| T6 | `internal/runlog/**`（新建）、`internal/logging/file_handler.go` |
| T7a | `internal/template/**`（新建）、`internal/httpapi/template_*`（新建） |
| T8 | `internal/cli/db_commands.go`（新建）、`internal/data/transfer/**`（新建） |
| W1 | `apps/web/src/features/settings/**`（新建） |
| W2 | `apps/web/src/features/notifications/**`（新建） |
| W3 | `apps/web/src/features/tasks/templates/**`（新建） |
| D1 | `apps/web/src/lib/desktop/**`（新建）、`internal/desktop/bridge*.go` |
| D2 | `internal/notify/sink_desktop*.go`、`internal/desktop/notification*.go` |
| D3 | `internal/desktop/tray*.go`、`autostart*.go`、`window_state*.go` |
| G0 | `.trellis/tasks/07-27-g0-agent-runtime-spike/research/**`（**只写研究产物，不改仓库代码**） |
| G1 | `packages/**`（新建）、`docs/agent-contracts/**`（新建） |
| G2 | `services/agent-gateway/**`（新建） |
| G3 | `services/agent-gateway/src/runtime/**`（adapter 层 + `pi/` + `omp/`） |
| G4 / G5 | `apps/agent-web/**`（新建） |
| G6 | `services/agent-gateway/src/mcp/**`、`internal/httpapi/agent_bridge_*`（新建） |

### 8.2 共享文件与追加约定

| 文件 | 触及任务 | 约定 |
|---|---|---|
| `internal/config/types.go` | T1a T2a T4 T5 T6 D3 | 每任务只**追加**自己的配置 struct 到文件尾；不动他人字段与顺序 |
| `internal/config/defaults.go` | 同上 | 同上 |
| `internal/httpapi/router.go` | T1a T2a T6 T7a G6 | 只追加自己的路由注册行；分组注释标注任务 ID |
| `apps/web/src/routes/router.tsx` | W1 W2 W3 | 只追加自己的 route 定义 |
| `apps/web/src/app/queryClient.ts` | W1 W2 W3 | 只追加 query key 前缀常量 |
| `apps/web/src/components/**`（AppShell / 导航） | W1 W2 D1 | 导航项 append-only；D1 只加 capability gate，不改布局 |
| `internal/desktop/desktop.go` | D1 D2 D3 | 只追加自己的 binding 注册；D1 先定注册模式 |
| 根 / 服务 `package.json` | T5 T8 G2 G4 | 只追加自己的 script |

### 8.3 ⚠️ Ent 生成物：真实的并行陷阱

本项目 Ent 使用**自动 migration**（`internal/data/ent/migrate/`），无版本化 SQL 文件。但 `go generate` 会重写 `client.go`、`ent.go`、`mutation.go` 等超大生成文件。

**两个任务同时新增 Ent entity，生成物必然产生无法手工合并的冲突。**

强制约定：

1. 新增 Ent entity 的任务：**T2a（通知事件表）、T6（run 日志索引）**。这两个任务**不得同时处于「已改 schema 未合并」状态**；T2a 优先。
2. 合并他人改动后，**永远重跑 `pnpm generate:go`，不要手工合并生成物**：
   ```bash
   git checkout --ours services/taskdaemon-go/internal/data/ent/
   # 保留双方 schema/*.go，然后
   pnpm generate:go
   ```
3. `internal/data/ent/schema/*.go` 一任务一文件，禁止改他人 schema 文件。

### 8.4 前端并行约定

1. **feature 目录隔离**：W1/W2/W3 各自新建 `features/<domain>/`，互不进入。
2. **共享 UI 组件只增不改**：需要改 `components/ui/*` 时先在自己 feature 内本地化，稳定后单独提 PR 上移。
3. **契约冻结后用 mock 开发**：后端未交付时用 handler mock 或 fixture，禁止为了「先跑起来」自行发明字段名——发明的字段名合并时必然与后端不一致。
4. **Desktop 能力只经 `src/lib/desktop`**：W 轨任务禁止直接引用 Wails 全局对象（既有规范的禁止模式）。

### 8.5 集成策略

- **分支**：每任务一个 `feat/<task-id>-<slug>` 分支，从 `dev` 切出，合回 `dev`。
- **worktree**：并行 ≥3 条线时建议用 git worktree，避免频繁切分支导致 Go/Vite 缓存抖动。
- **集成节奏**：每条线**至少每日 rebase 一次 `dev`**，把冲突压在小粒度。
- **集成责任**：谁后合谁负责解冲突并重跑 §11 的全量验收命令。
- **契约变更**：冻结后若必须改契约，改动方负责通知所有下游任务并在本文 §9 记录变更行，禁止静默改。

---

## 9. 跨任务契约冻结点

> 冻结 = 该契约由指定任务一次性定义并写入文档/代码；其他任务**只消费不发明**。冻结前，下游任务不得开工。

| # | 契约 | 冻结任务 | 冻结产物 | 消费方 |
|---|---|---|---|---|
| C-1 | **配置写入 API 形状**（`PUT /api/config/<section>`、热生效 vs 需重启标记、校验错误结构、reload 结果 envelope） | **T1a** | `.trellis/spec/backend/configuration-runtime-guidelines.md`（C-1 节）+ `api-contracts.md` + `error-handling.md`（`CONFIG_*`）+ `config_{dto,routes}.go` / `classification.go` / `write.go` | **W1**、T4、T5、T6、D3 |
| C-2 | **通知事件 schema**（事件名空间、payload 字段、`Sink`/`Bus` 接口、sink 注册与失败语义、已读状态模型） | **T2a** | `services/taskdaemon-go/internal/notify/event.go` + `.trellis/spec/backend/notification-event-contract.md` + `api-contracts.md`/`error-handling.md` 增补 | **W2**、T4、D2、G6 |
| C-3 | **Agent 对外契约**（Agent Protocol 子集 + AG-UI 事件映射表 + `AgentRuntime` 接口 + **`RuntimeCapabilities` 能力协商** + 共享 TS 类型包） | **G1** | **`packages/agent-protocol/`**（`@taskdaemon/agent-protocol`）+ **`docs/agent-contracts/`**（`README.md`、`agent-protocol-subset.md`、`agui-event-mapping.md`、`agent-runtime.md`、`runtime-capabilities.md`、`non-coding-profile.md`） | G2、G3、**G4**、G5、G6 |
| C-4 | **信任边界契约**（注入头名称与语义、`PrincipalResolver` 接口、audit/usage 事件 schema） | **G1** | **`docs/agent-contracts/trust-boundary.md`** + 类型 `packages/agent-protocol/src/trust.ts` | G2、G6、未来 Java 网关 |
| C-5 | **Desktop capability 契约**（capability 命名、`useDesktopCapability` 返回形状、Web fallback 语义、Wails binding 注册模式） | **D1** | `apps/web/src/lib/desktop/index.ts` + `.trellis/spec/frontend/routing-platform-guidelines.md`（C-5 节）+ `.trellis/spec/backend/error-handling.md`（`DESKTOP_*`）+ `services/taskdaemon-go/internal/desktop/bridge.go` | D2、D3、W 轨全部任务 |
| C-6 | **错误码前缀分配** | **本父任务（下表）** | 本文 §9.1 | 全部 |

### 9.1 错误码前缀分配（立即生效，避免撞码）

现有 envelope 与错误码规范见 `.trellis/spec/backend/error-handling.md` 与 `api-contracts.md`。新任务按下表取前缀：

| 前缀 | 归属 | 示例 |
|---|---|---|
| `AUDIO_*` | T0（已在用） | `AUDIO_QUEUE_FULL` |
| `CONFIG_*` | T1a | `CONFIG_FIELD_REQUIRES_RESTART` |
| `NOTIFY_*` | T2a T4 D2 | `NOTIFY_SINK_UNAVAILABLE` |
| `SERVICE_*` | T5 | `SERVICE_INSTALL_PERMISSION_DENIED` |
| `RUNLOG_*` | T6 | `RUNLOG_ARCHIVE_NOT_FOUND` |
| `TEMPLATE_*` | T7a | `TEMPLATE_RUNNER_UNSUPPORTED` |
| `DBXFER_*` | T8 | `DBXFER_SCHEMA_MISMATCH` |
| `DESKTOP_*` | D1 D3 | `DESKTOP_CAPABILITY_UNAVAILABLE` |
| `AGENT_*` | G2 G3 | `AGENT_RUN_CANCELLED` |
| `MCP_*` | G6 | `MCP_TOOL_NOT_ALLOWED` |

### 9.2 前端 query key 前缀分配

避免 `queryClient.ts` 撞 key：

| 前缀 | 归属 |
|---|---|
| `settingsKeys` | W1 |
| `notificationKeys` | W2 |
| `templateKeys` | W3 |
| `desktopKeys` | D1 |
| `agentKeys` | G4 / G5 |

### 9.2.1 C-3 G5 append（2026-07-27）

| 变更 | 说明 | 任务 |
|---|---|---|
| `PATCH /threads/{id}`、`POST .../fork`、`POST .../hitl` | 会话重命名/归档、分叉、HITL 响应 | G5 |
| `ListThreadsQuery.q` / `includeArchived`、`ThreadStatus.archived` | 搜索与归档 | G5 |
| CUSTOM `hitl_request` / `FileChangePayload.diff` | HITL 与 diff 预览 | G5 |
| 类型 | `packages/agent-protocol` append-only | G5 |

### 9.3 traceId 贯穿约定

- taskdaemon 已有 `internal/httpapi/trace_middleware.go`，沿用其头名。
- agent-gateway **必须**接受上游 `X-Trace-Id`，缺失时自生成，并透传到 Pi Runtime 日志。
- G6 双引擎互调时，调用方必须透传自己的 traceId，不得重新生成。

---

## 10. `packages/` ROI 拐点

**今天**：`apps/web` 是唯一 TS 应用，拆共享包 ROI 低 —— v1 的判断正确。

**G1 之后**：`services/agent-gateway`(TS) + `apps/agent-web`(TS) + `apps/web`(TS) 三方并存，AG-UI 事件类型与 Agent Protocol DTO 必须单点定义，否则三处手写类型必然漂移。**此时 `packages/` 从「ROI 低」变成「硬需求」**。

G1 落地范围（最小）：

```text
packages/agent-protocol/     # Agent Protocol DTO + AG-UI 事件类型 + AgentRuntime 接口（types only，零运行时依赖）
```

**明确不做**：不为了形式 monorepo 去拆 `packages/ui`、`packages/utils`。T/W/D 三轨不因此受影响。

---

## 11. 验收基线（所有任务通用）

每个子任务的 `implement.md` 必须包含以下命令，且全绿才算完成：

```bash
# 仓库根
pnpm typecheck
pnpm lint
pnpm test:go
pnpm vet:go

# Go 服务（触及 Go 时）
cd services/taskdaemon-go && go test ./...

# 触及前端时额外
pnpm --filter @taskdaemon/web test
```

Track D 任务额外：Desktop 能力必须**同时**验证两条路径 —— Wails 壳内可用、浏览器内优雅降级（禁用态而非崩溃）。

Track G 任务额外（G1 之后生效）：

```bash
pnpm --filter @taskdaemon/agent-gateway test
pnpm --filter @taskdaemon/agent-web test
```

**Definition of Done（所有任务）**：

- [ ] `prd.md` 全部 Acceptance Criteria 勾选完毕
- [ ] 上述验收命令全绿
- [ ] 触及的 spec 文档已更新（`.trellis/spec/`）
- [ ] 若冻结了契约，契约产物已落盘且被下游可见
- [ ] 本路线图 §7 表格状态已回写
- [ ] 破坏性变更有回滚说明

---

## 12. 明确不建任务 / 后置项

| 项 | 原因 |
|---|---|
| 在 Go taskdaemon 内嵌 LLM agent loop | 与 TS/Pi 生态冲突；边界混乱 |
| **在 gateway 重做 Pi 已有能力**（agent loop / MCP 执行 / 会话内状态机 / 模型切换） | Pi 已提供；gateway 一律透传，见 §5.4 |
| 浏览器直连 Pi RPC | 非企业 API；安全与多租户不可控 |
| 完整 Agent Protocol Store + 全量 stream 原语 | 过载；先 threads/runs/stream/cancel |
| **拆 `apps/desktop` 独立前端** | 「桌面端是 Web 套壳」定位下会让每个功能改两遍；见 §4.2 重估条件 |
| **A2A / Agent Card** | 无单 agent 产品体验前无价值；G5 交付后重估 |
| **第三个及以后的 Runtime Adapter** | 初版已有 Pi + OMP 两个（§5.2），抽象已被检验；再加需有真实需求 |
| **SSO / 多租户 / 审计平台 / 配额执行**（原 C1–C3） | 归外部网关，见 §6 |
| Asynq / 分布式队列 | 与「本机低占用守护」目标冲突 |
| 强拆 `packages/ui`、`packages/utils` | 单 app 时 ROI 低；只拆 §10 的类型包 |
| 完整邮件通知平台 / 完整 Webhook 平台 | T4 只做出站通道，不做平台 |

---

## 13. 与旧任务树的关系

| 旧内容 | 关系 |
|---|---|
| `05-14-cross-platform-scheduler-daemon` 第二轮清单 | Track T/W/D 的需求来源；**不重复开已交付一期子项** |
| `05-28-hermes-agent-audio-api` | **即 T0**，已 `add-subtask` 挂到本父任务；完成后 archive | | **done**（写配置→T1a/T1b） |
| 本文档 v1 | 被本版完全取代；变更见 §1.1 |

---

## 14. 文档维护规则

| 触发时机 | 动作 | 责任人 |
|---|---|---|
| 子任务 `task.py start` | §7 表格标记 `in_progress` | 该任务执行者 |
| 子任务 archive | §7 表格标记 `done` + 回写实际交付偏差 | 该任务执行者 |
| 契约冻结完成 | §9 表格补冻结产物实际路径 | 冻结任务执行者 |
| 契约冻结后变更 | §9 追加变更行 + 通知全部下游 | 变更方 |
| G0/G1 完成 | §5.1 版本锚点表补齐 | G0/G1 执行者 |
| 新增子任务 | §7 + §8 + §9 三处同步 | 提出者 |
| 范围变更 | §16 修订记录追加一行，含决策人 | 提出者 |

**本文档是四轨的规划 SoT。** 任何与本文冲突的子任务 PRD，以本文为准或先改本文。

---

## 15. 参考链接

- 主仓库 README：`README.md`
- 主 PRD（第二轮）：`.trellis/tasks/archive/2026-05/05-14-cross-platform-scheduler-daemon/prd.md`
- 音频任务（T0）：`.trellis/tasks/05-28-hermes-agent-audio-api/prd.md`
- 后端规范索引：`.trellis/spec/backend/index.md`
- 前端规范索引：`.trellis/spec/frontend/index.md`
- 平台边界规范：`.trellis/spec/frontend/routing-platform-guidelines.md`
- Pi RPC：https://pi.dev/docs/latest/rpc
- Agent Protocol：https://langchain-ai.github.io/agent-protocol/
- AG-UI：https://docs.ag-ui.com/introduction
- Google 协议族综述：https://developers.googleblog.com/developers-guide-to-ai-agent-protocols/

---

## 16. 修订记录

| 日期 | 版本 | 说明 | 决策人 |
|---|---|---|---|
| 2026-07-27 | v1 | 首版：后 MVP 产品化 + 通用 Agent（Hono/Pi）合并路线图 | mudssky |
| 2026-07-27 | v2 | 双轨重构；通知合并为事件总线；企业能力移出仓库；新增 G0 spike；补依赖图/文件所有权/契约冻结点 | mudssky |
| 2026-07-27 | v3 | 前后端按层拆任务（T1/T2/T7 → a/b）；新增 Track W 与 Track D；D1 平台边界层立项；补 §4.2 Web/Desktop 架构决策、C-5 capability 契约、query key 分配；子任务总数 21 | mudssky |
| 2026-07-27 | v4 | **多 Runtime Adapter 转向**：撤销「第二 adapter 后置」，初版即 Pi + OMP 并存；新增 §5.3 两个正交维度（接入方式 × profile）、§5.4 `RuntimeCapabilities` 能力协商、§5.5 非 coding 场景；gateway 职责增至七项；G3 改为「Runtime Adapter 层 + Pi/OMP 实现」 | mudssky |
| 2026-07-27 | v5 | **C-2 冻结**：T2a 交付通知事件总线 + 站内 sink；产物 `internal/notify/*` + `.trellis/spec/backend/notification-event-contract.md`；W2/T4/D2 可开工 | mudssky |
| 2026-07-27 | v5.1 | **C-5 冻结**：D1 落地 `src/lib/desktop` + Go Registry + Environment 样板；§7.4 D1=done | mudssky / worker |
| 2026-07-27 | v5.2 | G0 实测回写：§5.1 填入 Pi 0.82.0 / OMP 17.1.3；确认 OMP 可 RPC 程序驱动、双 adapter 可抽象；CLI 冷启动均为 high（需预热池）；gateway 薄索引持久化；§7.5 G0=done。详情 `.trellis/tasks/07-27-g0-agent-runtime-spike/research/HANDOFF.md` | G0 worker |
| 2026-07-27 | chore | 新增子任务 `07-27-deps-latest-upgrade`：JS/Go 依赖升 latest；独占 lockfile；与功能波次错开 | mudssky |
| 2026-07-27 | v5.3 | T5 系统服务安装器合入（launchd/systemd/SCM；真实三平台装机残留） | mudssky / worker |
| 2026-07-27 | chore | 新增子任务 `07-27-deps-latest-upgrade`：JS/Go 依赖升 latest；独占 lockfile；与功能波次错开 | mudssky |
| 2026-07-27 | chore | **deps-latest-upgrade 合入**：JS/TS+Go 直接依赖升 latest（TS7/Biome2.5/Wails alpha2.118 等） | mudssky / worker |
| 2026-07-27 | v5.3 | **C-3 / C-4 冻结（G1）**：落地 `packages/agent-protocol`（types only）与 `docs/agent-contracts/**`；§5.1 AG-UI/Agent Protocol/MCP 锚点补齐；§5.2/§5.4 按 G0 增补 followUp/getMessages/setModel/disposeSession 与 capabilities 扩展字段；§7.5 G1=done；G2/G3/G4 可并行开工 | G1 worker |
| 2026-07-27 | v5.4 | T6 Run 完整日志归档实现完成（待合入）：`internal/runlog`、RUNLOG_* API、Ent run 三态字段、runs 下载入口 | mudssky / worker |
| 2026-07-27 | v5.5 | T4 Webhook/邮件出站 sink 合入 | mudssky / worker |
| 2026-07-27 | coord | 协调波次：可并行任务（G0–G6、T2a/T2b/T3–T6、TD1/TD2、deps）均已合入并 Trellis 归档；剩余 T0 用户进行中及其阻塞链 T1a/T1b/T7a/T7b/T8 | mudssky / coordinator |
