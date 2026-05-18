# Scheduler 与 Runner 生命周期规范

> 调度和执行历史是 taskdaemon 的核心业务语义，不能依赖第三方库默认行为来推断产品状态。

---

## 概览

`internal/scheduler` 负责任务定义、cron 注册、手动触发、取消、overlap 策略和执行历史写入。`internal/runner` 负责把结构化 runner 配置转换为进程命令，并将进程结果映射为稳定业务状态。

调度层可以使用 `github.com/go-co-op/gocron/v2` 执行 cron，但 taskdaemon 的业务状态必须由自己的 service 和数据库记录表达。

---

## 核心边界

* 任务定义持久化在数据库，包含 cron、timezone、runner 类型、runner 配置、timeout、启停状态和 overlap 策略。
* 执行历史持久化在数据库，包含 trigger、status、exit code、开始/结束时间、耗时、错误摘要和截断输出。
* `running` 是当前 daemon 进程内 scheduler 视图，不写入数据库。
* UI/API 查询的执行历史来自数据库，不从 gocron job 状态或最新 run 记录反推任务定义。
* handler、CLI 和 Desktop binding 不直接调用 gocron 或 exec；它们通过 service/API 边界触发行为。

---

## Cron 校验

* 支持 5 字段和 6 字段 cron。
* timezone 使用 Go `time.LoadLocation` 校验，空值标准化为 `Local`。
* 非法 cron、非法 timezone 和字段数错误是硬错误，阻止保存。
* 秒级 cron 和高频 cron 是软警告，需要调用方显式确认后才能保存。
* 软警告使用稳定 warning code，例如 `second_level_cron`、`high_frequency_cron`。
* 软警告不能伪装成硬错误；前端和 API 应能区分“不能保存”和“确认风险后可保存”。

---

## Runner 配置

runner 配置必须是结构化字段，不提供长期裸命令入口作为唯一抽象。

当前内置类型：

* `shell`
* `bash`
* `pwsh`
* `python`
* `node`
* `typescript`

规则：

* `runner.Validate` 负责类型、命令存在性和 timeout 合法性。
* `runner.BuildCommand` 负责把结构化配置转换为进程名和参数。
* shell runner 在 Windows 使用 `cmd /C`，其他平台使用 `sh -c`。
* interpreter runner 优先支持 inline，也支持 script path + args。
* `WorkDir` 和 `Env` 只传给当前执行进程，不写入日志明文和错误响应。
* 新增 runner 类型时必须补命令构造测试、执行测试、scheduler JSON round-trip 测试和前端 DTO/schema 映射。

---

## Timeout、Cancel 与状态映射

runner 结果状态固定为：

* `success`：进程退出码为 0。
* `failed`：进程启动失败或非零退出，且不是 timeout/cancel。
* `timeout`：runner timeout 到期。
* `cancelled`：调用方 context 取消。

数据库 run 状态还包含：

* `skipped`：同一任务 overlap，被业务规则跳过。

规则：

* 进程非零退出映射到 `Result.Status=failed`，不作为 `Execute` 的 error 返回。
* 配置非法、runner 类型不支持、命令缺失等前置错误作为 error 返回。
* timeout 和 cancel 必须写入执行历史，不折叠成 generic failed。
* `ErrorSummary` 只保存截断摘要，不保存无限 stderr 或敏感环境变量。
* 调用方 context 必须向 runner 执行传播，让 API cancel、CLI cancel 和 daemon shutdown 生效。

---

## Overlap 与运行态

当前 overlap 策略为 skip。

* 同一任务开始执行前，scheduler service 先通过进程内 `running` map 登记运行态。
* 已有运行态时，不进入 runner，写入 `skipped` 执行历史。
* gocron singleton/limit 只能作为防御性保护，不能作为执行历史来源。
* 删除运行中任务返回 `ErrTaskAlreadyRunning`，不强删数据库历史。
* 取消未运行任务返回 `ErrTaskNotRunning`。
* `IsTaskRunning` 只表达当前 daemon 进程内视图；重启后 running 视图清空。

---

## 执行历史

* 手动触发使用 `trigger=manual`。
* cron 触发使用 `trigger=cron`。
* 开始执行时先创建 run 记录，再执行 runner，最后更新最终状态。
* 无法恢复 runner 配置或进入 runner 前失败时，更新为 `failed` 并写入错误摘要。
* stdout/stderr 必须受 `OutputLimitBytes` 限制，默认限制由 runner 层提供。
* 执行历史列表必须设置默认 limit 或分页，避免无上限返回日志。
* `DurationMs` 应以 runner 实际耗时和 service 观察时间中较大值兜底，避免记录负数或明显不合理值。

---

## Runner JSON 字段

任务定义中的 runner JSON 是持久化边界，字段名需要稳定：

* `type`
* `inline`
* `scriptPath`
* `args`
* `workDir`
* `env`
* `timeoutSeconds`
* `outputLimitBytes`
* `cronWarnings`

规则：

* `timeoutSeconds` 同时写入任务列和 runner JSON，保证 API/前端兼容。
* JSON -> `runner.Config` 恢复逻辑必须容忍缺失字段并使用默认值。
* 新增字段时必须保证旧任务仍可读取。
* 不把未分类任意 JSON 当作 runner 扩展点；新增字段应有业务含义、校验和测试。

---

## 执行契约速查

### 1. Scope / Trigger

触发条件：修改 cron 校验、任务启停/删除/触发/取消、runner 类型、runner JSON 字段、执行状态、overlap 策略、timeout/cancel 或执行历史写入。

### 2. Signatures

* cron 校验：`ValidateCron(input CronValidationInput) (CronValidationResult, error)`。
* 任务输入：`CreateTaskInput{CronExpression, Timezone, ConfirmCronWarnings, Runner}`。
* runner 校验：`runner.Validate(cfg runner.Config) error`。
* runner 执行：`Executor.Execute(ctx context.Context, cfg Config) (Result, error)`。
* 执行历史：`TriggerTask(ctx context.Context, taskID int) (*ent.Run, error)` 和 `CancelTask(ctx context.Context, taskID int) error`。

### 3. Contracts

* cron：5/6 字段；软警告用稳定 warning code；未确认软警告返回 `ErrCronWarningsNeedConfirmation`。
* runner：类型白名单、inline/scriptPath 二选一、timeout 正数或默认值、输出限制默认值。
* 状态：runner status 映射为 run status；overlap 额外写 `skipped`。
* JSON：runner JSON 字段名稳定，旧任务缺失新增字段时可读取。

### 4. Validation & Error Matrix

| 条件 | 返回/状态 | 说明 |
|---|---|---|
| cron 字段数非法 | error | 阻止保存 |
| 秒级或高频 cron 未确认 | `ErrCronWarningsNeedConfirmation` | 返回 warning code |
| runner 类型不支持 | `runner.ErrUnsupportedType` | 阻止保存/执行 |
| 缺少 inline 和 scriptPath | `runner.ErrMissingCommand` | 阻止保存/执行 |
| 同任务已运行 | run status `skipped` 或 `ErrTaskAlreadyRunning` | trigger 写历史，delete 返回冲突 |
| cancel 未运行任务 | `ErrTaskNotRunning` | API 映射 409 |
| 进程非零退出 | run status `failed` | `Execute` 不返回 error |
| context timeout/cancel | `timeout` / `cancelled` | 必须写执行历史 |

### 5. Good / Base / Bad Cases

* Good：新增 runner 字段时，同步 `runner.Config`、API DTO、scheduler JSON round-trip、前端 schema 和执行测试。
* Base：只调整 timeout 默认值时，同步 runner 默认、scheduler 持久化字段和相关测试。
* Bad：handler 直接 `exec.CommandContext`，或把 gocron job 是否存在当作执行历史。

### 6. Tests Required

* cron 硬错误、软警告、确认保存。
* runner type/build/execute、timeout/cancel、非零退出、输出截断。
* scheduler trigger/cancel/overlap/delete 与执行历史状态。
* runner JSON 新旧字段兼容和前后端 DTO 映射。

### 7. Wrong vs Correct

#### Wrong

```go
cmd := exec.CommandContext(ctx, "sh", "-c", req.Command)
```

#### Correct

```go
result, err := runner.NewExecutor().Execute(ctx, runner.Config{Type: runner.TypeShell, Inline: inline})
```

---

## Context 与并发

* scheduler、runner、data 路径必须接受并传递 `context.Context`。
* 运行态 map、job map 和 scheduler 引用必须通过 mutex 保护。
* 不在数据库事务中运行外部进程。
* 长时间 runner 执行外部进程时，只持有进程 context，不持有数据库事务或全局锁。
* daemon shutdown 应停止 scheduler，并让正在执行的 runner 能通过 context 取消。

---

## 测试要求

Scheduler 测试至少覆盖：

* cron 5/6 字段、timezone、秒级/高频软警告和确认流程。
* 创建、更新、启停、删除任务时的 cron 注册/移除。
* 手动触发、cron 触发、overlap skip、timeout、cancel 和执行历史排序。
* `ErrTaskAlreadyRunning`、`ErrTaskNotRunning` 等稳定错误。
* runner JSON round-trip，尤其 args/env/timeout/output limit。

Runner 测试至少覆盖：

* 每种 runner 类型的命令构造。
* 跨平台 shell 命令 helper。
* context cancel、timeout、非零退出、stdout/stderr 截断。
* unsupported type、missing command、invalid timeout。

使用真实 timer 的测试必须设置明确超时；能用 fake clock 时优先 fake clock。

---

## 禁止模式

* 不把 gocron 内部状态当作产品执行历史。
* 不从最新 run 记录反推任务是否正在运行。
* 不在 handler、CLI 或 Desktop binding 中直接启动 runner。
* 不把完整 stdout/stderr、runner env、脚本内容或带凭证路径写入日志和错误响应。
* 不在事务中执行外部进程。
* 不新增未校验的任意命令字符串作为长期 runner 抽象。
