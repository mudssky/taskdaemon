# 后端开发规范

> taskdaemon Go 后端的长期代码规范索引。

---

## 概览

本目录只沉淀长期有效、可重复执行的后端工程约定。历史任务的 PRD、接口全集、阶段性验收矩阵和一次性需求背景应保留在 `.trellis/tasks/`、研究笔记或 ADR 中，不直接搬进长期 spec。

当前 Go 后端位于 `services/taskdaemon-go`，包含 HTTP API、CLI、daemon、调度器、runner、数据层、认证和 Wails Desktop 边界。写后端代码前，应优先根据触达包读取对应规范。

---

## 规范索引

| 文档 | 内容 | 状态 |
|---|---|---|
| [目录结构](./directory-structure.md) | workspace、Go module、入口与包边界 | 当前代码事实 |
| [Go 文件组织与拆分](./file-organization.md) | 文件体量、职责切分、拆分顺序 | 当前代码事实 |
| [API 与 DTO 契约](./api-contracts.md) | HTTP handler、DTO、envelope、错误码、前后端联动 | 当前代码事实 |
| [配置、CLI 与运行时重载](./configuration-runtime-guidelines.md) | 配置加载顺序、env、local 覆盖、runtime reload、CLI daemon API | 当前代码事实 |
| [数据库规范](./database-guidelines.md) | Ent、迁移、查询边界、事务 | 当前代码事实 |
| [错误处理](./error-handling.md) | sentinel/typed error、API envelope、敏感信息 | 当前代码事实 |
| [Scheduler 与 Runner 生命周期](./scheduler-runner-guidelines.md) | cron 校验、运行态、执行历史、timeout/cancel、runner JSON | 当前代码事实 |
| [通知事件总线契约 C-2](./notification-event-contract.md) | 事件名、payload、Sink/Bus、站内已读模型、配置键 | **C-2 已冻结** |
| [质量规范](./quality-guidelines.md) | 后端测试、行为质量、评审清单 | 当前代码事实 |
| [日志规范](./logging-guidelines.md) | slog、HTTP 日志、脱敏、运行时重载 | 当前代码事实 |

---

## 开发前检查

后端实现前：

1. 读取 [目录结构](./directory-structure.md)，确认要修改的包边界。
2. 若新增或修改 Go 文件，读取 [Go 文件组织与拆分](./file-organization.md)。
3. API、DTO、HTTP handler、错误码、traceId 或前后端接口联动读取 [API 与 DTO 契约](./api-contracts.md) 和 [错误处理](./error-handling.md)。
4. 配置结构、配置文件、环境变量、CLI 配置命令或运行时 reload 读取 [配置、CLI 与运行时重载](./configuration-runtime-guidelines.md)。
5. 数据、schema、migration 或 repository 相关工作读取 [数据库规范](./database-guidelines.md)。
6. API、CLI、auth、scheduler、runner 相关错误路径读取 [错误处理](./error-handling.md)。
7. 调度、runner、daemon 状态、执行历史、timeout/cancel 或 runner JSON 读取 [Scheduler 与 Runner 生命周期](./scheduler-runner-guidelines.md) 和 [质量规范](./quality-guidelines.md)。
8. 日志、trace、HTTP body 捕获或配置热重载相关工作读取 [日志规范](./logging-guidelines.md)。
9. 跨前后端或跨包改动时，同时读取 `.trellis/spec/guides/` 下的思考指南。

---

## Spec 维护边界

适合写入本目录：

* 已由现有代码验证的包边界、命名、错误处理、日志、测试和依赖约定。
* 会被多个未来任务复用的接口形状、配置键、错误码、运行时行为。
* 反复容易踩坑的禁止模式、评审清单和最小示例。
* 跨 HTTP API、CLI、Desktop、前端和数据层的长期契约。

不适合写入本目录：

* 单个需求的完整 PRD、验收标准、任务拆解、路线图或阶段性范围。
* 一次性接口全集，除非它已经成为长期公共契约并需要未来实现保持兼容。
* 过期计划性描述，例如未落地计划或路线图承诺，除非明确标记为开放决策并有 owner。

---

**语言**：项目工作文档以中文为主；第三方工具生成内容例外。
