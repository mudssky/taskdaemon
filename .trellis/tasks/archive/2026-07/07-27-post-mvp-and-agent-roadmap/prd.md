# post-MVP 与通用 Agent 路线图（父任务：地图与跨任务契约）

## Goal

为 taskdaemon 从「一期调度核心可用」切换到「产品深度 + 通用 Agent 后端」提供**并行开发施工图**：四轨分期、任务依赖图、跨任务契约冻结点、文件所有权边界。

**本任务不写业务代码。** 交付物是路线图文档、任务树结构与跨任务契约仲裁。

## Context

- 路线图 SoT：`docs/roadmap-post-mvp-and-agent.md`（v3）
- 一期已交付基线与 Out of Scope 来源：`.trellis/tasks/archive/2026-05/05-14-cross-platform-scheduler-daemon/prd.md`
- 唯一活跃任务 `05-28-hermes-agent-audio-api`（即 T0）已挂为本任务子任务

## Requirements

### R1 路线图文档

- 四轨（T 后端 / W Web 前端 / D Desktop / G Agent）分期与任务归属清晰可查
- 依赖图区分「完整交付依赖」与「契约冻结依赖」两种边
- 并行波次表给出每波可同时开工的最大任务集合
- 明确 Track G 暂停时 T/W/D 三轨的自洽性

### R2 跨任务契约仲裁（本任务独有职责）

- 错误码前缀分配表（C-6）由本任务维护，新任务取前缀前先查表
- 前端 query key 前缀分配表由本任务维护
- traceId 贯穿约定由本任务定义
- 契约冻结点 C-1 ~ C-5 由本任务登记归属任务与产物路径；契约**内容**由对应任务产出

### R3 文件所有权边界

- 每个子任务有明确「独占路径」列表，路径之间无重叠
- 共享文件列出 append-only 约定
- Ent 生成物冲突处置写明强制流程（本项目 Ent 自动 migration，生成物不可手工合并）

### R4 任务树结构

- 21 个子任务全部建立并挂到本父任务
- 每个子任务有完整 `prd.md`，无 `TBD` 残留
- 无依赖任务（W1 波次：T2a / T5 / T6 / D1 / G0）额外有 `design.md` + `implement.md`
- 有前置契约依赖的任务在 PRD 中标注「design 待 C-x 冻结后补」及原因

### R5 范围边界裁定

- 企业能力（SSO / 多租户 / 审计聚合 / 配额执行）移出本仓库，归外部网关；仓库内只留外部做不到的钩子
- gateway 不重做 runtime 已有能力，规模由 G0 实测判定；初版即 Pi + OMP 两个 adapter 并存
- Pi WebUI 参考重写并简化，不 fork、不作为依赖
- 不拆 `apps/desktop` 独立前端；Desktop 差异收敛在 `src/lib/desktop` 边界层

## Acceptance Criteria

- [ ] `docs/roadmap-post-mvp-and-agent.md` 四轨结构、依赖图、并行波次表完整
- [ ] §8 文件所有权表覆盖全部 21 个子任务，路径无重叠
- [ ] §9 契约冻结点表 C-1 ~ C-6 有明确冻结任务、产物路径与消费方
- [ ] §9.1 错误码前缀、§9.2 query key 前缀无冲突
- [ ] §8.3 Ent 生成物冲突处置流程可直接执行
- [ ] 21 个子任务全部创建并挂载，`python3 ./.trellis/scripts/task.py list` 可见
- [ ] 每个子任务 `prd.md` 完成，无 `TBD` 残留
- [ ] W1 波次任务（T2a / T5 / T6 / D1 / G0）有 `design.md` + `implement.md`
- [ ] 依赖契约的任务在 PRD 中标注了 design 补齐条件
- [ ] §14 文档维护规则明确各触发时机的责任人

## Out of Scope

- 任何业务代码实现（全部归子任务）
- 契约**内容**的技术设计（归 T1a / T2a / D1 / G1）
- 一期已交付项的重构
- 子任务的具体实现排期与人力分配

## Definition of Done

- 上述 Acceptance Criteria 全部勾选
- 路线图 §16 修订记录已更新
- 后续子任务开工时无需回头补充跨任务约定

## Notes

- 本任务在全部子任务 archive 后才 archive
- 子任务范围变更必须回写路线图 §7 / §8 / §9 三处
- 本任务是四轨的规划 SoT；子任务 PRD 与路线图冲突时以路线图为准，或先改路线图
