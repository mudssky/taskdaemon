package cli

import (
	"context"
	"os"
)

type sessionTokenContextKey struct{}

// SessionTokenFromContext 读取 CLI 传入的管理员 session token。
//
// 参数:
//   - ctx: CLI 命令 context。
//
// 返回值:
//   - string: 管理员 session token，未配置时为空。
func SessionTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(sessionTokenContextKey{}).(string)
	if token != "" {
		return token
	}
	return os.Getenv("TASKDAEMON_SESSION_TOKEN")
}

// withSessionToken 将命令行 session token 放入 context。
//
// 参数:
//   - ctx: CLI 命令 context。
//   - token: flag 传入的 session token。
//
// 返回值:
//   - context.Context: 携带 token 的 context。
func withSessionToken(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionTokenContextKey{}, token)
}
