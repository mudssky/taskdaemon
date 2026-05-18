package app

import (
	"errors"
	"net"
	"strconv"
	"strings"

	"taskdaemon/internal/config"
)

// daemonClientAddress 返回 CLI 访问本机 daemon API 时使用的地址。
//
// 参数:
//   - server: server 监听配置。
//
// 返回值:
//   - string: host:port 格式客户端地址。
func daemonClientAddress(server config.ServerConfig) string {
	host := strings.TrimSpace(server.Host)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(server.Port))
}

// isAddressInUse 判断 HTTP server 启动错误是否为监听地址已被占用。
//
// 参数:
//   - err: server 启动返回的错误。
//
// 返回值:
//   - bool: true 表示错误来自地址占用。
func isAddressInUse(err error) bool {
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	if opErr.Op != "listen" {
		return false
	}
	return strings.Contains(strings.ToLower(opErr.Err.Error()), "address already in use") ||
		strings.Contains(strings.ToLower(opErr.Err.Error()), "only one usage of each socket address")
}
