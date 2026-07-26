# TD1 Desktop 平台能力边界层 — 执行计划

## 前置

- [ ] 读 `.trellis/spec/frontend/routing-platform-guidelines.md`（本任务是这份规范的实现载体）
- [ ] 读 `.trellis/spec/frontend/hook-guidelines.md`、`state-management.md`、`type-safety.md`
- [ ] 读 `.trellis/spec/backend/error-handling.md`（`DESKTOP_*` 错误码）
- [ ] 确认当前 Wails 版本与 binding 机制（`services/taskdaemon-go/go.mod` 的 `wails/v3`）
- [ ] 从 `dev` 切分支 `feat/td1-desktop-platform-bridge`

## 阶段 1 · Go 侧注册框架

- [ ] `internal/desktop/bridge.go`：`Capability` 接口、`Registry`、`CapabilityDescriptor`
- [ ] `Registry.Invoke` 外层统一 `recover`（design §6.2 —— panic 会杀桌面进程）
- [ ] `DESKTOP_*` 错误码定义，四种场景各有对应码
- [ ] 未注册 capability 返回 `DESKTOP_CAPABILITY_NOT_FOUND`
- [ ] 单测：
  - [ ] 注册与查找
  - [ ] 未注册名返回正确错误
  - [ ] capability 内部 panic 被 recover，不向上传播
  - [ ] 不可用能力返回结构化 reason

**验证**：`cd services/taskdaemon-go && go test ./internal/desktop/...`

## 阶段 2 · 样板能力 Environment

- [ ] `internal/desktop/capability_environment.go`：返回平台名、OS 版本、应用版本、已注册能力清单
- [ ] 在 `internal/desktop/desktop.go` 装配处**追加**注册行（append-only，标注 `// TD1`）
- [ ] 暴露给前端的 Wails binding 方法（拉能力清单 + 调用 capability 两个入口）
- [ ] 单测：Environment 返回字段完整

**验证**：`go test ./internal/desktop/...`；Wails 壳内手动确认可调用

## 阶段 3 · 前端边界层骨架

- [ ] 创建 `apps/web/src/lib/desktop/`
- [ ] `platform.ts`：`isDesktopRuntime()`、`getPlatformInfo()` —— **环境判断的唯一来源**
- [ ] `bridge.ts`：Wails 调用封装 —— **全仓库唯一读取 Wails 全局对象的文件**
- [ ] `capabilities.ts`：`DesktopCapability` 常量、`DesktopCapabilityName` 类型、`CapabilityState` union、`UnavailableReason`
- [ ] `index.ts`：对外唯一出口
- [ ] 单测：`platform.ts` 在有/无 Wails 全局对象两种环境下的判断正确

**验证**：`pnpm --filter @taskdaemon/web test`

## 阶段 4 · Hook 与缓存

- [ ] `useDesktopCapability.ts` 实现（design §4）
- [ ] 非 Desktop 环境**不发请求**，直接返回 `not-desktop`
- [ ] 能力清单走 query cache，key 前缀 `desktopKeys`（路线图 §9.2）
- [ ] 权限类状态不缓存或短 TTL
- [ ] 提供显式失效方法（供 D2 授权后刷新）
- [ ] `invoke` 内部捕获错误返回结构化结果，**不抛异常**
- [ ] 单测：
  - [ ] 浏览器环境返回 `not-desktop` 且零请求
  - [ ] Desktop 环境命中清单返回 `available` 且带 `invoke`
  - [ ] 清单加载中返回 `checking`
  - [ ] `invoke` 失败返回结构化结果，不抛异常
  - [ ] 类型层面：`unavailable` 分支拿不到 `invoke`（用类型测试或编译期断言）

**验证**：`pnpm --filter @taskdaemon/web test`、`pnpm typecheck`

## 阶段 5 · 样板能力端到端

- [ ] 一个最小消费点（可放在现有页面的调试区或设置入口）展示 Environment 结果
- [ ] Desktop 环境：显示真实平台信息
- [ ] 浏览器环境：显示禁用态与说明，**不是报错、不是空白**
- [ ] 手动验证三平台 Wails 壳内可用
- [ ] 手动验证浏览器内优雅降级

## 阶段 6 · 唯一性守卫

- [ ] 全仓库搜索确认：除 `bridge.ts` 外无 Wails 全局对象读取
  ```bash
  grep -rn "window.runtime\|window.go\|wails" apps/web/src --include="*.ts" --include="*.tsx" | grep -v "lib/desktop/bridge.ts"
  ```
- [ ] 全仓库搜索确认：环境判断只有 `platform.ts` 一处
- [ ] 若可行，加 lint 规则或 review checklist 条目固化这条约束

## 阶段 7 · 契约冻结（C-5）

- [ ] capability 命名规则写入 `.trellis/spec/frontend/routing-platform-guidelines.md`
- [ ] `CapabilityState` 返回形状与四种 `UnavailableReason` 语义写入同处
- [ ] Web fallback 语义写入同处
- [ ] **「新增一个 Desktop 能力的完整步骤」**写入同处（Go 侧实现 → 注册 → 前端常量 → 消费），这是 D2/D3 的直接依据
- [ ] `DESKTOP_*` 错误码写入 `.trellis/spec/backend/error-handling.md`
- [ ] 回写路线图 §9 表格 C-5 行的产物路径
- [ ] 回写路线图 §7.4 状态为 done

> **冻结完成即通知 D2 / D3 可以开工。**

## 全量验收

```bash
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/web test
pnpm test:go
pnpm vet:go
```

- [ ] 上述命令全绿
- [ ] Desktop 与浏览器**两条路径**均手动验证（路线图 §11 对 Track D 的额外要求）
- [ ] `prd.md` 全部 Acceptance Criteria 勾选

## 评审门

| 门 | 时机 | 检查 |
|---|---|---|
| G-1 | 阶段 3 结束 | **邀请 D2/D3 视角审查契约**：四种 `UnavailableReason` 是否够用？异步权限授予流程能否表达？ |
| G-2 | 阶段 5 结束 | 样板能力代码是否足够可读 —— D2/D3 会直接模仿它 |
| G-3 | 阶段 7 前 | 「新增能力步骤」文档能否让人不看源码就照做 |

## 回滚点

| 阶段 | 回滚方式 |
|---|---|
| 1–2 | 删除新增 Go 文件，撤销装配行 |
| 3–5 | 删除 `src/lib/desktop/` 与消费点 |
| 全部 | 纯新增，revert 即可，无数据与配置残留 |

## 与其他任务的协调

- **D2 / D3**：阶段 7 完成后通知开工。**契约不够用时回本任务改**，不要在 D2/D3 绕过
- **W 轨任务**：若设置页需要展示 Desktop 能力状态，消费本任务的 hook
- **T5**：注意 D3 的「Desktop 自启动」与 T5 的「系统服务」是两回事，本任务的 capability 命名要避免歧义
