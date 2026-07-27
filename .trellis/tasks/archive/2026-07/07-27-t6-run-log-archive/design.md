# T6 Run 完整日志归档 — 技术设计

## 1. 核心区分：进程日志 ≠ run 日志

| | 进程日志 | run 日志（本任务） |
|---|---|---|
| 内容 | taskdaemon 自身的 slog 输出 | 被调度命令的 stdout/stderr |
| 现有实现 | `internal/logging/file_handler.go`（lumberjack） | 无，仅 DB 截断字段 |
| 生命周期 | 跟随进程 | 跟随单次 run |
| 保留策略 | 按文件大小轮转 | 按天数 + 总体积 |

**不要复用 lumberjack 的轮转逻辑管 run 日志** —— 它按单文件大小轮转，而 run 日志是「每次 run 一个文件、按集合淘汰」，语义完全不同。

新建独立包 `internal/runlog/`。

## 2. 分层

```text
internal/runner/                    ← 执行命令，产生 stdout/stderr 流
        │ 写入
        ▼
internal/runlog/writer.go           ← 落盘（不阻塞执行）
        │
internal/runlog/store.go            ← 路径规则、元信息、范围读取
internal/runlog/prune.go            ← 保留策略
        │
internal/httpapi/runlog_routes.go   ← 查询/下载 API
```

## 3. 落盘

### 3.1 路径规则

```
<userDataDir>/taskdaemon/runlogs/<yyyy-mm>/<runID>.log
```

按年月分目录，避免单目录文件过多。路径**可从 runID 推导**，不需要额外索引表。

### 3.2 stdout / stderr 区分

**决策：单文件 + 行级前缀标记**，而非两个文件。

理由：两个文件会丢失时序关系（用户看不出 stderr 是在哪行 stdout 之后出现的），而排障时时序恰恰是关键。单文件带标记既保留时序又可区分来源。

格式在实现时定，要求：可被简单工具过滤（如 grep），不引入解析成本。

### 3.3 不阻塞执行

- 写入走带缓冲的 writer，runner 侧写入不等待磁盘
- **写日志失败绝不导致 run 失败**：捕获错误 → 记进程日志 → 在 run 记录上打标记 → 继续执行
- 进程退出时确保缓冲刷盘

## 4. 与数据库的关系

| 存哪 | 存什么 |
|---|---|
| DB（现有字段） | **截断摘要**（如首尾各 N 行），供列表页快速预览 |
| 文件 | 完整输出 |
| DB（新增字段） | 归档状态：`archived` / `absent` / `pruned` |

**三态区分是必须的**（R2）：

- `archived` — 有文件可下载
- `absent` — 旧数据，本功能上线前的 run
- `pruned` — 曾有，已被保留策略清理

前端对三态的提示完全不同，合并成「有/无」会让用户困惑（「我昨天还能下载的日志去哪了？」）。

## 5. 保留策略

两个维度**同时生效**，任一触发即淘汰：

| 维度 | 默认 | 0 的含义 |
|---|---|---|
| 保留天数 | 30 | 不限 |
| 总体积上限 | 2 GiB | 不限 |

### 清理顺序

1. 先按天数删过期的
2. 若仍超体积，按时间从旧到新删到达标为止

### 边界条件

- **不清理正在写入的 run 日志**：清理前检查 run 状态，运行中的跳过
- 清理时更新 DB 状态为 `pruned`，不是直接删记录
- 清理有日志记录（删了几个、释放多少空间）

### 触发时机

参考 `internal/audio/service.go:408 pruneHistory` 的模式：run 结束后异步触发，不在写入路径上。

## 6. API

新文件 `internal/httpapi/runlog_routes.go` + `runlog_dto.go`，路由注册追加到 `router.go`（append-only，`// T6` 标注）。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/runs/{runID}/log/meta` | 元信息：状态、大小、创建时间 |
| GET | `/api/runs/{runID}/log` | 下载完整日志（流式） |
| GET | `/api/runs/{runID}/log/tail?lines=N` | 尾部 N 行 |
| GET | `/api/runs/{runID}/log/range?offset=&length=` | 字节范围 |

### 流式下载

用 `io.Copy` 直接从文件流到 response，**不整体读入内存**。设置 `Content-Length` 与 `Content-Disposition`。

### 尾部读取

从文件末尾反向扫描找到第 N 个换行，只读这部分。避免为了拿 100 行而读 500MB 文件。

### 错误码（`RUNLOG_*`）

| 码 | 场景 |
|---|---|
| `RUNLOG_ARCHIVE_NOT_FOUND` | run 无归档（absent/pruned） |
| `RUNLOG_RUN_NOT_FOUND` | run 本身不存在 |
| `RUNLOG_INVALID_RANGE` | 范围参数非法 |
| `RUNLOG_READ_FAILED` | 读取失败 |

## 7. 安全（重点）

### 路径遍历防护

**runID 是整数**，从 DB 查出 run 记录后由服务端推导路径，**绝不用请求参数拼路径**。这从根上消除了遍历风险。

即便如此，仍加一层防御：拼出的绝对路径必须在归档根目录内（`filepath.Clean` + 前缀检查），参考 `internal/audio/service.go:560 absolutePath` 的既有做法。

### 其他

- 归档目录权限 `0700`，文件 `0600`
- 日志内容脱敏边界与 `internal/httpapi/redaction.go` 一致 —— 注意：**run 输出是用户自己的命令产生的**，taskdaemon 不应过度改写，但要确保不把 taskdaemon 自己的凭据混进去
- 下载 API 走管理员认证

## 8. 前端最小接入

**刻意压到最小**，避免与 W 轨任务争抢 runs feature 目录：

- runs 详情处加「下载完整日志」按钮
- 三态提示：可下载 / 无归档（旧数据）/ 已清理
- 遵守 `interaction-guidelines.md` 的 loading/error 约定
- **不做**日志查看器（搜索、高亮、跟随）

## 9. Ent schema

若需要新增字段（归档状态、文件大小），追加到现有 `Run` schema 还是新建 entity？

**决策：追加字段到 `internal/data/ent/schema/run.go`**，不新建 entity。理由：一对一关系，新建表徒增 join。

⚠️ 这意味着**要改现有 schema 文件**，与「一任务一文件」的约定有冲突。处置：

- 追加字段到文件尾部，不动现有字段
- 与 T2a 的 Ent 冲突约定仍适用：**T2a 优先**，本任务在 T2a 合并后再改 schema 并重跑 `pnpm generate:go`

## 10. 兼容性与回滚

- 新增字段有默认值，旧 run 记录状态为 `absent` → 无破坏性变更
- 回滚：配置 `runlog.enabled = false` 停用；代码 revert 后新增字段留空不影响其他功能
- 已产生的日志文件需手动清理，文档中给出路径

## 11. 风险

| 风险 | 处置 |
|---|---|
| **大文件读入内存** | §6 的流式 + 尾部反向扫描 |
| 路径遍历 | §7：runID 是整数，服务端推导路径，绝不拼请求参数 |
| 写日志阻塞 run | §3.3 的缓冲写 + 失败不影响 run |
| 清理正在写入的文件 | §5 的运行中跳过检查 |
| 与 T2a 争抢 Ent 生成物 | **T2a 优先**，本任务后合并时重跑 `pnpm generate:go` |
| 磁盘写满 | 总体积上限默认 2 GiB；写失败不影响 run 执行 |
