# Gateway 最小规模判定

> 依据：能力矩阵 + 抽象可行性 + 延迟  
> 对应路线图 §5.8 七项职责

## 七项职责判定表

| 职责 | Pi 覆盖 | OMP 覆盖 | gateway 需做 | 工作量 | 依据实验 |
|---|---|---|---|---|---|
| 1 会话↔进程编排 | 无（单会话进程） | 无 | **必须**：spawn/池化/并发上限/僵尸回收/会话路由 | **L** | 多会话=多进程；coldStart high；OMP RSS~3× |
| 2 非交互调用入口 | CLI/SDK 私有 | 同 | **必须**：HTTP Agent Protocol 子集 + 内部 API | **M** | runtime 无多租户 HTTP |
| 3 thread/run 持久化与恢复 | **runtime 会话文件** | **同** | **薄索引即可**：映射 threadId→runtime sessionFile/进程；**不必**重存完整 transcript | **S–M** | session_persist 双 runtime 恢复成功 |
| 4 对外协议转换 | 私有 JSONL 事件 | 同构超集 | **必须**：→ AG-UI / AP；事件过滤与降级 | **M** | 事件样本齐全 |
| 5 多 adapter 抽象与能力协商 | — | — | **必须**：AgentRuntime + capabilities 透出 | **M** | 抽象可行性：可行 |
| 6 策略执行 | 弱（本地 flag） | 更弱（默认面更宽） | **必须**：tool allowlist、cwd 沙箱、egress、profile 裁剪 | **M** | OMP --no-tools 仍可能有 MCP |
| 7 usage/audit 产出 | 消息 usage 字段有 | 有 | **必须**：归一化 usage 事件外送；不自建计费 | **S** | message 上有 usage |

## 关键问答

### gateway 是否需要自己的 thread/run 存储？

**需要元数据存储，不需要完整消息双写（默认）。**

| 数据 | 存放 |
|---|---|
| threadId、runtime、sessionFile、cwd、tenant、状态 | **gateway DB/SQLite** |
| 消息/工具/thinking 全文 | **runtime session jsonl**（权威） |
| run 状态机（queued/running/interrupted/failed） | **gateway**（进程崩溃时 runtime 不知 AP 语义） |

两 runtime 持久化都是 runtime 文件且可恢复 → 取 **「gateway 索引 + runtime 权威」**，而不是「两套全文」或「只信弱的一方」。若 runtime 文件不可迁移跨机，跨机 HA 再升级为双写（后置）。

### 会话↔进程编排复杂度

| coldStartCost | 策略 |
|---|---|
| high（Pi/OMP CLI） | 预热池 + 长空闲保留 + 严格 maxConcurrency |
| low（Pi SDK 可选） | 按需创建 + 短空闲 + 注意同进程故障域 |

OMP 额外：`memoryClass=heavy` → 更低并发。

### 多 adapter 抽象层工作量

**M**：接口冻结（G1）+ 两个 CLI adapter 共享 JSONL 传输层；差异集中在 SessionOpts 默认值、事件过滤、secret sanitize、进程池参数。

### 直接透传、不封装

- agent loop / LLM 调用 / 工具执行引擎  
- steer/abort 语义（只做命令转发与状态映射）  
- thinking 内容（只做事件映射）  
- runtime 会话文件内容（只做路径索引）

## G2 做什么 / 不做什么

| 做 | 不做 |
|---|---|
| 进程池与 session 路由 | 自研 agent loop |
| AP threads/runs/stream/cancel | 完整重做消息 DB |
| AG-UI 事件桥 | 密钥管理（走外部网关/Bifrost） |
| capabilities 协商 API | OMP/Pi 插件市场 |
| tool/cwd 策略 | Desktop 特化 UI |
| usage 事件外送 | 企业 SSO（外部） |
| 预热与健康检查 | 第三 runtime |

## 规模结论

Gateway 是 **「编排 + 协议 + 策略 + 薄索引」** 服务，不是第二 agent 运行时。  
在双 CLI adapter 前提下，G2 主体工作量 **M–L**，最大块是进程编排与协议转换；持久化可保持薄。
