# T6 Run 完整日志归档 — 执行计划

## 前置

- [ ] 读 `.trellis/spec/backend/index.md` 开发前检查第 1、3、5、7、8 项
- [ ] 读 `.trellis/spec/backend/scheduler-runner-guidelines.md`（run 生命周期与执行历史）
- [ ] 读 `.trellis/spec/backend/logging-guidelines.md`（区分进程日志与 run 日志）
- [ ] 读 `internal/audio/service.go` 的 `pruneHistory`、`absolutePath`（本任务直接复用其模式）
- [ ] **确认 T2a 的 Ent schema 改动状态**（路线图 §8.3：T2a 优先）
- [ ] 从 `dev` 切分支 `feat/t6-run-log-archive`

## 阶段 1 · 落盘核心

- [ ] 创建 `internal/runlog/` 包
- [ ] `store.go`：路径规则（`<dataDir>/runlogs/<yyyy-mm>/<runID>.log`）、归档根解析
- [ ] `writer.go`：带缓冲的写入器，stdout/stderr 行级前缀标记
- [ ] 进程退出时刷盘
- [ ] **写失败不影响 run**：捕获 → 记进程日志 → 返回标记
- [ ] 目录 `0700`、文件 `0600`
- [ ] 单测：
  - [ ] 路径可从 runID 正确推导
  - [ ] stdout/stderr 来源可区分且时序正确
  - [ ] 写失败时返回标记而非 error 中断
  - [ ] 文件权限位正确

**验证**：`cd services/taskdaemon-go && go test ./internal/runlog/...`

## 阶段 2 · 接入 runner

- [ ] 在 `internal/runner/` 的执行路径接入 writer
- [ ] 写入不阻塞命令执行（用慢磁盘或注入延迟验证）
- [ ] run 结束时关闭 writer 并刷盘
- [ ] 单测：写日志失败时 run 仍正常完成并返回正确状态

**验证**：`go test ./internal/runner/... ./internal/runlog/...`

## 阶段 3 · Ent schema 与三态

> ⚠️ 本阶段有 Ent 生成物冲突风险。**先确认 T2a 已合并**。

- [ ] 在 `internal/data/ent/schema/run.go` **尾部追加**字段：归档状态枚举、日志大小
- [ ] 状态枚举三态：`archived` / `absent` / `pruned`
- [ ] 旧记录默认 `absent`
- [ ] `pnpm generate:go`
- [ ] run 结束时写入归档状态与大小
- [ ] 单测：
  - [ ] 三态可正确区分
  - [ ] 旧记录（无归档）为 `absent`
  - [ ] 写失败的 run 状态正确

**验证**：`go test ./internal/runlog/... ./internal/data/...`

## 阶段 4 · 保留策略

- [ ] `prune.go`：天数 + 总体积双维度，参考 `audio/service.go:408`
- [ ] 清理顺序：先按天数，再按体积从旧到新
- [ ] **跳过正在写入的 run 日志**（检查 run 状态）
- [ ] 清理后更新 DB 状态为 `pruned`，不删记录
- [ ] 清理有日志记录（删几个、释放多少）
- [ ] run 结束后异步触发，不在写入路径上
- [ ] 单测：
  - [ ] 天数淘汰正确，配置 0 不限
  - [ ] 体积淘汰正确，配置 0 不限
  - [ ] 两维度同时生效
  - [ ] 运行中的 run 日志不被清理
  - [ ] 清理后状态为 `pruned`

**验证**：`go test ./internal/runlog/...`

## 阶段 5 · 配置

- [ ] `internal/config/types.go` **尾部追加** `RunLogConfig`（append-only，`// T6` 标注）
- [ ] `defaults.go` 追加：`Enabled=true`、`RetainDays=30`、`MaxTotalBytes=2GiB`
- [ ] `env.go` 追加 `TASKDAEMON_RUNLOG_*` 映射
- [ ] 单测：默认值、env 覆盖

## 阶段 6 · 查询与下载 API

- [ ] `runlog_dto.go` + `runlog_routes.go`：design §6 的四个端点
- [ ] **流式下载**：`io.Copy`，不整体读入内存
- [ ] **尾部读取**：从文件末尾反向扫描，不全量读
- [ ] 范围读取：offset + length
- [ ] `RUNLOG_*` 错误码
- [ ] 走管理员认证
- [ ] `router.go` **追加**注册行（append-only）
- [ ] route 测试（参考 `audio_routes_test.go`）：
  - [ ] 元信息查询三态各一次
  - [ ] 完整下载成功
  - [ ] 尾部 N 行正确
  - [ ] 范围读取正确
  - [ ] 非法范围返回 `RUNLOG_INVALID_RANGE`
  - [ ] 无归档返回 `RUNLOG_ARCHIVE_NOT_FOUND`
  - [ ] run 不存在返回 `RUNLOG_RUN_NOT_FOUND`
  - [ ] 未认证返回 401
  - [ ] 响应含 traceId

**验证**：`go test ./internal/httpapi/...`

## 阶段 7 · 安全审查

- [ ] **路径遍历测试**：构造恶意 runID 参数（`../`、绝对路径、URL 编码变体），断言全部被拒
- [ ] 核对：路径由服务端从 runID 推导，**代码中无请求参数拼路径**
- [ ] 绝对路径前缀检查生效（参考 `audio/service.go:560`）
- [ ] 归档目录与文件权限正确
- [ ] 日志中不含 taskdaemon 自身凭据

## 阶段 8 · 大文件验证

- [ ] 生成一个大日志文件（≥ 500MB）
- [ ] 下载：内存占用平稳，无整体读入（观察 RSS）
- [ ] 尾部 100 行：响应快速，不扫全文件
- [ ] 范围读取：正确且高效

## 阶段 9 · 前端最小接入

> 刻意压到最小，避免与 W 轨争抢 runs feature 目录。

- [ ] runs 详情处加「下载完整日志」按钮
- [ ] 三态提示：可下载 / 无归档（旧数据）/ 已清理
- [ ] loading / error 状态
- [ ] **不做**日志查看器
- [ ] 单测：三态渲染分支

**验证**：`pnpm --filter @taskdaemon/web test`

## 阶段 10 · 收尾

- [ ] 回写路线图 §7.2 状态为 done
- [ ] 若新增配置项，形状符合 C-1 契约（若 T1a 已冻结）

## 全量验收

```bash
pnpm typecheck
pnpm lint
pnpm test:go
pnpm vet:go
pnpm --filter @taskdaemon/web test
cd services/taskdaemon-go && go test ./...
```

- [ ] 上述命令全绿
- [ ] `prd.md` 全部 Acceptance Criteria 勾选

## 评审门

| 门 | 时机 | 检查 |
|---|---|---|
| G-1 | 阶段 2 结束 | 写日志是否真的不阻塞、不影响 run 成败 |
| G-2 | 阶段 3 前 | **T2a 是否已合并**，避免 Ent 生成物冲突 |
| G-3 | 阶段 7 结束 | 路径遍历防护是否覆盖典型载荷 |
| G-4 | 阶段 8 结束 | 大文件路径的内存与响应表现是否可接受 |

## 回滚点

| 阶段 | 回滚方式 |
|---|---|
| 1–2 | 纯新增包 + runner 接入点，删除即可 |
| 3 | revert；新增字段有默认值，旧记录不受影响 |
| 4–6 | revert；配置与路由均为 append |
| 9 | 删除前端按钮 |
| 运行时 | `runlog.enabled = false` 停用 |
| 用户侧 | 已产生的日志文件需手动清理，文档给出路径 |

## 与其他任务的协调

- **T2a**：⚠️ **Ent 生成物冲突，T2a 优先**。本任务阶段 3 开始前确认 T2a 已合并；合并他人改动后**重跑 `pnpm generate:go`，不手工合并生成物**
- **T1a（C-1）**：若日志配置要进设置页，需符合 C-1 分级；不进则无依赖
- **W 轨任务**：本任务的前端改动刻意最小化，若与 W 轨在 runs feature 有交集，以 W 轨为主
