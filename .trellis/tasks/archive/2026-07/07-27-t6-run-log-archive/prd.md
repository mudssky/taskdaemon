# T6 Run 完整日志归档

> **轨**：Track T ｜ **依赖**：无（W1 波次，可立即开工）
> **性质**：后端为主，前端仅在 runs 页面追加下载入口（小改动，不单独拆任务）。

## Goal

为每次任务运行落盘完整的 stdout/stderr 日志文件，提供检索与下载能力，摆脱当前「数据库存截断输出」的限制。

## Context

一期 Out of Scope 明确列了「完整 run 日志文件归档（DB 截断输出 ≠ 进程日志）」，来源：`.trellis/tasks/archive/2026-05/05-14-cross-platform-scheduler-daemon/prd.md`。

当前状态：

- run 输出存在数据库字段中，长输出被截断
- `internal/logging/file_handler.go` 已有 lumberjack 进程日志轮转，但那是**进程日志**，不是**run 日志**，两者不能混用
- 执行历史与 run 生命周期见 `.trellis/spec/backend/scheduler-runner-guidelines.md`
- 错误码前缀：`RUNLOG_*`（路线图 §9.1 已分配）
- 独占路径：`internal/runlog/**`（新建）、`internal/logging/file_handler.go`

## Requirements

### R1 日志落盘

- 每次 run 产生独立日志文件，stdout 与 stderr 可区分（合并存储也可，但必须能区分来源）
- 文件路径规则确定且可推导，存放在 taskdaemon 用户数据目录下受控位置
- 落盘不阻塞 run 执行，不因写日志失败导致 run 失败
- 落盘失败必须记录到进程日志并在 run 记录上留下可见标记

### R2 与数据库字段的关系

- 数据库保留**截断摘要**用于列表页快速预览，不存全量
- run 记录关联到日志文件（路径或标识），关联在 run 结束时确定
- 明确区分三种状态：有归档、无归档（旧数据）、归档已被清理

### R3 保留策略

- 按**保留天数**和**总体积上限**两个维度配置，有默认值
- 配置为 0 表示该维度不限
- 清理策略明确：先清哪个、边界条件是什么
- 清理动作不影响正在写入的 run 日志
- 清理有日志记录，可观测

### R4 查询与下载 API

- 按 runId 查询日志元信息（是否存在、大小、创建时间）
- 下载完整日志文件
- 支持范围读取（尾部 N 行 / 字节偏移），供页面「查看最近输出」而不下载整个文件
- 大文件下载走流式，不整体读入内存
- 统一 envelope、traceId、`RUNLOG_*` 稳定错误码

### R5 安全

- 日志文件路径不可被请求参数穿越（路径遍历防护）
- 日志内容脱敏边界与 `internal/httpapi/redaction.go` 一致
- 归档目录权限不对其他用户可读

### R6 前端最小接入

- runs 页面追加「下载完整日志」入口
- 无归档时显示明确说明，不显示无效按钮
- 遵守 `.trellis/spec/frontend/interaction-guidelines.md` 的 loading/empty/error 约定
- **范围限制**：只加下载入口与状态提示，不做日志查看器（归后续任务）

### R7 Ent schema 约定

- 若需要新增字段或 entity，独立成文件
- 遵守路线图 §8.3：与 T2a 不得同时处于「已改 schema 未合并」状态，**T2a 优先**

## Acceptance Criteria

- [ ] 每次 run 产生独立日志文件，stdout/stderr 来源可区分
- [ ] 文件路径规则确定，存放在受控目录
- [ ] 落盘失败不导致 run 失败，且有进程日志与 run 记录标记
- [ ] 数据库保留截断摘要，列表页预览不变慢
- [ ] run 记录能关联到日志文件
- [ ] 「有归档 / 无归档 / 已清理」三种状态可区分
- [ ] 保留天数与总体积上限均可配置，默认值合理
- [ ] 两个维度配置为 0 时不限
- [ ] 清理不影响正在写入的日志
- [ ] 清理动作有日志记录
- [ ] 日志元信息查询 API 可用
- [ ] 完整日志下载 API 可用，大文件走流式
- [ ] 范围读取（尾部 N 行或字节偏移）可用
- [ ] 路径遍历攻击被拒绝（用测试证明）
- [ ] 日志内容脱敏边界与现有 redaction 一致
- [ ] 归档目录权限不对其他用户可读
- [ ] runs 页面有下载入口，无归档时显示明确说明
- [ ] 全部错误路径返回 `RUNLOG_*` 稳定错误码与 traceId
- [ ] 后端 route 测试覆盖成功路径与主要失败路径
- [ ] `pnpm test:go`、`pnpm vet:go`、`pnpm typecheck`、`pnpm lint`、`pnpm --filter @taskdaemon/web test` 全绿

## Out of Scope

- 日志查看器 UI（搜索、高亮、实时跟随）
- 日志聚合到外部系统（ELK / Loki）
- 实时日志流式推送（WebSocket/SSE）
- 日志压缩归档到对象存储
- 跨 run 的日志检索

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**产出** | 无契约冻结职责 |
| 本任务**消费** | C-1 配置写入契约（若日志配置要进设置页；不进则无依赖） |
| 共享文件 | `internal/config/types.go` / `defaults.go`、`internal/httpapi/router.go`（均 append-only） |
| ⚠️ 冲突 | 与 T2a 争抢 Ent 生成物，**T2a 优先**，本任务合并后重跑 `pnpm generate:go` |

## Definition of Done

- Acceptance Criteria 全部勾选
- 路线图 §7.2 状态已回写
- 若新增配置项，已按 C-1 契约形状定义

## Notes

- **进程日志与 run 日志是两回事**，不要复用 lumberjack 的轮转逻辑去管 run 日志的保留策略
- 大文件路径是本任务的性能风险点，流式与范围读取要在设计阶段定清楚
- 前端改动刻意压到最小，避免与 W 轨任务争抢 runs feature 目录
