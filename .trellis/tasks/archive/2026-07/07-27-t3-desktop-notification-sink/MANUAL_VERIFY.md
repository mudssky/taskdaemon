# 三平台手动验证记录 · T3 Desktop 通知 sink

> 填写后归档。本 worker 未执行 GUI 手测。

## 环境矩阵

| 平台 | 系统版本 | 桌面环境 | 结果 | 备注 |
|---|---|---|---|---|
| macOS | | | ⬜ 待测 | 未签名包可能影响通知；需用户授权 |
| Windows | | | ⬜ 待测 | Toast 通常无需额外授权 |
| Linux | | | ⬜ 待测 | 依赖 `org.freedesktop.Notifications`；无守护进程则记不支持 |

## 用例

1. **浏览器降级**：`pnpm dev:web` → 设置页「系统通知」显示不可用 / 非 Desktop，无异常。
2. **Desktop 授权**：`pnpm dev:desktop` → 请求通知权限 → 状态变为已授权。
3. **测试通知**：发送测试通知 → 系统弹出标题/正文。
4. **事件投递**：触发 `task.run.failed`（或等价）→ 系统通知标题/正文与事件一致。
5. **点击激活**：点击通知 → 主窗口前置。
6. **拒绝权限（macOS）**：拒绝后不自动反复弹窗；show 返回 `DESKTOP_PERMISSION_DENIED`。
7. **配置开关**：`notify.desktop.enabled=false` 后不再弹出。

## 记录模板

```text
日期:
操作者:
平台/版本:
结果: pass / fail / unsupported
日志/截图路径:
```
