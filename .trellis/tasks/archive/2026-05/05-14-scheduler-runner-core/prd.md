# 调度核心与 runner

## Goal

实现第一版可用的任务调度与执行核心，支持 cron/manual trigger、内置结构化 runner、执行历史、超时、取消和重叠触发处理。

## Requirements

* 使用进程内调度库；gocron spike 已通过，第一版选择 `github.com/go-co-op/gocron/v2`。
* 当前验证版本为 `gocron/v2 v2.21.2`，覆盖 cron、timezone、动态任务管理、skip-overlap 业务记录、全局并发限制和 shutdown context cancellation。
* 已完成子任务 `05-14-gocron-scheduler-spike`，结论见其 `research/gocron-capability.md`。
* 支持 5/6 字段 cron，秒字段可选。
* 支持手动触发任务。
* 高频/秒级 cron 属于软警告，显式确认后可保存。
* 同一任务默认 skip-overlap，重叠触发记录 `skipped`。
* `skipped` 由 taskdaemon 业务层记录：在 gocron `BeforeJobRunsSkipIfBeforeFuncErrors` 中检查 task-level running state，已运行则写 skipped 历史并返回 sentinel error 跳过本次执行。
* gocron `WithSingletonMode(LimitModeReschedule)` 作为第二道保护，不作为唯一 skipped 记录来源。
* 不依赖 Redis、Asynq 或外部分布式队列。
* 只允许内置结构化 runner 类型：`shell`、`bash`、`pwsh`、`python`、`node`、`typescript`。
* TypeScript runner 默认使用 `tsx`。
* 每个任务必须有 timeout，默认 1 小时。
* 超时后终止进程并记录 `timeout`。
* API/CLI 可以取消 running 任务，记录 `cancelled`。
* 执行历史保存状态、退出码、耗时、错误摘要和截断 stdout/stderr。

## Acceptance Criteria

* [x] 可创建并注册 cron 任务。
* [x] 完成 `gocron` spike，并记录最终选择。
* [x] `05-14-gocron-scheduler-spike` 完成并将结论同步回来。
* [x] cron 5/6 字段校验可用。
* [x] 软警告确认机制可用。
* [x] API/CLI 可手动触发任务。
* [x] runner 配置保存前经过结构化校验。
* [x] `tsx` 作为 TypeScript runner 默认执行器。
* [x] 重叠触发记录 `skipped`。
* [x] timeout 记录 `timeout`。
* [x] cancel 记录 `cancelled`。
* [x] stdout/stderr 截断保存。

## Testing Constraints

* 调度、runner、cron 校验、skip-overlap、timeout、cancel、执行历史采用 TDD。
* runner 测试优先覆盖命令构造、超时/取消、输出截断和状态归档。
* 样式、静态配置文件、文档不要求单元测试。

## Out of Scope

* 不实现插件系统。
* 不实现分布式任务队列。
* 不实现完整文件日志和日志轮转。

## Decision (ADR-lite)

### 调度库选择 gocron

**Context**: 第一版需要进程内 cron、5/6 字段 cron、timezone、动态任务新增/更新/删除、手动触发、同任务 skip-overlap、shutdown/cancel 与后续全局并发扩展能力。

**Decision**: 第一版使用 `github.com/go-co-op/gocron/v2`。当前 spike 验证版本为 `v2.21.2`，验证代码位于 `services/taskdaemon-go/internal/scheduler/gocron_spike_test.go`。

**Consequences**: gocron 满足第一版调度基础能力，并为 interval、singleton、scheduler-wide concurrency 保留扩展空间。`CRON_TZ` 可用于任务级 timezone，但 `NextRun()` 返回值仍按 scheduler location 表示同一触发瞬间，展示层需要按任务 timezone 格式化。`skipped` 历史不完全依赖 gocron singleton，而由 taskdaemon 业务层在执行前检查 running state 并写入。

## Parent

* `.trellis/tasks/05-14-cross-platform-scheduler-daemon/prd.md`
