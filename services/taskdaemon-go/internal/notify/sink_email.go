package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"taskdaemon/internal/config"
)

// EmailSinkName 是邮件出站 sink 的稳定名称。
const EmailSinkName = "email"

// emailMessage 描述一封待发送邮件（不含 SMTP 凭据）。
type emailMessage struct {
	From    string
	To      []string
	Subject string
	Body    string
}

// emailSender 抽象 SMTP 发送，便于单元测试注入。
type emailSender interface {
	Send(ctx context.Context, cfg config.NotifySMTPConfig, msg emailMessage) error
}

// smtpEmailSender 使用 net/smtp 发送邮件。
type smtpEmailSender struct{}

// EmailSink 将 C-2 事件渲染为可读邮件并经 SMTP 投递。
type EmailSink struct {
	logger *slog.Logger
	sender emailSender
	sleep  func(time.Duration)
	now    func() time.Time

	mu     sync.RWMutex
	cfg    config.NotifyEmailConfig
	status emailRuntimeStatus
}

// emailRuntimeStatus 记录邮件 sink 内存投递状态（重启清零）。
type emailRuntimeStatus struct {
	LastSuccessAt   *time.Time `json:"lastSuccessAt,omitempty"`
	LastFailureAt   *time.Time `json:"lastFailureAt,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
	SuccessCount    int64      `json:"successCount"`
	FailureCount    int64      `json:"failureCount"`
	LastDeliveredAt *time.Time `json:"lastDeliveredAt,omitempty"`
}

// EmailSinkOptions 控制邮件 sink 可选依赖。
type EmailSinkOptions struct {
	Logger *slog.Logger
	Sender emailSender
	Sleep  func(time.Duration)
	Now    func() time.Time
}

// NewEmailSink 创建邮件出站 sink。
//
// 参数:
//   - cfg: 邮件配置。
//   - opts: 可选 logger / sender / 测试时钟。
//
// 返回值:
//   - *EmailSink: sink 实例。
func NewEmailSink(cfg config.NotifyEmailConfig, opts EmailSinkOptions) *EmailSink {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	sender := opts.Sender
	if sender == nil {
		sender = smtpEmailSender{}
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &EmailSink{
		logger: logger,
		sender: sender,
		sleep:  opts.Sleep,
		now:    now,
		cfg:    cfg,
	}
}

// Name 返回 sink 名称。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: 固定为 "email"。
func (sink *EmailSink) Name() string {
	return EmailSinkName
}

// Enabled 返回邮件 sink 是否启用。
//
// 参数:
//   - 无。
//
// 返回值:
//   - bool: 启用时返回 true。
func (sink *EmailSink) Enabled() bool {
	if sink == nil {
		return false
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	return sink.cfg.Enabled
}

// UpdateConfig 热更新邮件配置段。
//
// 参数:
//   - cfg: 完整通知配置。
//
// 返回值:
//   - 无。
func (sink *EmailSink) UpdateConfig(cfg config.NotifyConfig) {
	if sink == nil {
		return
	}
	sink.mu.Lock()
	sink.cfg = cfg.Email
	sink.mu.Unlock()
}

// emailConfig 返回当前邮件配置快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - config.NotifyEmailConfig: 当前配置。
func (sink *EmailSink) emailConfig() config.NotifyEmailConfig {
	if sink == nil {
		return config.Default().Notify.Email
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	return sink.cfg
}

// DeliveryStatus 返回邮件 sink 投递状态快照。
//
// 参数:
//   - 无。
//
// 返回值:
//   - emailRuntimeStatus: 状态快照。
func (sink *EmailSink) DeliveryStatus() emailRuntimeStatus {
	if sink == nil {
		return emailRuntimeStatus{}
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	return sink.status
}

// Deliver 按严重级别过滤后渲染并发送邮件。
//
// 参数:
//   - ctx: 请求上下文。
//   - event: C-2 事件。
//
// 返回值:
//   - error: 投递失败时返回 NOTIFY_EMAIL_DELIVER_FAILED。
func (sink *EmailSink) Deliver(ctx context.Context, event Event) error {
	if sink == nil {
		return fmt.Errorf("%s: email sink is nil", CodeEmailDeliverFailed)
	}
	cfg := sink.emailConfig()
	if !cfg.Enabled {
		return nil
	}
	if !severityMeetsMinimum(event.Severity, cfg.MinSeverity) {
		return nil
	}
	if strings.TrimSpace(cfg.SMTP.Host) == "" {
		err := fmt.Errorf("%s: smtp host is empty", CodeEmailDeliverFailed)
		sink.recordFailure(err.Error())
		return err
	}
	if strings.TrimSpace(cfg.From) == "" || len(cfg.To) == 0 {
		err := fmt.Errorf("%s: from/to is required", CodeEmailDeliverFailed)
		sink.recordFailure(err.Error())
		return err
	}

	msg := emailMessage{
		From:    cfg.From,
		To:      append([]string(nil), cfg.To...),
		Subject: renderEmailSubject(event),
		Body:    renderEmailBody(event),
	}
	policy := resolveRetry(cfg.MaxRetries, cfg.BackoffMs, cfg.MaxRetries, cfg.BackoffMs)
	// MaxRetries 在 email 上直接使用配置值（无 target 覆盖）。
	if cfg.MaxRetries < 0 {
		policy.maxRetries = 0
	} else {
		policy.maxRetries = cfg.MaxRetries
	}
	if cfg.BackoffMs <= 0 {
		policy.initialBackoff = 200 * time.Millisecond
	} else {
		policy.initialBackoff = time.Duration(cfg.BackoffMs) * time.Millisecond
	}
	policy.sleep = sink.sleep

	// 将超时并入 ctx，避免阻塞。
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := doWithRetry(sendCtx, policy, func(attempt int) error {
		if err := sink.sender.Send(sendCtx, cfg.SMTP, msg); err != nil {
			// 脱敏：不把 password 放进错误链。
			return fmt.Errorf("%s: smtp send failed (attempt %d): %s", CodeEmailDeliverFailed, attempt+1, sanitizeSMTPError(err))
		}
		return nil
	})
	if err != nil {
		sink.recordFailure(err.Error())
		sink.logger.Error("notify email deliver failed",
			"smtp_host", cfg.SMTP.Host,
			"smtp_port", cfg.SMTP.Port,
			"error", sanitizeError(err.Error()),
		)
		return err
	}
	sink.recordSuccess()
	return nil
}

// recordSuccess 更新成功状态。
//
// 参数:
//   - 无。
//
// 返回值:
//   - 无。
func (sink *EmailSink) recordSuccess() {
	now := sink.now()
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.status.SuccessCount++
	sink.status.LastSuccessAt = &now
	sink.status.LastDeliveredAt = &now
	sink.status.LastError = ""
}

// recordFailure 更新失败状态。
//
// 参数:
//   - message: 失败原因。
//
// 返回值:
//   - 无。
func (sink *EmailSink) recordFailure(message string) {
	now := sink.now()
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.status.FailureCount++
	sink.status.LastFailureAt = &now
	sink.status.LastDeliveredAt = &now
	sink.status.LastError = sanitizeError(message)
}

// renderEmailSubject 生成可读邮件主题。
//
// 参数:
//   - event: 事件。
//
// 返回值:
//   - string: 主题行。
func renderEmailSubject(event Event) string {
	title := strings.TrimSpace(event.Title)
	if title == "" {
		title = string(event.Name)
	}
	return fmt.Sprintf("[taskdaemon/%s] %s", event.Severity, title)
}

// renderEmailBody 生成可读纯文本正文（非裸 JSON）。
//
// 参数:
//   - event: 事件。
//
// 返回值:
//   - string: 正文。
func renderEmailBody(event Event) string {
	var b strings.Builder
	b.WriteString("Taskdaemon notification\n")
	b.WriteString("=======================\n\n")
	fmt.Fprintf(&b, "Event: %s\n", event.Name)
	fmt.Fprintf(&b, "Severity: %s\n", event.Severity)
	fmt.Fprintf(&b, "OccurredAt: %s\n", event.OccurredAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "ID: %s\n", event.ID)
	if event.Subject.Kind != "" || event.Subject.ID != "" {
		fmt.Fprintf(&b, "Subject: %s %s\n", event.Subject.Kind, event.Subject.ID)
	}
	b.WriteString("\n")
	if event.Title != "" {
		fmt.Fprintf(&b, "Title: %s\n", event.Title)
	}
	if event.Body != "" {
		fmt.Fprintf(&b, "Body: %s\n", event.Body)
	}
	if detail := filterDetail(event.Detail); len(detail) > 0 {
		b.WriteString("\nDetail:\n")
		for key, value := range detail {
			fmt.Fprintf(&b, "  - %s: %v\n", key, value)
		}
	}
	return b.String()
}

// sanitizeSMTPError 将 SMTP 错误转为可记录摘要，剥离凭据。
//
// 参数:
//   - err: 原始错误。
//
// 返回值:
//   - string: 脱敏摘要。
func sanitizeSMTPError(err error) string {
	if err == nil {
		return ""
	}
	return sanitizeError(err.Error())
}

// Send 通过 SMTP 发送一封邮件。
//
// 参数:
//   - ctx: 请求上下文（用于超时取消；smtp 库本身不感知 ctx，靠外层超时）。
//   - cfg: SMTP 配置。
//   - msg: 邮件内容。
//
// 返回值:
//   - error: 发送失败时返回错误。
func (smtpEmailSender) Send(ctx context.Context, cfg config.NotifySMTPConfig, msg emailMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", effectiveSMTPPort(cfg)))
	raw := buildSMTPMessage(msg)

	encryption := strings.ToLower(strings.TrimSpace(cfg.Encryption))
	if encryption == "" {
		encryption = "starttls"
	}

	switch encryption {
	case "none":
		return sendSMTPPlain(addr, cfg, msg.From, msg.To, raw)
	case "tls":
		return sendSMTPTLS(addr, cfg, msg.From, msg.To, raw)
	case "starttls":
		return sendSMTPStartTLS(addr, cfg, msg.From, msg.To, raw)
	default:
		return fmt.Errorf("unsupported smtp encryption %q", encryption)
	}
}

// effectiveSMTPPort 返回 SMTP 端口，0 时按加密方式给默认值。
//
// 参数:
//   - cfg: SMTP 配置。
//
// 返回值:
//   - int: 端口。
func effectiveSMTPPort(cfg config.NotifySMTPConfig) int {
	if cfg.Port > 0 {
		return cfg.Port
	}
	if strings.EqualFold(cfg.Encryption, "tls") {
		return 465
	}
	return 587
}

// buildSMTPMessage 组装 RFC 822 风格消息。
//
// 参数:
//   - msg: 邮件内容。
//
// 返回值:
//   - []byte: 原始消息。
func buildSMTPMessage(msg emailMessage) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", msg.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(msg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", sanitizeHeader(msg.Subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(msg.Body)
	if !strings.HasSuffix(msg.Body, "\n") {
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

// sanitizeHeader 去掉主题中的换行，防止 header 注入。
//
// 参数:
//   - value: 原始 header 值。
//
// 返回值:
//   - string: 安全 header 值。
func sanitizeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

// sendSMTPPlain 明文 SMTP 发送。
//
// 参数:
//   - addr: host:port。
//   - cfg: SMTP 配置。
//   - from: 发件人。
//   - to: 收件人。
//   - raw: 消息体。
//
// 返回值:
//   - error: 发送失败时返回错误。
func sendSMTPPlain(addr string, cfg config.NotifySMTPConfig, from string, to []string, raw []byte) error {
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return smtp.SendMail(addr, auth, from, to, raw)
}

// sendSMTPTLS 隐式 TLS SMTP 发送。
//
// 参数:
//   - addr: host:port。
//   - cfg: SMTP 配置。
//   - from: 发件人。
//   - to: 收件人。
//   - raw: 消息体。
//
// 返回值:
//   - error: 发送失败时返回错误。
func sendSMTPTLS(addr string, cfg config.NotifySMTPConfig, from string, to []string, raw []byte) error {
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	return smtpClientSend(client, cfg, from, to, raw)
}

// sendSMTPStartTLS STARTTLS SMTP 发送。
//
// 参数:
//   - addr: host:port。
//   - cfg: SMTP 配置。
//   - from: 发件人。
//   - to: 收件人。
//   - raw: 消息体。
//
// 返回值:
//   - error: 发送失败时返回错误。
func sendSMTPStartTLS(addr string, cfg config.NotifySMTPConfig, from string, to []string, raw []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	return smtpClientSend(client, cfg, from, to, raw)
}

// smtpClientSend 在已建立的 SMTP client 上完成认证与投递。
//
// 参数:
//   - client: SMTP 客户端。
//   - cfg: SMTP 配置。
//   - from: 发件人。
//   - to: 收件人。
//   - raw: 消息体。
//
// 返回值:
//   - error: 发送失败时返回错误。
func smtpClientSend(client *smtp.Client, cfg config.NotifySMTPConfig, from string, to []string, raw []byte) error {
	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
