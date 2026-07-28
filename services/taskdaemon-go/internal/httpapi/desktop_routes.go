package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// DesktopCapabilityDTO 是 Desktop 能力清单条目（HTTP 线格式）。
type DesktopCapabilityDTO struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

// DesktopInvokeResultDTO 是 Desktop capability 调用结果（HTTP 线格式）。
type DesktopInvokeResultDTO struct {
	OK      bool            `json:"ok"`
	Data    json.RawMessage `json:"data,omitempty"`
	Code    string          `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
}

// DesktopBridge 是 Desktop 能力的 HTTP 适配接口。
// 由 desktop 壳在同进程内注入；纯 Web / serve-only 模式为 nil。
type DesktopBridge interface {
	ListDesktopCapabilities() []DesktopCapabilityDTO
	InvokeDesktopCapability(name string, payloadJSON string) DesktopInvokeResultDTO
}

var (
	desktopBridgeMu sync.RWMutex
	desktopBridge   DesktopBridge
)

// SetDesktopBridge 注入或清空同进程 Desktop 能力桥。
//
// 参数:
//   - bridge: Desktop 实现；传 nil 表示卸载（桌面壳退出时）。
//
// 返回值:
//   - 无。
func SetDesktopBridge(bridge DesktopBridge) {
	desktopBridgeMu.Lock()
	defer desktopBridgeMu.Unlock()
	desktopBridge = bridge
}

// getDesktopBridge 返回当前注入的 Desktop 桥（可能为 nil）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - DesktopBridge: 已注入实现或 nil。
func getDesktopBridge() DesktopBridge {
	desktopBridgeMu.RLock()
	defer desktopBridgeMu.RUnlock()
	return desktopBridge
}

// registerDesktopRoutes 注册 Desktop 能力 HTTP 入口。
// 业务 API 走本机 HTTP，避免 wails.localhost AssetServer 转发 POST body 丢失。
//
// 参数:
//   - router: gin 引擎。
//   - authService: 会话认证；nil 时 requireSession 返回 401。
//
// 返回值:
//   - 无。
func registerDesktopRoutes(router *gin.Engine, authService AuthService) {
	group := router.Group("/api/desktop")
	// AuthService 为 nil 时 requireSession 也会 401，避免桌面能力在无会话时暴露。
	group.Use(requireSession(authService))

	group.GET("/capabilities", func(ctx *gin.Context) {
		bridge := getDesktopBridge()
		if bridge == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "DESKTOP_BRIDGE_UNAVAILABLE", "Desktop bridge is not running in this process", nil)
			return
		}
		items := bridge.ListDesktopCapabilities()
		if items == nil {
			items = []DesktopCapabilityDTO{}
		}
		writeAPIOK(ctx, items)
	})

	group.POST("/invoke", func(ctx *gin.Context) {
		bridge := getDesktopBridge()
		if bridge == nil {
			writeAPIError(ctx, http.StatusServiceUnavailable, "DESKTOP_BRIDGE_UNAVAILABLE", "Desktop bridge is not running in this process", nil)
			return
		}
		var req struct {
			Name        string `json:"name"`
			PayloadJSON string `json:"payloadJSON"`
		}
		if err := ctx.ShouldBindJSON(&req); err != nil {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "Invalid request body", gin.H{"field": "body"})
			return
		}
		if req.Name == "" {
			writeAPIError(ctx, http.StatusBadRequest, "bad_request", "name is required", gin.H{"field": "name"})
			return
		}
		writeAPIOK(ctx, bridge.InvokeDesktopCapability(req.Name, req.PayloadJSON))
	})
}
