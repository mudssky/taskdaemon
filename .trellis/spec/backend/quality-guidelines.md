# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

Backend code should keep business behavior testable at package boundaries and avoid hiding product state transitions inside third-party library behavior. Scheduler behavior, runner lifecycle, persistence, auth, and API handlers need focused tests before implementation.

---

## Forbidden Patterns

* Do not rely on gocron singleton mode as the only source for taskdaemon `skipped` execution history. Library-level suppression is not a persisted business event by itself.
* Do not format scheduler `NextRun()` values for users without applying the task's intended timezone. `CRON_TZ=...` affects the trigger instant, but gocron can return that instant in the scheduler location.
* Do not put business logic in `cmd/taskdaemon`; keep it in `internal/*` packages.

---

## Required Patterns

* Use `github.com/go-co-op/gocron/v2` for first-version in-process scheduling.
* Record overlap skips in taskdaemon business code before the runner starts. Use gocron `BeforeJobRunsSkipIfBeforeFuncErrors` to check task-level running state, write `skipped`, and return a sentinel error to skip the current run.
* Treat gocron `WithSingletonMode(gocron.LimitModeReschedule)` as a second guard for no-overlap jobs, not the source of execution history.
* Use `WithLimitConcurrentJobs` only for scheduler-wide protection; same-task overlap remains a task-level rule.
* Runner tasks should accept `context.Context` where possible so shutdown, cancel, and timeout paths can cooperate with process termination.

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
