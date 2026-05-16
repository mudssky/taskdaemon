package desktop

import (
	"testing"

	"taskdaemon/internal/config"
)

// TestDesktopStartURLUsesFrontendDevServer 验证 Wails dev 模式下桌面窗口直连 Vite dev server。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatalf 终止。
func TestDesktopStartURLUsesFrontendDevServer(t *testing.T) {
	t.Setenv("FRONTEND_DEVSERVER_URL", "http://localhost:9245")

	got := desktopStartURL(config.Default())

	if got != "http://localhost:9245" {
		t.Fatalf("desktopStartURL = %q, want dev server URL", got)
	}
}

// TestDesktopStartURLUsesAPIOriginOutsideDev 验证非 dev 模式下桌面窗口直连后端托管的前端入口。
//
// 参数:
//   - t: Go 测试上下文。
//
// 返回值:
//   - 无。测试失败时通过 t.Fatalf 终止。
func TestDesktopStartURLUsesAPIOriginOutsideDev(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 41000

	got := desktopStartURL(cfg)

	if got != "http://127.0.0.1:41000" {
		t.Fatalf("desktopStartURL = %q, want API origin URL", got)
	}
}
