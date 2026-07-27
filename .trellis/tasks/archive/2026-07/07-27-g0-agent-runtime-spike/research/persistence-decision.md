# 持久化选型结论

## 结论（合法且省事）

**Gateway 需要「薄」持久化：thread/run 元数据 + 路由索引。不需要把 runtime transcript 全量双写进 PostgreSQL。**

「完全不需要持久化」**不成立**：因为要支撑刷新/断线/进程崩溃后的 thread 列表与 run 状态；但「独立大消息库」本阶段不需要。

## 依据

1. Pi/OMP **session jsonl 已能跨进程恢复**（`BLUEBERRY` 实测）。  
2. 两 runtime 持久化形态同构 → 可统一「索引指向文件」。  
3. 完整双写会制造一致性问题（谁为权威），ROI 低。

## 方案

| 层 | 选型 |
|---|---|
| 元数据 DB | **SQLite**（开发/Desktop 默认）+ 可选 PostgreSQL（与 taskdaemon 同实例或独立 library） |
| 查询层 | TS：`drizzle` 或 `kysely`（二选一；避免引入 Ent/Go 到 gateway） |
| migration | gateway 自管 SQL migration；**不**进 Go Ent 生成物 |
| transcript | runtime session file path 存在元数据行 |
| 租户 | 表含 `tenant_id`；索引 `(tenant_id, thread_id)`；对接 `X-Tenant-Id` |

## 与 Go/Ent 边界

- **不**复用 taskdaemon Ent schema  
- 跨引擎关联用 ID（task id / thread id）而非共享表  
- 备份：元数据 DB + runtime session 目录一并纳入部署备份

## 何时升级到全文双写

- 需要跨机器无共享盘恢复  
- 需要统一检索/审计查询全文  
- runtime 文件格式频繁破坏性变更  

当前 G2：**不做**。
