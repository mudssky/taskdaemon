# gocron 调度库 spike

## Goal

验证 gocron 是否适合作为 taskdaemon 第一版调度核心。若关键能力验证通过，第一版优先使用 gocron；若不满足 5/6 字段 cron、timezone、动态任务管理、skip-overlap 记录或 shutdown 等需求，则回退 `robfig/cron/v3`。

## Requirements

* 验证 gocron 支持 5 字段 cron。
* 验证 gocron 支持 6 字段 cron 或可通过配置实现秒字段。
* 验证 timezone 配置方式。
* 验证动态新增、更新、删除任务。
* 验证 singleton/skip-overlap 能否配合业务层记录 `skipped` 执行历史。
* 验证 scheduler-wide 并发限制是否适合后续扩展。
* 验证 shutdown 语义，确保退出时不留下调度 goroutine。
* 粗略记录依赖和体积影响。
* spike 需要形成可运行 Go 测试或最小验证代码，避免只凭文档结论选型。
* 整体目录结构已确认采用根 Go module + `cmd/`、`internal/`、`web/` 的轻量 monorepo 方案。
* spike 代码落在正式 Go module 的 `internal/scheduler` 附近，便于后续调度核心任务复用。

## Acceptance Criteria

* [x] 有最小 Go spike 代码或测试覆盖 5/6 字段 cron。
* [x] 有最小 Go spike 代码或测试覆盖 timezone。
* [x] 有最小 Go spike 代码或测试覆盖新增/更新/删除任务。
* [x] 有最小 Go spike 代码或测试覆盖 skip-overlap，并说明如何记录 `skipped`。
* [x] 有结论文档：使用 gocron 或回退 `robfig/cron/v3`。
* [x] 结论同步回父任务 PRD。
* [x] 若 gocron 不直接暴露 skipped 事件，需要明确 taskdaemon 业务层如何记录 `skipped`。
* [x] 记录 spike 对后续正式 scheduler service 的建议边界。

## Testing Constraints

* spike 优先用可运行测试验证 gocron 行为，而不是只写样例 main。
* 不为文档和静态配置编写测试。

## Out of Scope

* 不实现完整 scheduler service。
* 不接入真实 runner。
* 不实现 UI。
* 不实现任务数据库持久化。
* 不实现完整 cron 风险提示或软警告确认流程。

## Research References

* [`research/gocron-capability.md`](research/gocron-capability.md) — Context7 文档显示 gocron v2 覆盖 cron、timezone、动态任务、singleton、全局并发限制与 shutdown；仍需代码验证 skipped 可观测性与 shutdown 细节。

## Technical Approach

优先用测试驱动 spike：

* 创建项目根 Go module，并在 `internal/scheduler` 附近放置最小测试，后续 scheduler 任务可以复用依赖和测试经验。
* 通过短周期 duration job 或 cron 秒字段验证触发行为；对 cron parser、timezone、动态更新删除、overlap、shutdown 分别写小测试。
* 对 `skipped` 记录能力重点判断：gocron 是否能直接告诉业务层“本次触发被跳过”；如果不能，结论应建议 taskdaemon 在业务层维护 task-level running state 并自行写历史，gocron singleton 仅作为保护。

## Decision (ADR-lite)

**Context**: 第一版调度核心需要进程内 cron、5/6 字段 cron、timezone、动态任务管理、同任务 skip-overlap、shutdown/cancel 语义，并希望后续可以扩展 interval、singleton 和全局并发限制。

**Decision**: 使用 `github.com/go-co-op/gocron/v2` 作为第一版调度库，当前验证版本为 `v2.21.2`。

**Consequences**:

* gocron 覆盖 5/6 字段 cron、scheduler timezone、`CRON_TZ`、动态新增/更新/删除、scheduler-wide 并发限制和 shutdown context cancellation。
* `CRON_TZ` 会按指定时区计算触发瞬间，但 `NextRun()` 返回值 location 仍是 scheduler location；后续展示需按任务 timezone 格式化。
* `skipped` 执行历史由 taskdaemon 业务层记录：在 `BeforeJobRunsSkipIfBeforeFuncErrors` 中检查 task-level running state，已运行则写 skipped 历史并返回 sentinel error 跳过本次执行。
* gocron `WithSingletonMode(LimitModeReschedule)` 可作为第二道保护，不作为唯一 skipped 记录来源。
* `WithLimitConcurrentJobs` 适合作为后续全局并发保护；第一版同任务 overlap 仍按任务维度处理。

## Implementation Evidence

* Spike 测试：`internal/scheduler/gocron_spike_test.go`
* 验证命令：`go test ./... -count=1`
* 结论文档：`research/gocron-capability.md`

## Parent

* `.trellis/tasks/05-14-scheduler-runner-core/prd.md`
