# T3 / D2 Desktop 原生通知 sink

> **轨**：Track D（路线图中标识为 D2）｜ **依赖**：**C-5 冻结**（D1 capability 契约）+ **C-2 冻结**（T2a 事件契约）
> **性质**：跨 Go/Wails 与前端边界层，但工作量小，不再拆分。

## Goal

让 taskdaemon 桌面壳能弹出系统原生通知，作为 T2a 事件总线的一个 sink。

## Context

- 一期 Out of Scope 列了「Desktop 原生通知」，来源：`.trellis/tasks/archive/2026-05/05-14-cross-platform-scheduler-daemon/prd.md`
- Wails 侧代码：`services/taskdaemon-go/internal/desktop/`
- 事件契约来自 T2a（C-2），capability 接入方式来自 D1（C-5）
- 错误码前缀：`NOTIFY_*`（sink 语义）与 `DESKTOP_*`（能力不可用）
- 独占路径：`internal/notify/sink_desktop*.go`、`internal/desktop/notification*.go`

**本任务不发明任何契约**：事件模型照搬 C-2，capability 注册照搬 C-5。

## Requirements

### R1 Desktop 通知能力（Go/Wails 侧）

- 通过 Wails 能力弹出系统原生通知，覆盖 macOS / Windows / Linux
- 通知包含标题与正文，内容由 C-2 事件渲染
- 平台不支持或权限未授予时返回 `DESKTOP_*` 结构化错误，**不 panic**
- 能力按 C-5 定义的 binding 注册模式接入，不改注册框架

### R2 通知 sink（事件总线侧）

- 实现 C-2 的 `Notifier` 接口，注册为 sink
- 仅在 Desktop 运行模式下启用；纯 server 模式下该 sink 自动禁用且不报错
- 可通过配置独立开关（C-2 已约定每个 sink 可独立开关）
- 支持按严重级别过滤
- 投递失败不影响其他 sink、不阻塞主流程（C-2 约定）

### R3 权限处理

- macOS 通知权限需用户授权：首次请求时机明确，被拒绝后有可查询状态
- 权限被拒绝时不反复弹窗骚扰
- 权限状态可通过 capability 查询，供 UI 展示

### R4 点击行为（最小实现）

- 点击通知时激活主窗口
- **不做**深链跳转到具体任务详情页（留给 D3 或后续任务）

### R5 前端接入

- 通过 D1 的 `useDesktopCapability` 消费，**不直接读 Wails 全局对象**
- 非 Desktop 环境下相关设置项显示禁用态与说明，不显示无效开关
- 提供「发送测试通知」入口，便于用户确认权限与效果

## Acceptance Criteria

- [ ] macOS 可弹出系统原生通知
- [ ] Windows 可弹出系统原生通知
- [ ] Linux 可弹出系统原生通知（或明确记录不支持的桌面环境）
- [ ] 通知标题与正文由 C-2 事件正确渲染
- [ ] 平台不支持/权限未授予时返回 `DESKTOP_*` 结构化错误，不 panic
- [ ] 能力按 C-5 注册模式接入，未修改注册框架
- [ ] sink 实现 C-2 `Notifier` 接口并成功注册
- [ ] 纯 server 模式下该 sink 自动禁用且不报错
- [ ] sink 可通过配置独立开关
- [ ] 按严重级别过滤可用
- [ ] 投递失败不影响其他 sink、不阻塞主流程（用测试证明）
- [ ] macOS 权限请求时机明确，被拒后不反复弹窗
- [ ] 权限状态可通过 capability 查询
- [ ] 点击通知激活主窗口
- [ ] 前端仅通过 `useDesktopCapability` 消费，无直接 Wails 全局对象引用
- [ ] 非 Desktop 环境显示禁用态与说明
- [ ] 「发送测试通知」入口可用
- [ ] 三平台手动验证有记录（含系统版本）
- [ ] `pnpm test:go`、`pnpm vet:go`、`pnpm typecheck`、`pnpm lint` 全绿

## Out of Scope

- 通知点击深链到任务详情（后续任务）
- 通知中心的历史与已读管理（归 T2a 站内 sink + W2）
- 自定义通知样式、富媒体通知、通知声音定制
- 通知的批量聚合与免打扰时段

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**消费** | **C-2**（事件 schema + `Notifier` 接口）、**C-5**（capability 契约 + binding 注册模式） |
| 本任务**产出** | 无契约冻结职责 |
| 共享文件 | `internal/desktop/desktop.go`（append-only 注册行）、`internal/config/types.go`（append-only） |

**开工条件**：C-2 与 C-5 双双冻结后开工。**这是全项目唯一需要等两个契约的任务**。

## Design / Implement 补齐条件

`design.md` 与 `implement.md` **待 C-2 与 C-5 均冻结后补**。原因：sink 接口来自 C-2，能力注册方式来自 C-5，二者都未定时无法写设计。

## Definition of Done

- Acceptance Criteria 全部勾选
- 三平台手动验证记录归档到任务目录
- 路线图 §7.4 状态已回写

## Notes

- 本任务体量小但**跨层**，价值在于验证 C-2 与 C-5 两个契约是否真的好用
- 若发现契约不好用，反馈给 T2a/D1 修订契约，**不要在本任务里绕过契约**
- Linux 桌面通知在不同桌面环境差异大，不支持的环境明确记录即可，不追求全覆盖
