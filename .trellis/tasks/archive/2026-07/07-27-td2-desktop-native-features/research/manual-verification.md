# TD2 手动验证记录

本 worker 以自动化验收为主（typecheck / lint / desktop go test / web unit）。

| 项 | 状态 | 备注 |
|---|---|---|
| 托盘菜单 show / status / quit | 待协调者 `pnpm dev:desktop` | 依赖 GUI |
| 关窗最小化到托盘 | 待手测 | RegisterHook WindowClosing |
| macOS / Windows / Linux 登录项 | 待手测 | Wails Autostart |
| 第二实例激活首窗 | 待手测 | flock 自愈由 Wails 保证 |
| 窗口状态损坏回退 | **单测覆盖** | `TestLoadWindowStateCorruptFallsBack` |
| 屏外夹取 | **单测覆盖** | `TestClampWindowStateOffscreen` |
| 浏览器禁用态 | 设置页面板 empty-state | 经 capability not-desktop |

系统版本：开发机 Darwin arm64（本会话）；Windows/Linux GUI 由协调者补录。
