# G0 HANDOFF — 结论摘要

## 阶段 0 门

| 项 | 结果 |
|---|---|
| OMP 可程序驱动？ | **YES**（`omp --mode rpc` + JSONL） |
| 是否触发「重估双 adapter」？ | **否** |
| 路线图修订？ | **是**（§5.1 版本锚点、§7.5 G0=done、§16 记录；假设确认而非推翻双 adapter） |

## 核心结论

1. **Pi 与 OMP 都能 CLI RPC 程序驱动**；事件骨架同构，OMP 为超集且更重。  
2. **`AgentRuntime` 草案可用**：补 `followUp` / `getMessages` / `setModel` / `disposeSession` + **secret sanitize 不变量**。  
3. **coldStartCost=high**（CLI）：Pi med~3.1s，OMP med~3.8s → **必须预热池**。  
4. **会话持久化在 runtime 文件** → gateway 只需薄索引，不必全文双写。  
5. **Gateway = 编排+协议+策略+薄索引**，不重做 agent loop。  
6. **Pi WebUI** 参考 `agegr/pi-web`（MIT）；G4 只取聊天核心并去 Pi 单家假设。  
7. **工具链**：Hono on Node（Bun 亦可）；pnpm workspace + Biome + vitest。  

## 模型

全测默认 **`xh/grok-4.5`**。

## 交付索引

| 文件 | 用途 |
|---|---|
| `pi-capability-matrix.md` | R1 Pi |
| `omp-capability-matrix.md` | R1+R2 OMP |
| `runtime-abstraction-feasibility.md` | R3 ★ G1 输入 |
| `runtime-attach-and-latency.md` | R4 + 池化 |
| `gateway-scope-decision.md` | R5 G2 范围 |
| `runtime-toolchain.md` | R6 |
| `persistence-decision.md` | R7 |
| `deployment-shape.md` | R8 |
| `pi-webui-teardown.md` | R9 |
| `samples/**` | G3 fixture |
| `scripts/**` | G3 起点 |

## 验收自检

- 能力矩阵无「推测/文档上说」作为结论依据（均绑实验/样本路径）  
- 延迟为分布（n=10 cold）  
- 协议异常五类已测  
- 样本含 pi/omp  
- 业务代码未改（仅 research/ + roadmap）  

## 残留

- Pi WebUI 未做本机长程手工点击（源码+文档旅程）；若 G4 需要像素级交互细节可再补一轮。  
- OMP `get_entries` 超时未深挖。  
- 模型切换未测到「换到另一模型 id」的完整路径（available 列表未命中其它 xh 目标时回退同模型命令验证）。  
- 样本中曾出现 OMP state 密钥，已批量 redact；**adapter 必须默认剥离**。
