# TD2 / D3 Desktop 原生增量能力 — 技术设计

## 1. 问题定位

D1（C-5）已冻结 capability 边界层与 Environment 样板。本任务在 **不改注册框架** 的前提下，落地托盘、Desktop 应用自启动、单实例、窗口状态持久化。

Wails v3（`v3.0.0-alpha2.118`）已提供一等 API：

| 能力 | Wails API |
|---|---|
| 托盘 | `App.SystemTray.New()` + `Menu` |
| 自启动 | `App.Autostart`（Enable/Disable/Status） |
| 单实例 | `Options.SingleInstance`（flock，异常退出可自愈） |
| 窗口几何 | `WebviewWindow` Bounds/Size/Position + `RegisterHook(WindowClosing)` |

## 2. 与 T5 的边界（产品风险）

| | TD2 Desktop 自启动 | T5 系统服务 |
|---|---|---|
| 对象 | **Desktop GUI 应用** | **无界面 taskdaemon 服务** |
| 机制 | 登录项 / LaunchAgent / Run 键 / XDG autostart | OS service installer |
| 用户感知 | 登录后弹出/托盘应用 | 后台常驻 daemon |
| UI 文案 | 明确写「桌面应用登录时启动」；禁止写「系统服务」 | T5 CLI/文档 |

## 3. 分层与注册

```text
desktop.go Run
  ├── SingleInstance（Options，非 capability）
  ├── 恢复 window-state → NewWithOptions
  ├── 装配 tray / close→hide / 状态持久化
  └── registry.Register（append-only，// TD2）
        ├── desktop.tray
        ├── desktop.autostart
        └── desktop.window-state

apps/web/src/lib/desktop
  ├── capabilities.ts（常量已预留）
  ├── DesktopNativeFeaturesPanel.tsx（设置页消费）
  └── 仍只经 useDesktopCapability；禁止读 Wails 全局
```

**硬约束**：不改 `bridge.go` / `bridge.ts` 注册框架；能力实现内禁止 panic。

## 4. 配置（C-1 append-only）

`config.Desktop` 追加到 `Config` 尾部：

| 字段 | 默认 | 分级 | 说明 |
|---|---|---|---|
| `trayEnabled` | true | 重启生效 | 是否创建托盘 |
| `minimizeToTray` | true | 热语义（运行时 host 可变） | 关窗 → 隐藏到托盘 |
| `autostartEnabled` | false | 经 capability 写系统登录项 | 期望状态；启动时可选对齐 |
| `singleInstance` | true | 重启生效 | 是否启用单实例 |
| `windowStateEnabled` | true | 重启生效 | 是否读写窗口状态文件 |

键前缀：`desktop.*`。与配置文件分离的窗口状态文件：`$UserConfigDir/taskdaemon/window-state.json`。

## 5. 能力契约

### 5.1 `desktop.tray`

- **Available**：桌面 OS 且 tray 装配成功；否则 `platform`。
- **Invoke actions**（JSON `{"action":"..."}`）：
  - `status` → `{ serviceStatus: "running"|"stopped"|"error", minimizeToTray, label }`
  - `showWindow` → 显示并聚焦主窗口
  - `setMinimizeToTray` + `enabled: bool` → 更新运行时关窗行为
- **服务状态**：探测本机 `GET /api/health`（与 `waitForAPIReady` 同源）。

### 5.2 `desktop.autostart`

- **Available**：darwin/windows/linux；其它 `platform`。
- **Invoke**：
  - `status` → `{ enabled, strategy, path, note }`（note 含与 T5 区分文案）
  - `enable` / `disable` → 调 Wails Autostart；失败 `DESKTOP_INVOKE_FAILED` 或 `DESKTOP_PERMISSION_DENIED`

### 5.3 `desktop.window-state`

- **Available**：windowStateEnabled 且 host 已绑定窗口。
- **Invoke**：
  - `get` → 当前/持久化几何
  - `reset` → 默认 1200×760 居中并写盘

### 5.4 单实例（非 capability）

- UniqueID：`com.taskdaemon.desktop`
- 第二实例：`OnSecondInstanceLaunch` → 显示并聚焦已有窗口后退出
- 锁：Wails flock；进程被 kill 后锁释放 → 自愈（单测覆盖路径语义说明）

## 6. 托盘菜单

- 显示主窗口
- 运行状态（标签随 health 更新：运行中/已停止/异常）
- 退出（真正 Quit，绕过 minimize-to-tray）

关窗：`RegisterHook(WindowClosing)`，若 `minimizeToTray` 则 `Cancel` + `Hide`。

## 7. 窗口状态

```json
{ "x": 0, "y": 0, "width": 1200, "height": 760, "maximised": false }
```

- 启动：读文件 → 校验 → `clampToVisible`（与任一屏 WorkArea 相交至少 80px 边；否则默认居中）
- 损坏/缺文件：默认布局，不阻塞启动
- 变更：move/resize/maximise 事件防抖写盘；退出前再写一次

## 8. 前端

设置页 append `DesktopNativeFeaturesPanel`：

- 浏览器：三块 capability 均 `not-desktop` 禁用说明
- Desktop：自启动开关（文案区分 T5）、最小化到托盘开关、窗口状态重置、托盘状态只读

## 9. 测试策略

| 层 | 内容 |
|---|---|
| 纯函数 | window clamp / load-save 损坏回退 / invoke action 分发 |
| capability | mock host：Available、action 路由、错误码 |
| 装配 | `newApp` 清单含 Environment+Tray+Autostart+WindowState |
| 前端 | 常量；面板在 not-desktop 不抛错 |
| 手测 | 托盘/自启动/单实例三平台记 `research/manual-verification.md`（本 worker 以自动化为主） |

## 10. 风险

| 风险 | 处置 |
|---|---|
| headless CI 无 GUI | 托盘/窗口真实创建不进 unit；host 可注入 |
| Autostart 需权限 | 结构化错误 + UI message |
| 与 T5 文案混淆 | capability note + 面板固定说明 |
| `TestAppListAndInvokeEnvironment` 清单长度 | 同步更新为 4 |
