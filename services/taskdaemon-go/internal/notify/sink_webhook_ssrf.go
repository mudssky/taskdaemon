package notify

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"taskdaemon/internal/config"
)

// validateWebhookURL 校验 Webhook 目标 URL 的协议、Host 与私有网络策略。
// 策略模型对齐 T0 音频 inbound URL（默认 https、拒绝私网、可放行）。
//
// 参数:
//   - rawURL: 配置中的目标 URL。
//   - cfg: Webhook 安全策略配置。
//
// 返回值:
//   - *url.URL: 解析后的 URL。
//   - error: 不符合策略时返回 NOTIFY_WEBHOOK_URL_REJECTED。
func validateWebhookURL(rawURL string, cfg config.NotifyWebhookConfig) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("%s: empty url", CodeWebhookURLRejected)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("%s: invalid url", CodeWebhookURLRejected)
	}
	schemes := cfg.AllowedSchemes
	if len(schemes) == 0 {
		schemes = []string{"https"}
	}
	if !containsFold(schemes, parsed.Scheme) {
		return nil, fmt.Errorf("%s: scheme %q", CodeWebhookURLRejected, parsed.Scheme)
	}
	host := strings.ToLower(parsed.Hostname())
	if len(cfg.AllowedHosts) > 0 && !containsFold(cfg.AllowedHosts, host) {
		return nil, fmt.Errorf("%s: host not allowed", CodeWebhookURLRejected)
	}
	if !cfg.AllowPrivateNetworks && isPrivateHost(host) {
		return nil, fmt.Errorf("%s: private network", CodeWebhookURLRejected)
	}
	if !cfg.AllowPrivateNetworks && resolvesToPrivateHost(host) {
		return nil, fmt.Errorf("%s: private network", CodeWebhookURLRejected)
	}
	return parsed, nil
}

// containsFold 判断字符串切片是否包含目标（大小写不敏感）。
//
// 参数:
//   - values: 候选值。
//   - target: 目标值。
//
// 返回值:
//   - bool: 包含时返回 true。
func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

// isPrivateHost 判断 Host 是否属于本机或私有网络。
//
// 参数:
//   - host: URL hostname。
//
// 返回值:
//   - bool: 属于本机、回环、链路本地或私有地址时返回 true。
func isPrivateHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// resolvesToPrivateHost 判断域名解析结果是否包含私有地址。
//
// 参数:
//   - host: URL hostname。
//
// 返回值:
//   - bool: 解析到本机、回环、链路本地或私有地址时返回 true。
func resolvesToPrivateHost(host string) bool {
	if net.ParseIP(host) != nil {
		return false
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		// 解析失败时保守拒绝，避免 SSRF 绕过。
		return true
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
	}
	return false
}
