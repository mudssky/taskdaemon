# HANDOFF · TD1 Desktop 平台能力边界层

> 分支：`mudssky/td1-desktop-platform-bridge`（未 merge）
> 任务：`.trellis/tasks/07-27-td1-desktop-platform-bridge/`
> 契约：**C-5 已冻结**

## 做了什么

1. **Go 注册框架** `services/taskdaemon-go/internal/desktop/bridge.go`
   - `Capability` / `Registry` / `CapabilityDescriptor` / `InvokeResult`
   - `Registry.Invoke` 外层 `recover`（panic → `DESKTOP_INVOKE_FAILED`）
   - 错误码：`DESKTOP_CAPABILITY_UNAVAILABLE` / `NOT_FOUND` / `PERMISSION_DENIED` / `INVOKE_FAILED`
2. **样板能力 Environment** `capability_environment.go` + `// TD1` 注册于 `desktop.go` `newApp`
3. **Wails binding 入口**：`App.ListCapabilities`、`App.InvokeCapability`
4. **前端边界层** `apps/web/src/lib/desktop/**`
   - `platform.ts`：环境判断唯一来源（委托 `bridge.detectWailsRuntime`）
   - `bridge.ts`：唯一读 Wails 全局对象 / `Call.ByName`
   - `capabilities.ts`：具名常量 + `CapabilityState`
   - `useDesktopCapability` + `desktopKeys`（`queryClient.ts` append）
   - `DesktopEnvironmentPanel`：设置页 append 消费点
5. **C-5 文档冻结**
   - `.trellis/spec/frontend/routing-platform-guidelines.md`
   - `.trellis/spec/backend/error-handling.md`（`DESKTOP_*`）
   - 路线图 §7.4 D1=done、§9 C-5 产物路径、§16 v5

## 双路径验证

### 浏览器（优雅降级）

1. `pnpm dev:web`
2. 登录后打开 **设置** `/settings`
3. 底部「平台能力」面板：**Desktop 能力不可用** / 原因「非 Desktop 环境」
4. 不抛错、不空白、不发 Wails 请求

**自动化**：`useDesktopCapability`「not-desktop 且零请求」；`platform.test.ts`。

### Wails 壳内（真实数据）

1. `pnpm dev:desktop`
2. 设置页显示 `platform/arch`、`osVersion`、`appVersion`、能力含 `desktop.environment`
3. 依赖 Wails v3 `window.wails.Call.ByName`（或 `_wails` + Call）

**自动化**：`TestAppListAndInvokeEnvironment`、`TestEnvironmentCapabilityFields`、Registry panic recover。

> 本 worker 未拉起 GUI 壳；协调者可用 `pnpm dev:desktop` 肉眼确认。

## 唯一性守卫

```bash
rg -n "window\\.runtime|window\\.go|wails" apps/web/src -g '*.ts' -g '*.tsx' \
  | rg -v 'lib/desktop/bridge' \
  | rg -v '\\.test\\.'
```

结果：**无业务命中**。

## 验收命令

| 命令 | 结果 |
|---|---|
| `pnpm typecheck` | 绿 |
| `pnpm lint` | 绿 |
| `pnpm --filter @taskdaemon/web test` | 10 files / 31 tests 绿 |
| `go test ./internal/desktop/...` | 绿 |
| `go vet ./internal/desktop/...` | 绿 |
| `pnpm test:go`（全量） | config 包 2 个**预存**失败（见下） |
| `pnpm vet:go` | 绿 |

### 预存失败（非本任务）

`internal/config`：

- `TestProjectPathFindsFirstProjectConfig`
- `TestProjectLocalPathFindsFirstLocalConfig`

返回绝对 temp 路径 vs 断言相对 `config.yml`；`git blame` 指向 2026-05，本任务未改 `internal/config/**`。

## 解锁下游

D2 / D3 可只读：

- `apps/web/src/lib/desktop/index.ts`
- `.trellis/spec/frontend/routing-platform-guidelines.md`「新增一个 Desktop 能力的完整步骤」
- `services/taskdaemon-go/internal/desktop/bridge.go` 的 `Capability` 接口

**不要**在 D2/D3 绕过边界层；契约不够用时回本任务改。

## 未做 / 留给协调者

- 不 merge 到 `dev`
- 三平台 Wails GUI 手测
- D2/D3 能力实现
