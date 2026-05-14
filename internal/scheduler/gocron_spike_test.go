package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
)

var errTaskAlreadyRunning = errors.New("task already running")

// TestGoCronSpikeCronAndTimezone 验证 gocron 对 5/6 字段 cron 与 timezone 的基础支持。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestGoCronSpikeCronAndTimezone(t *testing.T) {
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.May, 14, 9, 0, 0, 0, chicago))
	s, err := gocron.NewScheduler(
		gocron.WithClock(fakeClock),
		gocron.WithLocation(chicago),
	)
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	defer shutdownScheduler(t, s)

	fiveField, err := s.NewJob(
		gocron.CronJob("30 9 * * *", false),
		gocron.NewTask(func() {}),
	)
	if err != nil {
		t.Fatalf("create 5-field cron job: %v", err)
	}

	sixField, err := s.NewJob(
		gocron.CronJob("30 9 * * * *", true),
		gocron.NewTask(func() {}),
	)
	if err != nil {
		t.Fatalf("create 6-field cron job: %v", err)
	}

	tzField, err := s.NewJob(
		gocron.CronJob("CRON_TZ=Asia/Hong_Kong 30 22 * * *", false),
		gocron.NewTask(func() {}),
	)
	if err != nil {
		t.Fatalf("create CRON_TZ cron job: %v", err)
	}

	s.Start()
	if err := fakeClock.BlockUntilContext(context.Background(), 3); err != nil {
		t.Fatalf("wait timers: %v", err)
	}

	fiveNext := mustNextRun(t, fiveField)
	if fiveNext.Location().String() != chicago.String() {
		t.Fatalf("5-field cron location = %s, want %s", fiveNext.Location(), chicago)
	}
	if fiveNext.Hour() != 9 || fiveNext.Minute() != 30 || fiveNext.Second() != 0 {
		t.Fatalf("5-field cron next run = %s, want 09:30:00", fiveNext)
	}

	sixNext := mustNextRun(t, sixField)
	if sixNext.Hour() != 9 || sixNext.Minute() != 9 || sixNext.Second() != 30 {
		t.Fatalf("6-field cron next run = %s, want 09:09:30", sixNext)
	}

	// CRON_TZ 会按指定时区计算触发瞬间，但 NextRun 仍以 scheduler 默认 location 返回。
	tzNext := mustNextRun(t, tzField)
	wantHongKongInstant := time.Date(2026, time.May, 14, 22, 30, 0, 0, time.FixedZone("HKT", 8*60*60))
	if !tzNext.Equal(wantHongKongInstant) {
		t.Fatalf("CRON_TZ next run = %s, want same instant as %s", tzNext, wantHongKongInstant)
	}
	if tzNext.Location().String() != chicago.String() {
		t.Fatalf("CRON_TZ returned location = %s, want scheduler location %s", tzNext.Location(), chicago)
	}
}

// TestGoCronSpikeDynamicJobLifecycle 验证运行中的 scheduler 可以新增、更新、删除任务。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestGoCronSpikeDynamicJobLifecycle(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	s, err := gocron.NewScheduler(gocron.WithClock(fakeClock))
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	defer shutdownScheduler(t, s)

	var firstRuns atomic.Int64
	job, err := s.NewJob(
		gocron.DurationJob(10*time.Second),
		gocron.NewTask(func() {
			firstRuns.Add(1)
		}),
	)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	s.Start()
	if err := fakeClock.BlockUntilContext(context.Background(), 1); err != nil {
		t.Fatalf("wait first timer: %v", err)
	}

	advanceAndWait(t, fakeClock, time.Second*10, func() bool {
		return firstRuns.Load() == 1
	})

	var updatedRuns atomic.Int64
	updated, err := s.Update(
		job.ID(),
		gocron.DurationJob(5*time.Second),
		gocron.NewTask(func() {
			updatedRuns.Add(1)
		}),
	)
	if err != nil {
		t.Fatalf("update job: %v", err)
	}
	if updated.ID() != job.ID() {
		t.Fatalf("updated job ID = %s, want original %s", updated.ID(), job.ID())
	}
	if err := fakeClock.BlockUntilContext(context.Background(), 1); err != nil {
		t.Fatalf("wait updated timer: %v", err)
	}

	advanceAndWait(t, fakeClock, time.Second*5, func() bool {
		return updatedRuns.Load() == 1
	})

	if err := s.RemoveJob(job.ID()); err != nil {
		t.Fatalf("remove job: %v", err)
	}
	if got := len(s.Jobs()); got != 0 {
		t.Fatalf("jobs after remove = %d, want 0", got)
	}
}

// TestGoCronSpikeBusinessVisibleSkipOverlap 验证业务层可以观测 overlap 并记录 skipped。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestGoCronSpikeBusinessVisibleSkipOverlap(t *testing.T) {
	s, err := gocron.NewScheduler()
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	defer shutdownScheduler(t, s)

	var running atomic.Bool
	var started atomic.Int64
	var skipped atomic.Int64
	release := make(chan struct{})

	_, err = s.NewJob(
		gocron.DurationJob(20*time.Millisecond),
		gocron.NewTask(func() {
			started.Add(1)
			<-release
			running.Store(false)
		}),
		gocron.WithEventListeners(
			gocron.BeforeJobRunsSkipIfBeforeFuncErrors(func(_ uuid.UUID, _ string) error {
				if !running.CompareAndSwap(false, true) {
					skipped.Add(1)
					return errTaskAlreadyRunning
				}
				return nil
			}),
		),
	)
	if err != nil {
		t.Fatalf("create singleton job: %v", err)
	}

	s.Start()
	waitUntil(t, time.Second, func() bool {
		return started.Load() == 1 && skipped.Load() >= 1
	})

	if started.Load() != 1 {
		t.Fatalf("started runs = %d, want 1 while first run is blocked", started.Load())
	}
	if skipped.Load() < 1 {
		t.Fatal("expected at least one business-visible skipped run")
	}
	close(release)
}

// TestGoCronSpikeSchedulerWideLimit 验证 scheduler-wide 并发限制可以限制同时执行数。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestGoCronSpikeSchedulerWideLimit(t *testing.T) {
	s, err := gocron.NewScheduler(
		gocron.WithLimitConcurrentJobs(1, gocron.LimitModeReschedule),
	)
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	defer shutdownScheduler(t, s)

	var active atomic.Int64
	var maxActive atomic.Int64
	var started atomic.Int64
	release := make(chan struct{})

	task := func() {
		current := active.Add(1)
		updateMax(&maxActive, current)
		started.Add(1)
		<-release
		active.Add(-1)
	}

	for i := 0; i < 3; i++ {
		_, err = s.NewJob(
			gocron.DurationJob(20*time.Millisecond),
			gocron.NewTask(task),
		)
		if err != nil {
			t.Fatalf("create limited job %d: %v", i, err)
		}
	}

	s.Start()
	waitUntil(t, time.Second, func() bool {
		return started.Load() == 1
	})
	time.Sleep(80 * time.Millisecond)
	close(release)

	if got := maxActive.Load(); got != 1 {
		t.Fatalf("max concurrent jobs = %d, want 1", got)
	}
	if got := s.JobsWaitingInQueue(); got != 0 {
		t.Fatalf("jobs waiting in queue = %d, want 0 for LimitModeReschedule", got)
	}
}

// TestGoCronSpikeShutdownCancelsRunningJobContext 验证 shutdown 会取消传入任务的 context。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatal/t.Fatalf 终止。
func TestGoCronSpikeShutdownCancelsRunningJobContext(t *testing.T) {
	s, err := gocron.NewScheduler()
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	started := make(chan struct{})
	cancelled := make(chan struct{})

	_, err = s.NewJob(
		gocron.DurationJob(time.Hour),
		gocron.NewTask(func(ctx context.Context) {
			close(started)
			<-ctx.Done()
			close(cancelled)
		}),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		t.Fatalf("create context job: %v", err)
	}

	s.Start()
	waitForSignal(t, started, time.Second, "job start")

	if err := s.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	waitForSignal(t, cancelled, time.Second, "job context cancellation")
}

// shutdownScheduler 关闭 scheduler，忽略已经关闭或未启动导致的幂等细节。
//
// 参数:
//   - t: Go 测试上下文。
//   - s: 待关闭的 gocron scheduler。
//
// 返回值:
//   - 无。关闭失败时通过 t.Fatalf 终止。
func shutdownScheduler(t *testing.T, s gocron.Scheduler) {
	t.Helper()
	if err := s.Shutdown(); err != nil {
		t.Fatalf("shutdown scheduler: %v", err)
	}
}

// mustNextRun 读取 job 下一次运行时间。
//
// 参数:
//   - t: Go 测试上下文。
//   - job: 待读取的 gocron job。
//
// 返回值:
//   - time.Time: job 下一次计划运行时间。
func mustNextRun(t *testing.T, job gocron.Job) time.Time {
	t.Helper()
	next, err := job.NextRun()
	if err != nil {
		t.Fatalf("next run: %v", err)
	}
	return next
}

// advanceAndWait 推进 fake clock 并等待断言成立。
//
// 参数:
//   - t: Go 测试上下文。
//   - fakeClock: gocron 使用的 fake clock。
//   - duration: 要推进的时间。
//   - done: 返回 true 表示预期状态已达成。
//
// 返回值:
//   - 无。超时时通过 t.Fatal 终止。
func advanceAndWait(t *testing.T, fakeClock *clockwork.FakeClock, duration time.Duration, done func() bool) {
	t.Helper()
	fakeClock.Advance(duration)
	waitUntil(t, time.Second, done)
}

// waitUntil 轮询等待条件成立。
//
// 参数:
//   - t: Go 测试上下文。
//   - timeout: 最大等待时间。
//   - done: 返回 true 表示条件已经成立。
//
// 返回值:
//   - 无。超时时通过 t.Fatal 终止。
func waitUntil(t *testing.T, timeout time.Duration, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

// waitForSignal 等待 channel 被关闭或收到值。
//
// 参数:
//   - t: Go 测试上下文。
//   - ch: 等待的信号 channel。
//   - timeout: 最大等待时间。
//   - name: 失败时用于说明等待目标的名称。
//
// 返回值:
//   - 无。超时时通过 t.Fatalf 终止。
func waitForSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, name string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s", name)
	}
}

// updateMax 用 CAS 更新并发峰值。
//
// 参数:
//   - max: 当前峰值计数器。
//   - candidate: 新观测到的并发数。
//
// 返回值:
//   - 无。
func updateMax(max *atomic.Int64, candidate int64) {
	for {
		current := max.Load()
		if candidate <= current {
			return
		}
		if max.CompareAndSwap(current, candidate) {
			return
		}
	}
}
