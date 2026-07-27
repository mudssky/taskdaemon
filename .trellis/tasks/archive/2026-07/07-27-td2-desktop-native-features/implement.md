# TD2 / D3 — 实施清单

## 阶段 0：激活

- [x] design.md 已写
- [ ] `python3 ./.trellis/scripts/task.py start 07-27-td2-desktop-native-features`

## 阶段 1：配置 append-only

- [ ] `types.go` 追加 `Desktop DesktopConfig` + 结构体注释分级
- [ ] `defaults.go` Default + defaultMap
- [ ] `load.go` 读取 `desktop.*`

## 阶段 2：窗口状态纯逻辑

- [ ] `window_state.go`：路径、load/save、clamp、默认值
- [ ] `window_state_test.go`：损坏回退、不可见区域夹取

## 阶段 3：单实例 + Run 装配

- [ ] `single_instance.go`：UniqueID 常量
- [ ] `desktop.go`：SingleInstance Options、恢复几何建窗、关窗 hook、退出前存盘

## 阶段 4：托盘

- [ ] `tray.go`：host、菜单、health 状态、show/hide
- [ ] `capability_tray.go` + 测试

## 阶段 5：自启动

- [ ] `autostart.go` host 包装 Wails Autostart
- [ ] `capability_autostart.go` + 测试 + T5 区分 note

## 阶段 6：window-state capability

- [ ] `capability_window_state.go` + 测试
- [ ] `newApp` append Register 三行 `// TD2`

## 阶段 7：前端

- [ ] capabilities 类型与注释
- [ ] `DesktopNativeFeaturesPanel.tsx`
- [ ] settings 路由 mount
- [ ] 常量/面板测试

## 阶段 8：验收

```bash
pnpm typecheck
pnpm lint
cd services/taskdaemon-go && go test ./internal/desktop/...
```

- [ ] HANDOFF.md
- [ ] worker_done（不 merge）
