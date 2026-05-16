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
| [组件规范](./component-guidelines.md) | 组件拆分、props、样式、可访问性 | 当前代码事实 |
| [Hook 规范](./hook-guidelines.md) | TanStack Query、表单和副作用封装 | 当前代码事实 |
| [状态管理](./state-management.md) | server/URL/form/local/app state 边界 | 当前代码事实 |
| [类型安全](./type-safety.md) | DTO、Zod schema、union、运行时校验 | 当前代码事实 |
| [质量规范](./quality-guidelines.md) | lint/typecheck/test 与评审清单 | 当前代码事实 |

---

## 开发前检查

前端实现前：

1. 读取 [目录结构](./directory-structure.md)，确认代码应放在 `routes`、`features`、`components`、`lib` 还是 `app`。
2. 创建或修改 UI 时读取 [组件规范](./component-guidelines.md)。
3. 增加数据请求、mutation、表单或副作用时读取 [Hook 规范](./hook-guidelines.md) 和 [状态管理](./state-management.md)。
4. 增加 DTO、schema、API client、表单映射或 URL 参数时读取 [类型安全](./type-safety.md)。
5. 修改测试、质量命令或依赖时读取 [质量规范](./quality-guidelines.md)。
6. 跨前后端契约变更时，同时读取后端错误/API 相关规范和 `.trellis/spec/guides/cross-layer-thinking-guide.md`。

---

## Spec 维护边界

适合写入本目录：

* 已经在 `apps/web` 中形成的目录、命名、状态管理、表单、测试和 API client 约定。
* 多个未来任务会复用的 DTO/错误码/缓存失效/校验边界。
* 明确禁止模式和常见错误。

不适合写入本目录：

* 某个页面在特定阶段必须出现的控件清单。
* 历史 PRD 中完整的 API 方法列表，除非它已经成为长期公共契约。
* 纯视觉稿或临时产品决策。

---

**语言**：项目工作文档以中文为主；第三方生成内容例外。
