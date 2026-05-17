# 前端开发规范

> taskdaemon Web/Desktop 共用管理台的长期代码规范索引。

---

## 概览

前端应用位于 `apps/web`，使用 Vite + React + TypeScript。它同时服务浏览器 Web 和 Wails Desktop，业务入口是任务管理、运行历史、认证和状态概览。

本目录只沉淀长期前端工程约定。单个任务的完整 PRD、验收矩阵、阶段性产品范围和一次性接口清单应保留在 `.trellis/tasks/` 或相关研究文档中。

---

## 规范索引

| 文档 | 内容 | 状态 |
|---|---|---|
| [目录结构](./directory-structure.md) | Vite app、feature、route、API client 组织 | 当前代码事实 |
| [文件组织与拆分](./file-organization.md) | 页面、组件、表单、测试和样式拆分规则 | 当前代码事实 |
| [API 与 Query 契约](./api-query-contracts.md) | API client、DTO、query key、mutation 和缓存失效 | 当前代码事实 |
| [组件规范](./component-guidelines.md) | 组件拆分、props、样式、可访问性 | 当前代码事实 |
| [交互状态与错误恢复](./interaction-guidelines.md) | loading/empty/error、确认交互、可访问性、traceId 与调试信息 | 当前代码事实 |
| [Hook 规范](./hook-guidelines.md) | TanStack Query、表单和副作用封装 | 当前代码事实 |
| [状态管理](./state-management.md) | server/URL/form/local/app state 边界 | 当前代码事实 |
| [运行时 Query 与轮询规范](./runtime-query-guidelines.md) | 自动轮询、手动刷新、query 副作用和跨页面缓存复用 | 当前代码事实 |
| [路由认证与平台边界](./routing-platform-guidelines.md) | TanStack Router、认证守卫、URL 参数和 Web/Desktop 共用边界 | 当前代码事实 |
| [类型安全](./type-safety.md) | DTO、Zod schema、union、运行时校验 | 当前代码事实 |
| [表单校验与测试分层](./form-testing-guidelines.md) | 复杂表单分层、软警告、schema/payload 映射和测试分层 | 当前代码事实 |
| [质量规范](./quality-guidelines.md) | lint/typecheck/test 与评审清单 | 当前代码事实 |

---

## 开发前检查

前端实现前：

1. 读取 [目录结构](./directory-structure.md)，确认代码应放在 `routes`、`features`、`components`、`lib` 还是 `app`。
2. 新增或拆分页面、组件、表单、测试或样式时读取 [文件组织与拆分](./file-organization.md)。
3. 创建或修改 UI 时读取 [组件规范](./component-guidelines.md)。
4. 修改 loading、empty、error、disabled、确认对话框、可访问性或调试信息时读取 [交互状态与错误恢复](./interaction-guidelines.md)。
5. 增加 API、query、mutation、DTO 或缓存失效逻辑时读取 [API 与 Query 契约](./api-query-contracts.md)。
6. 增加数据请求、表单或副作用时读取 [Hook 规范](./hook-guidelines.md) 和 [状态管理](./state-management.md)。
7. 增加运行态数据、自动轮询、手动刷新、后台 refetch 或跨页面 query cache 复用时读取 [运行时 Query 与轮询规范](./runtime-query-guidelines.md)。
8. 增加路由、认证守卫、页面标题/导航、URL 参数或 Desktop/Web 平台能力时读取 [路由认证与平台边界](./routing-platform-guidelines.md)。
9. 增加 schema、表单映射或 URL 参数时读取 [类型安全](./type-safety.md)。
10. 新增或修改复杂表单、软警告、payload mapper 或测试分层时读取 [表单校验与测试分层](./form-testing-guidelines.md)。
11. 修改测试、质量命令或依赖时读取 [质量规范](./quality-guidelines.md)。
12. 跨前后端契约变更时，同时读取后端错误/API 相关规范和 `.trellis/spec/guides/cross-layer-thinking-guide.md`。

---

## Spec 维护边界

适合写入本目录：

* 已经在 `apps/web` 中形成的目录、命名、文件拆分、状态管理、表单、测试和 API client 约定。
* 多个未来任务会复用的 DTO/错误码/缓存失效/校验边界。
* 多个表单会复用的 schema/default values/payload mapper、软警告和测试分层规则。
* 多个页面会复用的 loading、empty、error、disabled、确认交互和可访问性约定。
* 多个运行态页面会复用的轮询准入、停止条件、刷新状态和 query cache 失效约定。
* 多个页面或平台会复用的路由守卫、URL 参数校验和 Web/Desktop 边界约定。
* 明确禁止模式和常见错误。

不适合写入本目录：

* 某个页面在特定阶段必须出现的控件清单。
* 历史 PRD 中完整的 API 方法列表，除非它已经成为长期公共契约。
* 纯视觉稿或临时产品决策。

---

**语言**：项目工作文档以中文为主；第三方生成内容例外。
