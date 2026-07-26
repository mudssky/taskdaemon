package notify

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
)

// recordingEmailSender 记录发送调用，不触网。
type recordingEmailSender struct {
	mu       sync.Mutex
	messages []emailMessage
	fail     atomic.Int32
	calls    atomic.Int32
}

// Send 记录邮件并按 fail 计数模拟失败。
//
// 参数:
//   - ctx: 上下文。
//   - cfg: SMTP 配置（测试中不使用密码）。
//   - msg: 邮件内容。
//
// 返回值:
//   - error: 前 N 次调用返回错误。
func (sender *recordingEmailSender) Send(_ context.Context, cfg config.NotifySMTPConfig, msg emailMessage) error {
	_ = cfg
	n := sender.calls.Add(1)
	if int(n) <= int(sender.fail.Load()) {
		return fmt.Errorf("simulated smtp failure")
	}
	sender.mu.Lock()
	sender.messages = append(sender.messages, msg)
	sender.mu.Unlock()
	return nil
}

// snapshot 返回已发送邮件副本。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []emailMessage: 邮件列表。
func (sender *recordingEmailSender) snapshot() []emailMessage {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	return append([]emailMessage(nil), sender.messages...)
}

// TestEmailSinkRendersReadableBody 验证邮件主题正文可读且非裸 JSON。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestEmailSinkRendersReadableBody(t *testing.T) {
	sender := &recordingEmailSender{}
	cfg := config.NotifyEmailConfig{
		Enabled:        true,
		From:           "daemon@example.com",
		To:             []string{"ops@example.com"},
		TimeoutSeconds: 5,
		MaxRetries:     0,
		SMTP: config.NotifySMTPConfig{
			Host:       "smtp.example.com",
			Port:       587,
			Username:   "user",
			Password:   "smtp-password-secret",
			Encryption: "starttls",
		},
	}
	sink := NewEmailSink(cfg, EmailSinkOptions{Sender: sender})
	event := testEvent("mail-1")
	event.Title = "Task run failed"
	event.Body = "exit non-zero"
	event.Detail = map[string]any{"taskId": 3, "runId": 9}

	require.NoError(t, sink.Deliver(context.Background(), event))
	messages := sender.snapshot()
	require.Len(t, messages, 1)
	require.Equal(t, "daemon@example.com", messages[0].From)
	require.Equal(t, []string{"ops@example.com"}, messages[0].To)
	require.Contains(t, messages[0].Subject, "Task run failed")
	require.Contains(t, messages[0].Subject, string(SeverityInfo))
	require.Contains(t, messages[0].Body, "Taskdaemon notification")
	require.Contains(t, messages[0].Body, "Task run failed")
	require.Contains(t, messages[0].Body, "taskId")
	require.False(t, strings.HasPrefix(strings.TrimSpace(messages[0].Body), "{"))
	require.Equal(t, int64(1), sink.DeliveryStatus().SuccessCount)
}

// TestEmailSinkFiltersBySeverity 验证按严重级别过滤。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestEmailSinkFiltersBySeverity(t *testing.T) {
	sender := &recordingEmailSender{}
	cfg := config.NotifyEmailConfig{
		Enabled:     true,
		MinSeverity: string(SeverityError),
		From:        "daemon@example.com",
		To:          []string{"ops@example.com"},
		MaxRetries:  0,
		SMTP:        config.NotifySMTPConfig{Host: "smtp.example.com"},
	}
	sink := NewEmailSink(cfg, EmailSinkOptions{Sender: sender})

	infoEvent, err := NewEventWithID("info-1", NameSchedulerStarted, SeverityInfo, time.Now().UTC(), Subject{Kind: string(SubjectKindScheduler)}, "started", "", nil)
	require.NoError(t, err)
	require.NoError(t, sink.Deliver(context.Background(), infoEvent))
	require.Empty(t, sender.snapshot())

	errEvent, err := NewEventWithID("err-1", NameTaskRunFailed, SeverityError, time.Now().UTC(), Subject{Kind: string(SubjectKindRun), ID: "9"}, "failed", "exit 1", map[string]any{"taskId": 1})
	require.NoError(t, err)
	require.NoError(t, sink.Deliver(context.Background(), errEvent))
}

// TestEmailSinkRetries 验证邮件重试。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestEmailSinkRetries(t *testing.T) {
	sender := &recordingEmailSender{}
	sender.fail.Store(2)
	var sleeps atomic.Int32
	cfg := config.NotifyEmailConfig{
		Enabled:    true,
		From:       "daemon@example.com",
		To:         []string{"ops@example.com"},
		MaxRetries: 2,
		BackoffMs:  1,
		SMTP:       config.NotifySMTPConfig{Host: "smtp.example.com"},
	}
	sink := NewEmailSink(cfg, EmailSinkOptions{
		Sender: sender,
		Sleep: func(d time.Duration) {
			require.Greater(t, d, time.Duration(0))
			sleeps.Add(1)
		},
	})
	require.NoError(t, sink.Deliver(context.Background(), testEvent("retry-mail")))
	require.Equal(t, int32(3), sender.calls.Load())
	require.Equal(t, int32(2), sleeps.Load())
	require.Len(t, sender.snapshot(), 1)
}

// TestEmailSinkFailureDoesNotLeakPassword 验证失败错误不含 SMTP 密码。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestEmailSinkFailureDoesNotLeakPassword(t *testing.T) {
	sender := &recordingEmailSender{}
	sender.fail.Store(10)
	password := "super-smtp-password"
	cfg := config.NotifyEmailConfig{
		Enabled:    true,
		From:       "daemon@example.com",
		To:         []string{"ops@example.com"},
		MaxRetries: 0,
		SMTP: config.NotifySMTPConfig{
			Host:     "smtp.example.com",
			Password: password,
		},
	}
	sink := NewEmailSink(cfg, EmailSinkOptions{Sender: sender})
	err := sink.Deliver(context.Background(), testEvent("fail-mail"))
	require.Error(t, err)
	require.Contains(t, err.Error(), CodeEmailDeliverFailed)
	require.NotContains(t, err.Error(), password)
	require.NotContains(t, sink.DeliveryStatus().LastError, password)
}

// TestBusOutboundSinkIsolation 验证 webhook 失败不影响 email（及反之）。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。
func TestBusOutboundSinkIsolation(t *testing.T) {
	mailSender := &recordingEmailSender{}
	emailSink := NewEmailSink(config.NotifyEmailConfig{
		Enabled:    true,
		From:       "daemon@example.com",
		To:         []string{"ops@example.com"},
		MaxRetries: 0,
		SMTP:       config.NotifySMTPConfig{Host: "smtp.example.com"},
	}, EmailSinkOptions{Sender: mailSender})

	// webhook 指向无效私网 URL，必失败
	webhookSink := NewWebhookSink(config.NotifyWebhookConfig{
		Enabled:              true,
		AllowedSchemes:       []string{"http"},
		AllowPrivateNetworks: false,
		DefaultMaxRetries:    0,
		Targets: []config.NotifyWebhookTargetConfig{
			{Name: "bad", URL: "http://127.0.0.1:1/hook", Enabled: true},
		},
	}, WebhookSinkOptions{})

	bus := NewBus(config.NotifyConfig{BufferSize: 8}, BusOptions{})
	bus.Register(webhookSink)
	bus.Register(emailSink)
	bus.Start()
	t.Cleanup(bus.Shutdown)

	bus.Publish(context.Background(), testEvent("iso-1"))
	require.Eventually(t, func() bool {
		return len(mailSender.snapshot()) == 1
	}, 2*time.Second, 20*time.Millisecond)

	statuses := bus.SinkStatuses()
	require.Len(t, statuses, 2)
	var webhookStatus, emailStatus SinkStatus
	for _, status := range statuses {
		switch status.Name {
		case WebhookSinkName:
			webhookStatus = status
		case EmailSinkName:
			emailStatus = status
		}
	}
	require.GreaterOrEqual(t, webhookStatus.FailureCount, int64(1))
	require.GreaterOrEqual(t, emailStatus.SuccessCount, int64(1))
}
