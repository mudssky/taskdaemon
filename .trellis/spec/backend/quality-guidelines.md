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
* `(*scheduler.Service).TriggerTask(ctx context.Context, taskID int) (*ent.Run, error)`
* `(*scheduler.Service).CancelTask(ctx context.Context, taskID int) error`
* `(*scheduler.Service).ListTaskRuns(ctx context.Context, taskID int, limit int) ([]*ent.Run, error)`
* `runner.Validate(cfg runner.Config) error`
* `runner.BuildCommand(cfg runner.Config) (runner.Command, error)`
* `runner.NewExecutor().Execute(ctx context.Context, cfg runner.Config) (runner.Result, error)`
* `POST /api/tasks`
* `POST /api/tasks/{id}/trigger`
* `POST /api/tasks/{id}/cancel`
* `GET /api/tasks/{id}/runs`
* `taskdaemon task trigger <task-id> --session-token <token>`
* `taskdaemon task cancel <task-id> --session-token <token>`

### 3. Contracts

* `CreateTaskInput.Enabled` is `*bool`: nil means default enabled, explicit false must persist a disabled task.
* Cron expressions accept 5 or 6 fields. Six-field or high-frequency cron returns soft warnings and requires `ConfirmCronWarnings=true` to save.
* Runner types are only `shell`, `bash`, `pwsh`, `python`, `node`, and `typescript`; TypeScript uses `tsx`.
* Runner config stores structured fields: `inline`, `scriptPath`, `args`, `workDir`, `env`, `timeoutSeconds`, and `outputLimitBytes`.
* Timeout defaults to 3600 seconds and must be persisted consistently in both `Task.timeout_seconds` and runner JSON.
* Execution history must store trigger, status, exit code, started/finished time, duration, error summary, and truncated stdout/stderr.
* `GET /api/tasks/{id}/runs` returns newest runs first and caps unbounded limits.
* CLI task trigger/cancel must send `taskdaemon_session=<token>` to daemon HTTP API. Token source may be `--session-token` or `TASKDAEMON_SESSION_TOKEN`.

### 4. Validation & Error Matrix

* Empty cron -> hard validation error.
* Cron field count not 5 or 6 -> hard validation error.
* Invalid timezone -> hard validation error.
* Seconds/high-frequency cron without confirmation -> `scheduler.ErrCronWarningsNeedConfirmation`.
* Unsupported runner type -> `runner.ErrUnsupportedType`.
* Missing `inline` and `scriptPath` -> `runner.ErrMissingCommand`.
* Timeout from runner execution -> run status `timeout`.
* Context cancellation from API/daemon shutdown/user cancel -> run status `cancelled`.
* Overlap trigger -> write `skipped` run before returning `scheduler.ErrTaskAlreadyRunning` to gocron.
* Cancel when task is not running -> `scheduler.ErrTaskNotRunning`, API maps to `409 task_not_running`.
* Missing CLI session token -> command returns an actionable error and must not call the daemon API.

### 5. Good/Base/Bad Cases

* Good: A logged-in CLI calls `POST /api/tasks/7/cancel`; the daemon process cancels the shared running context and the runner writes `cancelled`.
* Base: A manual trigger writes a `running` row before process start and finalizes it to `success`, `failed`, `timeout`, or `cancelled`.
* Bad: CLI opens the database and starts its own scheduler service for trigger/cancel; this can overlap with daemon jobs and cannot cancel daemon-owned processes.

### 6. Tests Required

* Scheduler tests assert cron 5/6 parsing, soft warning confirmation, structured runner persistence, skip-overlap, timeout, cancel, default timeout persistence, and newest-first run history.
* Runner tests assert command construction, `tsx` TypeScript default, context cancellation, timeout, nonzero exit mapping, and stdout/stderr truncation.
* HTTP tests assert task routes require session auth, map DTOs into service inputs, map `ErrTaskNotRunning` to `task_not_running`, and serialize run history.
* CLI/app tests assert task trigger/cancel parse positive IDs, carry session token from flag/env, and call daemon API with the `taskdaemon_session` cookie.

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
