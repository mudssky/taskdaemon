# HANDOFF · TD2 Desktop 原生增量能力

> 分支：`mudssky/td2-desktop-native-features`（**未 merge**）  
> 任务：`.trellis/tasks/07-27-td2-desktop-native-features/`  
> 依赖：C-5 / TD1 边界层（本 worktree 已含）

## 做了什么

### 配置（append-only）

- `internal/config`：`DesktopConfig`（trayEnabled / minimizeToTray / autostartEnabled / singleInstance / windowStateEnabled）
- defaults / load / env 别名 `TASKDAEMON_DESKTOP_*`

### Go Desktop 运行时

| 文件 | 职责 |
|---|---|
| `single_instance.go` | UniqueID `com.taskdaemon.desktop` |
| `tray.go` + `capability_tray.go` | 托盘菜单、health 三态、show/minimize 开关 |
| `autostart.go` + `capability_autostart.go` | 登录项 enable/disable/status；**文案区分 T5** |
| `window_state.go` + `capability_window_state.go` | 用户目录 JSON、损坏回退、屏外夹取、get/reset |
| `desktop.go` | SingleInstance Options、恢复几何建窗、关窗→托盘 hook、Register `// TD2` |

### 前端（仅边界层）

- `capabilities.ts`：Tray/Autostart/WindowState 类型
- `DesktopNativeFeaturesPanel.tsx`：设置页开关与只读状态
- `router.tsx` 设置页 append 挂载
- **未**绕过 `bridge.ts`

### 与 T5 边界

自启动 note 固定：

> 此项仅控制 Desktop 图形应用在用户登录时启动，不是 taskdaemon 系统服务（T5）…

设置面板标题区同样写明。

## 验收

| 命令 | 结果 |
|---|---|
| `pnpm typecheck` | 绿 |
| `pnpm lint` | 绿 |
| `go test ./internal/desktop/...` | 绿 |
| `pnpm --filter @taskdaemon/web test` | 14 files / 50 tests 绿 |

### 唯一性守卫

```bash
rg -n "window\\.runtime|window\\.go|wails" apps/web/src -g '*.ts' -g '*.tsx' \
  | rg -v 'lib/desktop/bridge' \
  | rg -v '\\.test\\.'
```

业务代码无命中（仅 `platform.test.ts`）。

## 未做 / 协调者

- **未 merge** 到基线
- 三平台 GUI 手测（托盘点击、登录项、第二实例激活）需真机；逻辑层已单测
- 托盘图标三态图标切换（当前同一 icon + 文案三态）
- 显示器列表在启动时未注入 Wails Screen（夹取在无 displays 时回默认；运行时 move/resize 写盘；屏外夹取单测覆盖）

## 解锁

D2 通知 sink 仍独立；本任务不改 `bridge.go` 框架。
