# HANDOFF · T6 Run 完整日志归档

> 分支：`mudssky/t6-run-log-archive`（基于含 T2a/T5/G0 的 `dev`）  
> 状态：**实现完成，待协调者验收；不要 merge**  
> Worker：supervised task `task_1b8ea91db1dc`

## 交付摘要

Run 完整 stdout/stderr 落盘归档 + 保留策略 + 查询/下载 API + runs 页最小下载入口。

### 核心包 `services/taskdaemon-go/internal/runlog/`

| 文件 | 职责 |
|---|---|
| `store.go` | 路径 `<userConfigDir>/taskdaemon/runlogs/<yyyy-mm>/<runID>.log`、尾部/范围读取、路径逃逸防护 |
| `writer.go` | 单文件行级前缀 `[stdout]`/`[stderr]`、缓冲写、失败永不向上传播 |
| `service.go` | Begin/Finish、Meta、流式下载、DB 三态回写 |
| `prune.go` | 天数 + 总体积双维度；跳过正在写入 / running；异步触发 |
| `errors.go` | `ErrRunNotFound` / `ErrArchiveNotFound` / `ErrInvalidRange` / `ErrReadFailed` |

### 接入点

- **runner**：`Config.StdoutMirror` / `StderrMirror` + `attachMirror`（MultiWriter）
- **scheduler**：run 开始 `Begin`，结束 `Finish` + `ApplyArchiveResult` + `SchedulePrune`
- **app/serve**：装配 `runlog.New` 注入 scheduler 与 router
- **Ent**：`schema/run.go` 追加 `log_archive_status` / `log_size_bytes` / `log_write_failed`（默认 `absent`）
- **config（append-only // T6）**：`RunLogConfig{Enabled,RetainDays,MaxTotalBytes}` 默认 30 天 / 2GiB
- **router（append-only // T6）**：`/api/runs/:runID/log{,/meta,/tail,/range}`
- **file_handler.go**：仅注释澄清进程日志 ≠ run 日志（不复用 lumberjack）
- **前端**：`RunHistoryTable` 三态下载入口（archived / absent / pruned）

### API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/runs/{id}/log/meta` | 元信息（三态、大小、可用） |
| GET | `/api/runs/{id}/log` | 完整流式下载 |
| GET | `/api/runs/{id}/log/tail?lines=N` | 尾部 N 行 |
| GET | `/api/runs/{id}/log/range?offset=&length=` | 字节范围 |

错误码：`RUNLOG_RUN_NOT_FOUND` / `RUNLOG_ARCHIVE_NOT_FOUND` / `RUNLOG_INVALID_RANGE` / `RUNLOG_READ_FAILED`（均含 traceId）。

## 验收命令（本 worker 已跑绿）

```bash
pnpm typecheck
pnpm lint
pnpm test:go
pnpm vet:go
pnpm --filter @taskdaemon/web test
cd services/taskdaemon-go && go test ./internal/runlog/... ./internal/logging/...
```

## 风险 / 协调注意

1. **Ent 生成物**：改了 `schema/run.go` 并 `pnpm generate:go`；合入后若与他任务冲突，后合者重跑 generate，禁止手揉。
2. **共享 append-only**：`config/types|defaults|env|load`、`httpapi/router.go` 仅尾部追加 `// T6`。
3. **scheduler / runner / app / task_dto / 前端 runs**：design 要求接入，非独占但必要；合入时注意与 W 轨 runs feature 冲突面极小（仅下载列）。
4. **未做**：日志查看器 UI、实时 SSE、对象存储压缩、跨 run 检索（PRD Out of Scope）。

## 建议合入后冒烟

1. 触发一次 shell 任务，确认 `~/Library/Application Support/taskdaemon/runlogs/<yyyy-mm>/<id>.log` 存在（macOS）。
2. `GET /api/runs/{id}/log/meta` → `status=archived`。
3. 下载接口返回 plain text；无归档 run 返回 `RUNLOG_ARCHIVE_NOT_FOUND`。
4. 配置 `runlog.enabled=false` 后新 run 保持 `absent`。
