# gocron 能力验证研究

## 来源

* Context7 library: `/go-co-op/gocron`
* 查询主题：5/6 字段 cron、timezone、动态新增/更新/删除任务、singleton/skip-overlap、scheduler-wide 并发限制、shutdown 语义。

## 结论摘要

gocron v2.21.2 的公开文档与本仓库 spike 测试显示，它覆盖 taskdaemon 第一版调度核心需要验证的关键能力。结论：第一版调度核心可以选择 gocron，暂不需要回退 `robfig/cron/v3`。

* `gocron.CronJob(expr, withSeconds)` 支持 5 字段 cron；`withSeconds=true` 时支持 6 字段秒字段。
* `gocron.NewScheduler(gocron.WithLocation(location))` 支持 scheduler 默认时区；cron 字符串也支持 `CRON_TZ=...`。
* `Scheduler.NewJob` 可动态新增任务。
* `Scheduler.Update(jobID, definition, task, options...)` 可替换任务定义与执行函数，并保留原 job ID。
* `Scheduler.RemoveJob(jobID)` 与 `Scheduler.RemoveByTags(tags...)` 可动态删除任务。
* `gocron.WithSingletonMode(gocron.LimitModeReschedule)` 可避免同一 job 重叠运行；重叠触发会被重排/跳过，但仍需 spike 确认业务层是否能可靠记录 `skipped` 历史。
* `gocron.WithLimitConcurrentJobs(limit, mode)` 支持 scheduler-wide 并发限制；`LimitModeReschedule` 更贴近“超出时跳过/重排”的第一版语义。
* `Scheduler.Shutdown()` 用于优雅关闭 scheduler，并会取消传入任务函数的 `context.Context`。

## Spike 测试结果

已在 `internal/scheduler/gocron_spike_test.go` 中加入最小验证测试，并通过 `go test ./... -count=1`。

* `TestGoCronSpikeCronAndTimezone`：5 字段 cron、6 字段 cron、scheduler `WithLocation` 与 `CRON_TZ=...` 均可用。
* `TestGoCronSpikeDynamicJobLifecycle`：scheduler 启动后可新增、`Update`、`RemoveJob`，且 `Update` 保留原 job ID。
* `TestGoCronSpikeBusinessVisibleSkipOverlap`：通过 `BeforeJobRunsSkipIfBeforeFuncErrors`，业务层可以在发现同一任务已运行时记录 `skipped` 并跳过当前执行。
* `TestGoCronSpikeSchedulerWideLimit`：`WithLimitConcurrentJobs(1, LimitModeReschedule)` 可限制全局并发，且 reschedule 模式不会产生等待队列。
* `TestGoCronSpikeShutdownCancelsRunningJobContext`：`Shutdown` 会取消传入任务函数的 context，后续 runner 可据此处理停止语义。

## 需要用代码验证的风险点

* `CRON_TZ=...` 会按指定时区计算触发瞬间，但 `NextRun()` 返回值的 location 仍是 scheduler location。后续 UI/API 展示时应使用任务 timezone 重新格式化。
* gocron 的 singleton skip 本身不直接等同于 taskdaemon 执行历史里的 `skipped` 记录。更推荐在业务层使用 `BeforeJobRunsSkipIfBeforeFuncErrors` 判断 running state、写入 skipped 历史并返回 error 跳过本次执行。
* scheduler-wide 并发限制适合作为后续全局保护，但第一版的同任务 skip-overlap 不应只依赖全局限制。

## 推荐 spike 验证项

* Cron：分别验证 5 字段和 6 字段表达式能创建 job，并可通过 mock clock 或短周期测试触发。
* Timezone：验证 `WithLocation` 与 `CRON_TZ` 至少一种方案可满足任务级 timezone。
* Dynamic jobs：验证新增、更新、删除后 scheduler job 数量和触发行为符合预期。
* Skip overlap：已验证业务层监听器可观测并记录 skipped。
* Global concurrency：已验证 `WithLimitConcurrentJobs` 的上限行为，建议作为后续全局保护。
* Shutdown：已验证关闭时会取消任务 context，runner 仍需要负责终止外部进程并归档状态。

## 对 taskdaemon 的落地建议

第一版可以选用 gocron，但不要把 `skipped` 历史完全托付给库内部 singleton。更稳妥的实现是：

* scheduler 负责按 cron 触发任务。
* 每个任务注册 `BeforeJobRunsSkipIfBeforeFuncErrors`，在真正启动 runner 前检查 task-level running state。
* 已在运行时，业务层直接写一条 `skipped` 执行历史，返回 sentinel error，不启动新进程。
* gocron `WithSingletonMode(LimitModeReschedule)` 可作为第二道保护，避免业务层 bug 导致同一 job 并发。
* runner 任务函数接收 context，shutdown/cancel/timeout 路径统一通过 context 和进程终止封装协作。
