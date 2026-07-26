package notify

import (
	"context"
	"time"
)

// retryPolicy 描述出站投递的重试与退避策略。
// T4：Webhook 与邮件共用，避免两套退避实现。
type retryPolicy struct {
	// maxRetries 是首次失败后的最大重试次数；总尝试次数 = maxRetries + 1。
	maxRetries int
	// initialBackoff 是首次重试前的等待；之后指数退避。
	initialBackoff time.Duration
	// sleep 可注入假时钟；nil 时使用可取消 timer。
	sleep func(time.Duration)
}

// doWithRetry 按退避策略执行 op，直到成功、次数用尽或 ctx 取消。
//
// 参数:
//   - ctx: 控制取消的上下文。
//   - policy: 重试策略。
//   - op: 单次尝试；返回 nil 表示成功。
//
// 返回值:
//   - error: 最后一次失败错误；成功时返回 nil。
func doWithRetry(ctx context.Context, policy retryPolicy, op func(attempt int) error) error {
	if policy.maxRetries < 0 {
		policy.maxRetries = 0
	}
	if policy.initialBackoff <= 0 {
		policy.initialBackoff = 200 * time.Millisecond
	}

	var lastErr error
	totalAttempts := policy.maxRetries + 1
	for attempt := range totalAttempts {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = op(attempt)
		if lastErr == nil {
			return nil
		}
		if attempt == totalAttempts-1 {
			break
		}
		backoff := policy.initialBackoff << attempt
		const maxBackoff = 30 * time.Second
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		if err := waitBackoff(ctx, policy.sleep, backoff); err != nil {
			return err
		}
	}
	return lastErr
}

// waitBackoff 等待退避间隔，支持取消与测试注入。
//
// 参数:
//   - ctx: 控制取消。
//   - sleep: 可选注入；非 nil 时直接调用（测试用）。
//   - backoff: 等待时长。
//
// 返回值:
//   - error: ctx 取消时返回错误。
func waitBackoff(ctx context.Context, sleep func(time.Duration), backoff time.Duration) error {
	if sleep != nil {
		// 测试路径：同步调用注入函数，再检查 ctx。
		done := make(chan struct{})
		go func() {
			sleep(backoff)
			close(done)
		}()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return nil
		}
	}
	timer := time.NewTimer(backoff)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// resolveRetry 合并目标级与默认重试参数。
//
// 参数:
//   - maxRetries: 目标级重试次数；负值时使用默认。
//   - backoffMs: 目标级初始退避毫秒；<=0 时使用默认。
//   - defaultMaxRetries: 全局默认重试次数。
//   - defaultBackoffMs: 全局默认退避毫秒。
//
// 返回值:
//   - retryPolicy: 解析后的策略（不含 sleep）。
func resolveRetry(maxRetries int, backoffMs int, defaultMaxRetries int, defaultBackoffMs int) retryPolicy {
	// 目标级 0 表示继承默认（与 timeoutSeconds 一致）；全局默认仍可为 0 表示不重试。
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	if backoffMs <= 0 {
		backoffMs = defaultBackoffMs
	}
	if backoffMs <= 0 {
		backoffMs = 200
	}
	return retryPolicy{
		maxRetries:     maxRetries,
		initialBackoff: time.Duration(backoffMs) * time.Millisecond,
	}
}
