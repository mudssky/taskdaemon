# OMP 能力矩阵（实测）+ 基础核实

> 模型：`xh/grok-4.5`  
> 版本：`omp/17.1.3`（包 `@oh-my-pi/pi-coding-agent`）  
> 快照日期：2026-07-27  
> 脚本：`research/scripts/capability-probe.mjs`、`rpc-driver.mjs`  
> 样本：`research/samples/omp/*.jsonl`

## R2 基础核实

| 项 | 实测 |
|---|---|
| 全称 | **Oh My Pi（OMP）** |
| npm 包 | `@oh-my-pi/pi-coding-agent`（bin: `omp`） |
| 仓库 | https://github.com/can1357/oh-my-pi |
| 官网 | https://omp.sh |
| 许可证 | **MIT** |
| 维护活跃度 | 2026-07-26 push；v17.1.3；~19.8k stars；未 archived |
| 与 Pi 关系 | Pi 系 fork/增强 monorepo；CLI 与 RPC 形状高度同源，能力面更宽（task/hub/browser/LSP/MCP/记忆等） |
| 非交互/RPC | **有且可程序驱动**（阶段 0 门 **PASS**） |
| 模式 | `--mode text|json|rpc|rpc-ui`；`-p/--print` |
| SDK | **有**：包导出大量 TS 模块 + `RpcClient`（`modes/rpc`）；主入口偏向 TS 源（`main: ./src/index.ts`），进程内嵌入更自然走 Bun/TS；程序驱动首选 **CLI RPC + RpcClient** |
| 配置根 | 默认 `~/.omp/agent`（`PI_CODING_AGENT_DIR` 可覆盖）；支持 `--profile` 隔离 |

### 阶段 0 门

- 命令：`omp --mode rpc ...` + JSONL `get_state` / `prompt`
- 结果：**可被程序驱动**，与 Pi 同构 RPC 客户端可复用（本任务 `rpc-driver.mjs` 双 runtime 共用）
- 决策影响：**无需重估「初版两 adapter」**；两 adapter 方案成立

## 能力表

| 能力 | 结论 | 验证方式 | 观察 |
|---|---|---|---|
| agent loop | **提供** | RPC prompt | 同 Pi 风格：`agent_start`/`turn_*`/`message_*`/`agent_end`；另有 `ready`、`available_commands_update` |
| MCP 工具接入 | **提供（更重）** | get_state / 工具列表 | 即使 `--no-tools`，本机配置下仍可能挂载 MCP（如 openviking_*）；会话隔离依赖 profile/配置，不是默认干净沙箱 |
| 会话持久化 | **提供（runtime）** | 杀进程恢复 | session-dir 下 jsonl；恢复后能答出 `BLUEBERRY`；`messageCount` 更高（含 custom/系统注入消息） |
| 多会话并发 | **一会话一进程（CLI）** | 双进程 | 独立 sessionId；内存显著高于 Pi |
| 模型切换 | **提供（会话级）** | `set_model` | 命令可用；历史保留。**警告**：`get_state().model` 可能带出 `headers.Authorization` 明文，gateway 必须剥离 |
| steering | **提供** | 类型/命令 | 与 Pi 同族：`steer`/`follow_up`/mode 设置 |
| abort | **提供（真停可继续）** | 长文 + abort | streaming true→false；后续回合成功 |
| 消息历史 | **提供** | `get_messages` | 返回 messages 数组；`get_entries` 在本次 60s 超时（OMP 更重/实现差异） |
| 工具调用可见性 | **full** | bash 工具 | `tool_execution_start/update/end`；相关事件 162 条 |
| thinking | **提供** | thinking high | 大量 thinking 命中；消息 content 含 thinking 块 |
| 非 coding 可用性 | **部分提供** | 空 cwd 重试 | 去掉错误 flag 后可在空目录对话成功；但仍注入全局 `~/.omp/agent` 的 AGENTS/扩展/工具。**不是**「零文件系统、零 coding 人格」的干净 general profile，需要 gateway 用 profile/`--system-prompt`/`工具裁剪` 强制收敛 |
| 错误与重试 | **提供（更硬）** | 坏模型 | 坏模型名可导致 **进程 exit 1** 直接起不来（与 Pi「进程仍在、事件报错」不同） |
| 资源 | 重 | RSS 采样 | 单 RPC 空闲约 **~493MB RSS**（Pi ~151MB） |

## RuntimeCapabilities（OMP 建议填值）

```ts
{
  profile: ['coding', 'general'], // general 需额外裁剪，非开箱即干净
  attachMode: 'cli-spawn',
  coldStartCost: 'high',          // median ~3.8s
  steering: true,
  thinking: true,
  toolCallDetail: 'full',
  modelSwitch: 'session',
  workspaceBinding: true,
  sessionPersistence: 'runtime'
}
```

## 与 Pi 的事件流差异（观察）

| 点 | OMP |
|---|---|
| 额外事件 | `ready`、`available_commands_update` 更常见 |
| 启动载荷 | 显著更大（样本 jsonl 体积常为 Pi 的数倍到数十倍） |
| 安全 | state 中可能泄露 provider header/密钥 |
| 工具默认面 | 更宽；`--no-tools` ≠ 无 MCP |
| 进程失败模式 | 配置/模型错误更易直接 exit |

## 阶段 0 结论（给协调者）

**OMP 可程序驱动。** 两 adapter 初版方案不因 R2 否决。差异主要在重量、默认工具面、错误退出语义与密钥泄漏面，可用 capabilities + adapter 差异化处理，不构成「无法抽象」。
