# W1 完整设置页（Web 前端）

> **轨**：Track W ｜ **依赖**：**C-1 冻结**（T1a 配置写入契约），**非** T1a 完整交付
> **性质**：纯前端。Web 与 Desktop 共用同一套组件（路线图 §4.2）。

## Goal

落地 taskdaemon 的完整设置页：按 section 分面板展示与编辑配置，明确区分热生效与需重启，危险操作有确认。

## Context

当前状态：

- 只有 `AudioSettingsPage`（只读展示，T0 正在补写入能力）
- 无统一设置页入口
- 前端规范见 `.trellis/spec/frontend/`，重点：`form-testing-guidelines.md`、`interaction-guidelines.md`、`api-query-contracts.md`
- query key 前缀：`settingsKeys`（路线图 §9.2 已分配）
- 独占路径：`apps/web/src/features/settings/**`（新建）

**并行前提**：C-1 冻结后即可用 mock 开发，不等 T1a 交付。禁止自行发明字段名（路线图 §8.4）。

## Requirements

### R1 页面结构

- 设置页按 section 分面板（数据库、日志、运行、音频等），每个面板独立保存
- 面板之间互不干扰：一个面板保存失败不影响其他面板已填内容
- 面板可折叠或分 tab，具体形式在 design 阶段定
- 路由沿用 TanStack Router 手写 route 定义模式（`routing-platform-guidelines.md`）

### R2 三级配置的差异化呈现

按 C-1 的配置分级：

| 级别 | UI 呈现 |
|---|---|
| 热生效 | 正常可编辑，保存后提示「已生效」 |
| 需重启 | 可编辑，保存后**显著提示需要重启**，并说明重启前的当前行为 |
| 仅配置文件 | 只读展示当前值或「已配置/未配置」状态，附配置文件路径提示，不提供编辑控件 |

- 需重启提示必须是持久可见的状态，不是一闪而过的 toast
- 只读项不能显示为「禁用的输入框」误导用户，应明确是只读展示

### R3 表单与校验

- 复杂表单按 `form-testing-guidelines.md` 分层：schema / default values / payload mapper 分离
- 客户端校验与 C-1 服务端校验规则一致，不能宽于服务端
- 服务端返回的**字段级错误**必须映射到对应表单字段，不是笼统 toast
- 未保存改动离开页面时有提示
- 保存按钮在无改动时禁用

### R4 敏感值处理

- 敏感值（token、密码）**永不明文回显**，只展示「已配置 / 未配置」
- 提供「重新设置」入口，输入新值后提交
- 生成类敏感值（如 Token）遵循「生成后只显示一次」，并提供复制按钮
- 前端不缓存敏感值明文，不写入 localStorage、query cache 持久层

### R5 危险操作确认

- 识别哪些配置属于危险操作（可能导致服务不可用、数据不可访问），在 design 阶段列出清单
- 危险操作使用确认对话框，说明后果，需二次确认
- 确认交互沿用 `interaction-guidelines.md` 的既有模式

### R6 状态与错误恢复

- loading / empty / error 状态齐全（`interaction-guidelines.md`）
- 后端不可用时展示明确恢复路径
- 错误展示包含 traceId，便于排障
- reload 结果（哪些子系统生效/失败）在保存后可见

### R7 Desktop 兼容

- 不直接读取 Wails 全局对象；需要 Desktop 能力时通过 D1 的 `useDesktopCapability`
- Desktop 专属配置项在浏览器环境显示禁用态与说明，不隐藏（用户需要知道存在这个能力）

### R8 测试

- 按项目规范：测业务逻辑、schema 映射、payload mapper、状态分支，**不测页面结构与 CSS**
- 字段级错误映射有测试
- 敏感值不回显有测试

## Acceptance Criteria

- [x] 设置页路由可达，按 section 分面板
- [x] 每个面板可独立保存，互不干扰
- [x] 热生效项保存后提示已生效
- [x] 需重启项保存后有**持久可见**的重启提示
- [x] 仅配置文件项为只读展示，附配置文件路径，不显示为禁用输入框
- [x] 表单按规范分层（schema / defaults / payload mapper 分离）
- [x] 客户端校验不宽于服务端
- [x] 服务端字段级错误正确映射到对应表单字段
- [x] 未保存改动离开页面有提示
- [x] 无改动时保存按钮禁用
- [x] 敏感值不明文回显，只展示已配置/未配置
- [x] 「重新设置」入口可用
- [x] 生成类敏感值只显示一次，有复制按钮
- [x] 敏感值明文不进 localStorage 与持久缓存（用测试证明）
- [x] 危险操作清单已列出，均有二次确认对话框并说明后果
- [x] loading / empty / error 状态齐全
- [x] 错误展示包含 traceId
- [x] reload 结果在保存后可见
- [x] 无任何直接 Wails 全局对象引用
- [x] Desktop 专属项在浏览器显示禁用态与说明
- [x] 业务逻辑与 schema 映射有单元测试
- [x] `pnpm typecheck`、`pnpm lint`、`pnpm --filter @taskdaemon/web test` 全绿
- [x] 与 T1a 联调通过，mock 与真实接口行为一致

## Out of Scope

- 通用 YAML 编辑器（C-1 已明确不做）
- 配置版本历史与回滚 UI
- 配置导入导出
- 后端配置写入能力（归 T1a）
- 通知配置面板（归 W2）

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**消费** | **C-1**（配置分级表、写入 API 形状、字段级错误结构、reload 结果结构）、**C-5**（Desktop capability，若涉及） |
| 本任务**产出** | `settingsKeys` query key 命名空间 |
| 共享文件 | `apps/web/src/routes/router.tsx`、`app/queryClient.ts`、AppShell 导航（均 append-only） |

**开工条件**：C-1 写入 spec 即可开工，用 mock 开发；联调等 T1a 交付。

## Design / Implement 补齐条件

`design.md` 与 `implement.md` **待 C-1 冻结后补**。原因：面板划分、字段分级呈现、错误映射结构全部由 C-1 决定。

## Definition of Done

- Acceptance Criteria 全部勾选
- 与 T1a 联调通过
- 路线图 §7.3 状态已回写

## Notes

- **禁止自行发明字段名**：mock 数据必须严格按 C-1 的 DTO 定义（路线图 §8.4）
- 「需重启」的提示是本页面最容易做错的交互 —— 用户保存完以为生效了，实际没有。提示必须持久可见
- 面板独立保存优于整页保存，能显著降低误操作范围
