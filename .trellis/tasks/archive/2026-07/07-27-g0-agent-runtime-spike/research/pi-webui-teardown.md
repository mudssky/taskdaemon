# Pi WebUI 拆解（参考重写用）

> 选型前提：路线图 §5.9 已定 **参考重写并简化**，不 fork、不依赖。  
> 主参考仓库：**agegr/pi-web**（npm `@agegr/pi-web`）

## 1. 仓库与许可证

| 项 | 值 |
|---|---|
| 仓库 | https://github.com/agegr/pi-web |
| 包 | `@agegr/pi-web` 0.8.1 |
| 许可证 | **MIT**（允许参考重写） |
| Stars | ~2855（2026-07-27） |
| 活跃 | 2026-07-26 push |
| 运行时 | Next.js 16 + React 19；默认 `127.0.0.1:30141` |
| 硬依赖 | `@earendil-works/pi-coding-agent` 等 **钉死 Pi SDK** |
| 次选 | jmfederico/pi-web、@firstpick/pi-package-webui（扩展型）；本拆解以 agegr 为主 |

**未在本机长时间交互点击全旅程**（避免污染本机 agent 会话与权限面）；交互形态依据 README 功能描述 + 源码树 + 关键文件（`lib/rpc-manager.ts`、`hooks/useAgentSession.ts`、`app/api/agent/new/route.ts`）交叉核对。标注为「源码/文档旅程」而非盲猜。

## 2. 用户旅程（文档+源码）

1. 启动 `pi-web` → 浏览器打开 loopback  
2. 侧栏按 **项目/cwd** 浏览历史 session 文件（读 `~/.pi/agent/sessions`）  
3. 选会话或新建 → 绑定 cwd → 创建/恢复 **Pi AgentSession**  
4. 聊天：SSE 收事件；输入区可改 model/thinking/tools/compact/slash  
5. 左侧文件树 + 右侧预览（源码/图片/PDF…）  
6. 顶栏：context 用量、cost、compaction、system prompt  
7. 配置：models.json、API key/OAuth、skills 安装开关  
8. 进阶：fork、从某条消息 edit、git worktree 切换  

安全声明（上游）：**无应用级鉴权**，禁止裸暴露公网。

## 3. 功能分级表

| 功能 | 解决什么 | 我们有这个问题吗 | 依赖 Pi 私有面 | 假设 | 分级 |
|---|---|---|---|---|---|
| 会话列表（按项目） | 找历史 | 有 | session 目录布局 / jsonl | 单机 Pi 文件 | **G4 必要**（但数据源改 AP threads） |
| 消息流 + Markdown | 读对话 | 有 | 消息 schema | 单 runtime 事件 | **G4 必要** |
| 工具调用展示 | 可观测性 | 有 | tool_execution_* / normalize | full tool detail | **G4 必要** |
| thinking 展示 | 推理可见 | 有（可关） | thinking 块 | thinking 能力 | **G4 必要**（capabilities 驱动） |
| 输入区 + 发送 | 主交互 | 有 | prompt/steer | 会话可写 | **G4 必要** |
| 中断 abort | 停长任务 | 有 | abort | abort 后可续 | **G4 必要** |
| 模型选择 | 换模型 | 有 | set_model / models.json | 会话级切换 | **G4 必要** |
| thinking level | 调推理 | 有 | set_thinking_level | 模型支持 | **G5 增强** |
| 上下文用量/cost | 状态透明 | 有 | session stats/usage | Pi usage 字段 | **G5 增强** |
| 文件树/预览 | coding 旁路 | coding 有 / general 无 | cwd 文件 API | **必有 workspace** | coding profile：**G5**；general：**不需要** |
| skills 商店 | 装技能 | 非核心 | Pi skills API | 单 Pi 生态 | **不需要**（后置） |
| models.json 编辑/测模型 | 本地改配置 | 运维向 | Pi 配置文件 | 本机 agent dir | **不需要**（密钥走外部网关） |
| OAuth/API key UI | 登录供应商 | 否（外部出口） | auth routes | 本机存密钥 | **不需要** |
| git worktree 切换 | 多工作树 | Desktop/coding 可能 | git + cwd | coding | **G5 增强 / Desktop** |
| fork / edit-from-here | 分支探索 | 有价值 | session fork API | Pi session 树 | **G5 增强** |
| slash commands UI | 扩展命令 | 部分 | available_commands | 单 runtime 命令表 | **G5** |
| 插件配置面板 | Pi 插件 | 否 | plugins API | 单 Pi | **不需要** |
| 音频完成提示 | 体验 | 可选 | — | — | **不需要** |
| 多 runtime 切换 | — | **我们特有** | — | 上游无 | **G4 必要（自建）** |
| capabilities 降级 | — | **我们特有** | — | 上游无 | **G4 必要（自建）** |

## 4. Pi 耦合点清单（→ G1 协议补齐）

| 耦合 | 位置（上游） | gateway 需提供的标准替代 |
|---|---|---|
| 直接 `createAgentSession*` / AgentSession | `lib/rpc-manager.ts` | Agent Protocol runs + 抽象 `AgentRuntime` |
| 读本地 `~/.pi/agent/sessions/**.jsonl` | session-reader | `GET /threads` / messages（不依赖 Pi 路径） |
| `set_model` / thinking / tools 命令 | agent API routes | 标准 run 输入 + thread 配置 API |
| toolCall 字段 normalize | `lib/normalize.ts` | RuntimeEvent → AG-UI 统一映射 |
| extension UI request/widget | rpc-manager / hooks | 可选 HITL；MVP 可忽略 |
| models.json / auth 读写 | models-config/auth routes | **不照搬**；接外部模型目录 |
| cwd 文件浏览 | files/cwd routes | 仅 coding profile 可选；走受控 FS API |
| 无鉴权 loopback | 架构假设 | 生产必须外置网关 |

## 5. 单 runtime / 单 profile 假设清单（→ G4 必须能力驱动）

| 假设 | 证据 | 我们的处理 |
|---|---|---|
| 只有 Pi | 依赖 `@earendil-works/*` only | runtime 选择器 + adapter id |
| 一定有 cwd/项目 | `cwd is required`（`agent/new`） | general 允许无 workspace |
| 会话=本地 jsonl 文件 | sessions 路径约定 | thread 元数据 + 可选导出 |
| 可本地改 API key | auth/* | 删除该旅程 |
| 文件预览总是有意义 | FileExplorer 一等公民 | `workspaceBinding` 为 false 时隐藏 |
| 工具预设= coding 工具名集合 | `CODING_TOOL_NAMES` | 按 capabilities/toolPolicy |
| 单机信任用户 | 无 auth | 多租户 header 模式 |

## 6. 值得借鉴

- 消息/工具/thinking 分区渲染清晰  
- 流式 SSE + 输入区状态机（streaming/queued）  
- 会话侧栏信息密度  
- 明确「不要暴露公网」的安全文案  

## 7. 明确不采纳

- Next 全家桶直接搬（本仓 web 已有栈则对齐本仓）  
- 本机 skills 市场与插件面板  
- 本机密钥/OAuth 配置 UI  
- 把 AgentSession 嵌进 Next Route Handler 的进程模型（我们改为 gateway 多 adapter 编排）  
- 无能力协商的硬编码 coding UI  

## 8. 给 G4 的范围一句话

**G4 = AP/AG-UI 驱动的多 runtime 聊天壳：会话列表、消息/工具/thinking、输入、abort、模型选择、capabilities 降级；文件树与 fork 等放 G5；密钥与 skills 商店不做。**
