# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

Backend code should keep business behavior testable at package boundaries and avoid hiding product state transitions inside third-party library behavior. Scheduler behavior, runner lifecycle, persistence, auth, and API handlers need focused tests before implementation.

---

## Forbidden Patterns

* Do not rely on gocron singleton mode as the only source for taskdaemon `skipped` execution history. Library-level suppression is not a persisted business event by itself.
* Do not format scheduler `NextRun()` values for users without applying the task's intended timezone. `CRON_TZ=...` affects the trigger instant, but gocron can return that instant in the scheduler location.
* Do not put business logic in `services/taskdaemon-go/cmd/taskdaemon`; keep it in `services/taskdaemon-go/internal/*` packages.

---

## Required Patterns

* Use `github.com/go-co-op/gocron/v2` for first-version in-process scheduling.
* Record overlap skips in taskdaemon business code before the runner starts. Use gocron `BeforeJobRunsSkipIfBeforeFuncErrors` to check task-level running state, write `skipped`, and return a sentinel error to skip the current run.
* Treat gocron `WithSingletonMode(gocron.LimitModeReschedule)` as a second guard for no-overlap jobs, not the source of execution history.
* Use `WithLimitConcurrentJobs` only for scheduler-wide protection; same-task overlap remains a task-level rule.
* Runner tasks should accept `context.Context` where possible so shutdown, cancel, and timeout paths can cooperate with process termination.
* CLI task actions that affect running daemon state must call the daemon HTTP API with the admin session cookie. Do not execute `task trigger` or `task cancel` in a short-lived CLI-local scheduler service, because that bypasses daemon overlap/cancel state.

---

## Scenario: Scheduler/Runner Core

### 1. Scope / Trigger

* Trigger: task definition, cron registration, runner execution, HTTP task routes, and CLI task commands form one cross-layer contract.
* Scope: `internal/scheduler` owns task business state and run history; `internal/runner` owns process execution; `internal/httpapi` exposes authenticated task routes; CLI task actions route through the running daemon API.

### 2. Signatures

* `scheduler.ValidateCron(input scheduler.CronValidationInput) (scheduler.CronValidationResult, error)`
* `scheduler.NewService(store *data.Store, opts scheduler.Options) *scheduler.Service`
* `(*scheduler.Service).CreateTask(ctx context.Context, input scheduler.CreateTaskInput) (*ent.Task, error)`
* `(*scheduler.Service).ListTasks(ctx context.Context, limit int) ([]*ent.Task, error)`
* `(*scheduler.Service).UpdateTask(ctx context.Context, taskID int, input scheduler.CreateTaskInput) (*ent.Task, error)`
* `(*scheduler.Service).SetTaskEnabled(ctx context.Context, taskID int, enabled bool) (*ent.Task, error)`
* `(*scheduler.Service).DeleteTask(ctx context.Context, taskID int) error`
* `(*scheduler.Service).TriggerTask(ctx context.Context, taskID int) (*ent.Run, error)`
* `(*scheduler.Service).CancelTask(ctx context.Context, taskID int) error`
* `(*scheduler.Service).ListTaskRuns(ctx context.Context, taskID int, limit int) ([]*ent.Run, error)`
* `(*scheduler.Service).IsTaskRunning(taskID int) bool`
* `runner.Validate(cfg runner.Config) error`
* `runner.BuildCommand(cfg runner.Config) (runner.Command, error)`
* `runner.NewExecutor().Execute(ctx context.Context, cfg runner.Config) (runner.Result, error)`
* `GET /api/tasks`
* `POST /api/tasks`
* `PUT /api/tasks/{id}`
* `PATCH /api/tasks/{id}/enabled`
* `DELETE /api/tasks/{id}`
* `POST /api/tasks/{id}/trigger`
* `POST /api/tasks/{id}/cancel`
* `GET /api/tasks/{id}/runs`
* `taskdaemon task trigger <task-id> --session-token <token>`
* `taskdaemon task cancel <task-id> --session-token <token>`
* `taskdaemon task runs <task-id> --session-token <token>`

### 3. Contracts

* `CreateTaskInput.Enabled` is `*bool`: nil means default enabled, explicit false must persist a disabled task.
* Cron expressions accept 5 or 6 fields. Six-field or high-frequency cron returns soft warnings and requires `ConfirmCronWarnings=true` to save.
* Runner types are only `shell`, `bash`, `pwsh`, `python`, `node`, and `typescript`; TypeScript uses `tsx`.
* Runner config stores structured fields: `inline`, `scriptPath`, `args`, `workDir`, `env`, `timeoutSeconds`, and `outputLimitBytes`.
* Timeout defaults to 3600 seconds and must be persisted consistently in both `Task.timeout_seconds` and runner JSON.
* Task list/update/enable APIs are authenticated and return task DTOs with `id`, `name`, `description`, `enabled`, `cronExpression`, `timezone`, `runnerType`, `runnerConfig`, `timeoutSeconds`, `overlapPolicy`, `running`, `createdAt`, and `updatedAt`.
* `running` is a process-local scheduler view from `IsTaskRunning`; it is not persisted and must not be treated as database state.
* `UpdateTask` is a full replacement of editable task definition fields and must re-run the same cron/runner validation as `CreateTask`.
* `SetTaskEnabled` must synchronize the in-process cron scheduler when a scheduler exists: disabled tasks remove registered jobs, enabled tasks register a validated cron job.
* `DeleteTask` must reject currently running tasks with `ErrTaskAlreadyRunning`; callers should cancel first, then delete after the run is finalized.
* `DeleteTask` removes the in-process cron job, deletes the task's execution history, and then deletes the task definition. Deleting run rows first avoids orphaned or foreign-key-blocked task deletes.
* Execution history must store trigger, status, exit code, started/finished time, duration, error summary, and truncated stdout/stderr.
* `GET /api/tasks/{id}/runs` returns newest runs first and caps unbounded limits.
* CLI task trigger/cancel/runs must send `taskdaemon_session=<token>` to daemon HTTP API. Token source may be `--session-token` or `TASKDAEMON_SESSION_TOKEN`.
* `taskdaemon task runs <task-id>` calls `/api/tasks/{id}/runs` on the running daemon and prints a YAML list of recent run summaries. It must not open the database directly because the daemon owns process-local running state and API auth.

### 4. Validation & Error Matrix

* Empty cron -> hard validation error.
* Cron field count not 5 or 6 -> hard validation error.
* Invalid timezone -> hard validation error.
* Seconds/high-frequency cron without confirmation -> `scheduler.ErrCronWarningsNeedConfirmation`.
* Unsupported runner type -> `runner.ErrUnsupportedType`.
* Missing `inline` and `scriptPath` -> `runner.ErrMissingCommand`.
* Task list failure -> API maps to `500 task_list_failed`.
* Task create/update validation failure -> API maps to `400 task_invalid` with safe validation details.
* Invalid task enabled request body -> API maps to `400 bad_request`.
* Delete running task -> `scheduler.ErrTaskAlreadyRunning`, API maps to `409 task_running`.
* Delete database or cron removal failure -> API maps to `409 task_delete_failed`.
* Timeout from runner execution -> run status `timeout`.
* Context cancellation from API/daemon shutdown/user cancel -> run status `cancelled`.
* Overlap trigger -> write `skipped` run before returning `scheduler.ErrTaskAlreadyRunning` to gocron.
* Cancel when task is not running -> `scheduler.ErrTaskNotRunning`, API maps to `409 task_not_running`.
* Missing CLI session token -> command returns an actionable error and must not call the daemon API.

### 5. Good/Base/Bad Cases

* Good: A logged-in CLI calls `POST /api/tasks/7/cancel`; the daemon process cancels the shared running context and the runner writes `cancelled`.
* Good: A logged-in CLI calls `GET /api/tasks/7/runs` through `taskdaemon task runs 7`; output contains run ID, trigger, status, exit code, duration and truncated stdout/stderr.
* Good: A logged-in Web UI edits a task through `PUT /api/tasks/7`; scheduler validates the full definition, updates Ent fields, removes any previous cron job, and registers the new one when enabled.
* Good: A logged-in Web UI deletes a stopped task through `DELETE /api/tasks/7`; scheduler removes its cron registration, run history and task row.
* Base: A manual trigger writes a `running` row before process start and finalizes it to `success`, `failed`, `timeout`, or `cancelled`.
* Base: `GET /api/tasks` returns at most the service default limit and includes process-local `running` flags for UI controls.
* Bad: CLI opens the database and starts its own scheduler service for trigger/cancel; this can overlap with daemon jobs and cannot cancel daemon-owned processes.
* Bad: UI deletes a running task directly; it must disable destructive delete while `Task.running=true` and let the user cancel first.
* Bad: UI infers running state by scanning the latest run row instead of using the `running` field returned by the daemon service.

### 6. Tests Required

* Scheduler tests assert cron 5/6 parsing, soft warning confirmation, structured runner persistence, skip-overlap, timeout, cancel, default timeout persistence, and newest-first run history.
* Scheduler tests assert newest-first task listing, full task update persistence, and dynamic cron job add/remove when enabling or disabling tasks.
* Scheduler tests assert deleting a stopped task removes cron registration plus run history, and deleting a running task returns `ErrTaskAlreadyRunning`.
* Runner tests assert command construction, `tsx` TypeScript default, context cancellation, timeout, nonzero exit mapping, and stdout/stderr truncation.
* HTTP tests assert task routes require session auth, map create/update/enable DTOs into service inputs, map `ErrTaskNotRunning` to `task_not_running`, map running delete to `task_running`, expose task list running state, and serialize run history.
* CLI/app tests assert task trigger/cancel/runs parse positive IDs, carry session token from flag/env, and call daemon API with the `taskdaemon_session` cookie.

### 7. Wrong vs Correct

#### Wrong

```go
// CLI 本地启动临时 service，绕开 daemon 的 running map。
return scheduler.NewService(store, scheduler.Options{Runner: runner.NewExecutor()}).
    CancelTask(ctx, taskID)
```

#### Correct

```go
// CLI 通过 daemon API 命中正在运行的 scheduler service。
req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/api/tasks/7/cancel", nil)
req.AddCookie(&http.Cookie{Name: httpapi.SessionCookieName, Value: sessionToken})
```

#### Wrong

```go
// CLI 直接打开数据库查询历史，绕过 daemon 认证与统一 API envelope。
runs, _ := store.Client().Run.Query().All(ctx)
```

#### Correct

```go
// CLI 通过 daemon API 查询历史，和 Web UI 使用同一条受保护路径。
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/api/tasks/7/runs", nil)
req.AddCookie(&http.Cookie{Name: httpapi.SessionCookieName, Value: sessionToken})
```

---

## Testing Requirements

* Scheduler tests must cover cron 5/6 field parsing, timezone behavior, dynamic add/update/remove, overlap skip recording, global concurrency limits when used, and shutdown/cancel context behavior.
* Prefer fake clocks for deterministic schedule tests. Use real short intervals only for concurrency behavior that cannot be usefully proven with a fake clock.
* Tests that use real timers must include explicit timeouts and must not block indefinitely.

---

## Code Review Checklist

* Verify every exported or helper function added in Go code has parameter and return-value documentation when required by project policy.
* Verify scheduler behavior that creates business history (`skipped`, `timeout`, `cancelled`, `failed`, `success`) is asserted in tests at the taskdaemon layer, not only inferred from gocron internals.
* Verify timezone-related code distinguishes stored trigger instants from display timezone formatting.
