# 调度库调研：robfig/cron 与 gocron

## 背景

第一轮实施目标是“调度核心可用”，需要选择 Go 调度库和第一版调度表达式范围。已确认需求包括：

* cron + 手动触发。
* 5/6 字段 cron，秒字段可选。
* 高频 cron 软警告，确认后可保存。
* 同一任务默认 skip-overlap。
* 未来可能支持 interval、更多并发策略和任务级策略。

## 资料来源

* Context7: `/robfig/cron`
* Context7: `/go-co-op/gocron`

## robfig/cron

* `robfig/cron/v3` 是 Go 生态常用 cron 调度库。
* 默认支持标准 5 字段 cron 表达式。
* 可通过 `cron.WithSeconds()` 支持 6 字段 seconds cron。
* 可通过自定义 parser 支持“秒字段可选 + descriptor”的表达式形式。
* `AddFunc` 返回 entry id，可用于跟踪和管理已注册任务。
* `Stop()` 可等待运行中的任务完成，适合守护进程优雅退出。
* v3 提供 job wrappers，可用于 panic recovery、并发控制等横切逻辑。

优点：

* 小而专注，cron 表达式能力强。
* 对 5/6 字段 cron 和自定义 parser 很直接。
* 适合第一版只做 cron + manual trigger。

缺点：

* interval、singleton、全局并发限制等产品级调度能力需要自己包一层。
* skip-overlap 虽可通过 wrapper 或业务层实现，但执行历史中的 `skipped` 语义仍要自己处理。

## gocron

* `gocron` 是更偏“任务调度器”的库，支持 interval、cron、多种 job 类型。
* 支持 `WithSingletonMode` 防止同一 job 重叠执行。
  * `LimitModeReschedule` 可跳过重叠执行。
  * `LimitModeWait` 可排队等待。
* 支持 `WithLimitConcurrentJobs` 做 scheduler-wide 并发限制。

优点：

* 内置能力更贴近 taskdaemon 未来产品形态。
* skip-overlap、排队、全局并发限制等后续策略更容易表达。
* 未来支持 interval 时更自然，不需要另写一套调度抽象。

缺点：

* 比 robfig/cron 更重、更抽象。
* 如果第一版只做 cron，使用 gocron 可能显得稍多。
* 需要确认 cron 5/6 字段、timezone、任务更新/删除、持久化恢复等细节是否完全满足需求。

## 映射到本项目

两种方案都可行：

### 方案 A：robfig/cron + 自研调度服务封装

适合“第一版只做 cron，尽量小依赖”的路线。

* 调度库只负责 cron tick。
* skip-overlap、执行历史、manual trigger、并发上限由业务服务封装。

### 方案 B：gocron + 调度服务封装

适合“第一版略多一点依赖，换未来 interval/并发策略更顺”的路线。

* 使用 gocron 承载 cron/interval/singleton/全局并发能力。
* 业务层仍负责执行历史、runner、manual trigger、持久化恢复。

## 当前推荐

如果严格按当前第一轮范围，`robfig/cron/v3` 足够且更轻。

但结合已确认的 skip-overlap、未来 interval、全局并发控制、任务级策略等方向，**建议重新考虑 gocron**。它更像 taskdaemon 需要的“调度产品内核”，可以减少后续自研 wrapper 的复杂度。

建议实施前做一个小 spike：

* 验证 gocron 是否支持 5/6 字段 cron 或如何配置 seconds。
* 验证 timezone。
* 验证任务新增、更新、删除。
* 验证 singleton skip 能否配合业务层记录 `skipped`。
* 验证 shutdown 语义。

## 第一版调度模型

无论选择哪个库，第一版保持：

* 数据模型保存规范化 schedule：
  * `schedule_type`: `cron` / `manual`
  * `cron_expr`: 标准 cron 表达式
  * `timezone`: IANA timezone，默认 local 或 UTC 待确认
* 支持手动触发任务，便于 CLI/API 验证 runner。
* interval 可以作为 UI/API 快捷输入，内部转换为 cron，或放到后续版本。

## 待确认

* 最终选择 `robfig/cron/v3` 还是 `gocron`。
* cron 是否支持秒级字段：5 字段更符合传统 cron；6 字段适合更高频任务，但可能增加误触发风险。
* timezone 默认值：本机 local 更符合桌面工具直觉；UTC 更适合服务端/公网部署。
