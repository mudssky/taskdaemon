# 接入方式与延迟实测

> 模型：`xh/grok-4.5` · thinking=`off` · `--no-tools` · ephemeral session  
> 主机：darwin arm64 · Node v24.14.1 · Apple M4  
> 原始数据：`samples/{pi,omp}/latency-summary.json` · `samples/protocol-anomalies-summary.json`  
> 脚本：`scripts/measure-latency.mjs` · `scripts/protocol-anomalies.mjs` · `scripts/rpc-driver.mjs`

## 1. 可用接入方式

| Runtime | 方式 | 版本 | 如何启动 | 可用性 |
|---|---|---|---|---|
| Pi | CLI RPC | 0.82.0 | `pi --mode rpc --model xh/grok-4.5 ...` | **可用**（主测路径） |
| Pi | CLI print | 0.82.0 | `pi -p --no-session ...` | 可用（单次非流式） |
| Pi | SDK in-process | 0.82.0 | `createAgentSession` from `@earendil-works/pi-coding-agent` | **可用**；本仓未装依赖时需绝对路径 import global package |
| OMP | CLI RPC | 17.1.3 | `omp --mode rpc --model xh/grok-4.5 --cwd <dir> ...` | **可用** |
| OMP | CLI print | 17.1.3 | `omp -p ...` | 可用 |
| OMP | SDK in-process | 17.1.3 | 包导出 TS 源 + 类型；`RpcClient` 稳定 | **进程内 SDK 可行但工程摩擦更大**（exports 指向 `src/*.ts`）；G3 建议先 CLI RPC |

### 测量点定义

- **coldStartMs**：`spawn` → 首次 `get_state` 成功（可接受 prompt）
- **firstByteMs**：`prompt` 写入后 → 第一个 agent 相关事件
- **turnMs**：`prompt` → `isStreaming===false`

## 2. CLI 冷启动分布（每 runtime n=10）

单位 ms。

| Runtime | 指标 | min | median | p95 | max | mean |
|---|---|---:|---:|---:|---:|---:|
| Pi | coldStartMs | 2331 | **3110** | 5729 | 5729 | 3489 |
| Pi | firstByteMs | 122 | **158** | 271 | 271 | 163 |
| Pi | turnMs | 2981 | **3580** | 19278 | 19278 | 7188 |
| OMP | coldStartMs | 3207 | **3786** | 6258 | 6258 | 4224 |
| OMP | firstByteMs | 12 | **15** | 223 | 223 | 63 |
| OMP | turnMs | 2590 | **2792** | 27799 | 27799 | 5363 |

### 预热进程内多轮（warm，同进程 n=5）

| Runtime | 指标 | min | median | p95 |
|---|---|---:|---:|---:|
| Pi | firstByteMs | 118 | **143** | 157 |
| Pi | turnMs | 2045 | **2544** | 3596 |
| OMP | firstByteMs | 3 | **5** | 12 |
| OMP | turnMs | 1808 | **2184** | 2757 |

说明：OMP 首字节极低，是因为启动后事件面更「吵」，探测逻辑更容易先看到非 response 事件；**不能**解读为模型 TTFT 更快。turnMs 更接近用户可感知完成时间。

## 3. SDK in-process（Pi 抽测）

| 步骤 | 观测 |
|---|---|
| import 包 | ~607ms（冷 import） |
| createAgentSession #1 | ~2482ms |
| createAgentSession #2–5 | ~260–650ms |

→ SDK 路径 `coldStartCost` 可标 **low~medium**（相对 CLI 的 high），但隔离性弱于进程边界。

## 4. 资源占用（RPC 起进程后 ~2.5s，get_state 后采样）

| Runtime | RSS |
|---|---|
| Pi | ~151 MB |
| OMP | ~493 MB |

并发含义：10 个 OMP 会话仅内存就可能 ~5GB 量级，池化/上限必须按 adapter 区分。

## 5. 协议异常（CLI RPC）

| 用例 | Pi | OMP |
|---|---|---|
| 半包 | 补齐前 0 response，补齐后成功 | 同 |
| 粘包 | 两帧都成功 | 同 |
| 超长行 ~2MB prompt | 进程未崩，prompt response success | 同 |
| 非法 JSON 行 | 进程不退出，后续 get_state 仍可用 | 同 |
| 中途 SIGKILL | 子进程被杀；prompt 的 accept 可能已返回 success | 同（accept≠完成） |

**工程含义**：gateway 的分帧器必须自写（禁用 Node readline）；必须用 `agent_end`/`isStreaming` 判定完成，不能只看 prompt response；进程退出要映射为 run 失败并回收会话。

## 6. coldStartCost 判定

| Runtime | attachMode | coldStartCost | 依据 |
|---|---|---|---|
| Pi CLI | cli-spawn | **high** | median coldStart ~3.1s，p95 ~5.7s |
| Pi SDK | sdk-inprocess | **low** | 创建稳态 <1s（import 已热） |
| OMP CLI | cli-spawn | **high** | median ~3.8s，p95 ~6.3s，RSS 更大 |

## 7. 池化策略建议（给 G2）

1. **CLI adapter 必须预热池**：建议每个 enabled runtime 保有 `minIdle=1~2` 的热进程；空闲保留 **5–15min**（冷启动 >3s，按需起对交互不友好）。
2. **按 runtime 分池**：OMP 内存约 Pi 的 3×，并发上限应更低（例如 Pi max 8、OMP max 3，具体按机器内存重算）。
3. **SDK 路径**（若 G3 做 Pi SDK adapter）：可更激进按需创建，但仍要限制同进程会话数与崩溃半径。
4. **完成判定**：`waitForIdle = !isStreaming && 见 agent_end/agent_settled`；abort 后允许同一 session 继续。
5. **健康检查**：周期性 `get_state`；非法输入后多数仍存活，但 OMP 坏模型可能直接 exit——要自动换进程。

## 8. G3 接入建议

- **MVP**：两 runtime 都走 CLI RPC + 共享 JSONL 客户端（本目录 `rpc-driver.mjs` 可作起点）。
- **可选加速**：Pi 增加 sdk-inprocess adapter（同一 `AgentRuntime` 接口，不同 `attachMode`）。
- **fixture**：`samples/pi|omp/*.jsonl` 可直接做事件映射测试。
