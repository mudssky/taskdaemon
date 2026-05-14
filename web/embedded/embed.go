package embedded

import "embed"

// Assets 保存发布期前端构建产物。
//
// 参数:
//   - 无。
//
// 返回值:
//   - embed.FS: Go embed 文件系统，由 Wails 或 HTTP 静态资源服务使用。
//
//go:embed all:dist
var Assets embed.FS
