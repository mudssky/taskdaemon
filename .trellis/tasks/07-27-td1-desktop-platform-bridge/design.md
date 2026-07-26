# TD1 Desktop 平台能力边界层 — 技术设计

## 1. 问题定位

`.trellis/spec/frontend/routing-platform-guidelines.md` 已规定 Desktop 能力必须经 `src/lib/desktop` 封装，但**该目录不存在**（当前 `apps/web/src/lib/` 只有 `api/`、`dayjs.ts`、`utils.ts`）。

规范存在而实现缺失的后果：D2（通知）、D3（托盘等）会各自发明接入方式，最终形成三套不兼容的写法。本任务是这条规范的**唯一实现载体**。

## 2. 分层

```text
业务组件（features/*）
  │ 只 import 这一层
  ▼
src/lib/desktop/index.ts          ← 对外唯一出口
  ├── capabilities.ts             ← capability 常量与类型
  ├── useDesktopCapability.ts     ← React hook
  ├── platform.ts                 ← 环境判断的唯一来源
  └── bridge.ts                   ← Wails 调用封装（唯一接触全局对象的文件）
       │
       ▼
internal/desktop/bridge*.go       ← Go 侧注册框架（本任务）
  └── internal/desktop/*.go       ← 各能力实现（D2/D3 追加）
```

**硬约束**：`bridge.ts` 是**全仓库唯一**允许读取 Wails 全局对象的文件。通过 lint 规则或 code review 守住。

## 3. Capability 模型（C-5 核心）

### 3.1 具名标识

```ts
export const DesktopCapability = {
  Environment: 'desktop.environment',   // 本任务的样板能力
  Notification: 'desktop.notification', // D2
  Tray: 'desktop.tray',                 // D3
  Autostart: 'desktop.autostart',       // D3
  WindowState: 'desktop.window-state',  // D3
} as const

export type DesktopCapabilityName =
  typeof DesktopCapability[keyof typeof DesktopCapability]
```

下游任务**只追加常量**，不改结构。非法名称在类型层被拒绝。

### 3.2 统一返回形状

```ts
type CapabilityState<TResult> =
  | { status: 'available'; invoke: (input?: unknown) => Promise<TResult> }
  | { status: 'unavailable'; reason: UnavailableReason; message: string }
  | { status: 'checking' }

type UnavailableReason =
  | 'not-desktop'        // 浏览器环境
  | 'platform'           // Desktop 但当前 OS 不支持
  | 'permission'         // 支持但权限未授予（D2 需要）
  | 'error'              // 检测本身失败
```

**四种 reason 的必要性**：UI 对它们的处理完全不同 —— `not-desktop` 显示「桌面端可用」，`platform` 显示「当前系统不支持」，`permission` 显示「去授权」按钮，`error` 显示重试。合并成一种会让下游任务无法做正确的 UI。

### 3.3 检测与缓存

- 能力清单在应用启动时从 Go 侧拉取一次，缓存在 query cache（key 前缀 `desktopKeys`）
- 权限类状态（`permission`）**不缓存或短 TTL** —— 用户可能在系统设置里改了授权
- 提供显式失效方法，供 D2 在请求权限后刷新

## 4. Hook

```ts
function useDesktopCapability<TResult = void>(
  name: DesktopCapabilityName
): CapabilityState<TResult>
```

行为：

- 非 Desktop 环境 → 直接返回 `{status:'unavailable', reason:'not-desktop'}`，**不发任何请求**
- Desktop 环境 → 读缓存的能力清单，命中返回 `available` 并带 `invoke`
- 清单加载中 → `checking`
- `invoke` 内部捕获错误，返回结构化失败结果，**不抛异常**

**为什么不抛异常**：业务组件如果要 try/catch 每次调用，边界层就失去了意义。

## 5. 环境判断

`platform.ts` 提供唯一来源：

```ts
export function isDesktopRuntime(): boolean
export function getPlatformInfo(): PlatformInfo | null
```

判断依据在实现时确认（Wails 注入的全局对象或 UA 标记）。**全仓库不得有第二处判断**。

## 6. Go 侧注册框架

### 6.1 注册模式

`internal/desktop/bridge.go`（新建）定义：

```go
type Capability interface {
    Name() string
    Available() (bool, UnavailableReason, string)
    Invoke(ctx context.Context, payload []byte) ([]byte, error)
}

type Registry struct { /* ... */ }
func (r *Registry) Register(c Capability)
func (r *Registry) List() []CapabilityDescriptor  // 供前端拉能力清单
func (r *Registry) Invoke(ctx, name, payload) ([]byte, error)
```

D2/D3 **只在装配处追加一行 `registry.Register(...)`**，不改 `bridge.go`（路线图 §8.2 的 append-only 约定）。

### 6.2 错误

能力不可用或调用失败返回结构化错误，错误码 `DESKTOP_*`：

| 码 | 场景 |
|---|---|
| `DESKTOP_CAPABILITY_UNAVAILABLE` | 能力在当前平台不可用 |
| `DESKTOP_CAPABILITY_NOT_FOUND` | 请求了未注册的 capability |
| `DESKTOP_PERMISSION_DENIED` | 权限未授予 |
| `DESKTOP_INVOKE_FAILED` | 调用本体失败 |

**任何路径都不 panic** —— Wails binding 中的 panic 会直接杀掉桌面进程。`Invoke` 外层统一 `recover`。

## 7. 样板能力：Environment

选最小的能力验证全链路：返回平台名、OS 版本、应用版本、已注册能力清单。

**为什么选它**：无权限要求、无副作用、三平台都可用、且它返回的能力清单本身就是 §3.3 的数据源 —— 一举两得。

验证链路：Go 实现 → 注册 → 前端拉取 → hook 消费 → 组件渲染 → 浏览器降级。

## 8. Web fallback

浏览器环境下：

- `isDesktopRuntime()` 返回 false
- 所有 capability 返回 `{status:'unavailable', reason:'not-desktop'}`
- **不发请求**（避免无意义的 404）
- 提供 noop `invoke` 的替代：直接不给 `invoke`，类型上强制 UI 分支处理

TypeScript 的 discriminated union 保证组件必须处理 `unavailable` 分支才能拿到 `invoke`，这是类型层面的安全带。

## 9. 兼容性与回滚

- 纯新增，无现有代码改动（除装配处追加）→ 无破坏性变更
- 回滚：删除 `src/lib/desktop/` 与 `internal/desktop/bridge.go`，撤销装配行
- 无数据迁移

## 10. 风险

| 风险 | 处置 |
|---|---|
| 契约定得太窄，D2/D3 不够用 | design 评审邀请 D2/D3 视角审查；特别确认 `permission` reason 与异步授权流程 |
| 全局对象读取扩散到其他文件 | lint 规则或 review checklist 守住 `bridge.ts` 唯一性 |
| Wails binding panic 杀进程 | `Invoke` 外层统一 recover（§6.2） |
| 能力清单缓存导致权限状态陈旧 | 权限类不缓存或短 TTL（§3.3） |
| 与 W 轨任务争抢 AppShell | 本任务只加 capability gate，不改布局（路线图 §8.2） |
