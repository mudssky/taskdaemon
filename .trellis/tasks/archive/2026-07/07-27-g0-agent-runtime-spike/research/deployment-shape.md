# 部署形态

## 结论

| 环境 | agent-gateway | Pi | OMP |
|---|---|---|---|
| 开发 | **独立 Node 进程**，`pnpm dev:agent-gateway` | 本机全局 `pi` 或 devDependency | 本机全局 `omp`（可选） |
| 生产 | **独立进程/容器**，位于外部网关之后 | 镜像内安装或 sidecar | 同左；可按租户开关 |
| Desktop | 由 taskdaemon/桌面壳 **拉起** 子进程 | 捆绑或探测本机 | 可选依赖 |

## 形态选项代价

| 选项 | 代价 | 建议 |
|---|---|---|
| 完全嵌入 taskdaemon 进程 | 崩溃半径大；Go/TS 混部难 | **否** |
| taskdaemon 拉起并托管生命周期 | 需监督协议；适合 Desktop | Desktop **是** |
| 完全独立部署 | 运维多一个服务；边界清晰 | 服务器 **是** |

## 多 runtime 分发

| 问题 | 决策 |
|---|---|
| 是否必须同时安装 Pi + OMP？ | **否**。gateway 启动时探测 bin/SDK，capabilities 只宣告可用 adapter |
| 缺一个能否运行？ | **能**。单 adapter 降级；UI 隐藏不可用 runtime |
| 二进制来源 | 系统 PATH / 配置路径 / 容器层安装；不把 OMP 整个 monorepo 打进 gateway 包 |
| 版本钉扎 | 运维文档钉 `pi@0.82.x`、`omp@17.1.x`；G0 已记版本锚点 |

## 开发期

```bash
# 建议（G2 落地时）
pnpm dev:agent-gateway   # Hono :PORT
# 依赖本机：
which pi && which omp
export G0_MODEL=xh/grok-4.5   # 或配置文件
```

与现有 `pnpm dev:web` / `dev:backend` **并行**，不塞进 Go 进程。

## 生产拓扑（呼应路线图 §4.3）

```text
Internet → 外部网关(鉴权/配额) → agent-gateway → (spawn) pi|omp
                                ↘ usage/audit 事件
taskdaemon ← 可选 G6 调用 ← agent-gateway
```

- 密钥：模型 API key 仍走外部 LLM 出口（Bifrost 等），gateway 不存明文；并 **sanitize** runtime state。  
- 会话盘：持久卷挂载 session 目录或仅本地盘（单机）。

## Desktop

- gateway 随应用启动，监听 loopback  
- runtime 优先探测本机 Pi/OMP，避免塞进巨型 asar  
- 无外部网关时用 `dev-single-principal`
