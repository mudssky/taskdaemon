# 前端路由认证与桌面边界规范

## Goal

补齐 TanStack Router 路由组织、认证守卫、URL 参数和 Web/Desktop 共用边界规范，确保后续桌面能力和更多页面路由扩展时不会把平台差异泄漏到业务组件里。

## Requirements

* 规范受保护入口和认证状态 query 的职责，避免每个页面重复认证判断。
* 规范页面标题、导航、创建/编辑路由和复杂流程的组织方式。
* 规范 URL search params 的类型校验、默认值和状态边界。
* 规范 Web/Desktop 共用 UI：Desktop 专属能力通过 hook/service 边界注入，页面组件不直接依赖 Wails 全局对象。
* 规范 `/api` 请求在 Web/Desktop 下的统一网络路径假设，避免把 Desktop resource server 当作业务 API 转发层。
* 将稳定规则写入 `.trellis/spec/frontend`，必要时更新 `directory-structure.md`、`state-management.md` 或新增路由/平台边界规范文件。

## Acceptance Criteria

* [ ] `.trellis/spec/frontend` 包含路由与认证守卫长期规则。
* [ ] `.trellis/spec/frontend` 包含 Web/Desktop 共用边界规则。
* [ ] 明确 Desktop 专属能力的封装边界和 Web fallback 原则。
* [ ] 明确 URL 参数进入组件前的类型校验原则。
* [ ] 更新 `index.md` 的规范索引或开发前检查项。

## Definition of Done

* 文档主要内容使用中文。
* 不要求当前 Desktop 能力完整实现，只定义未来扩展边界。
* 如只改 spec/Markdown，不需要跑前端测试。

## Out of Scope

* 不拆分现有 route tree。
* 不改 Wails 配置或 Go desktop 代码。
* 不引入新的路由库或状态管理方案。

## Technical Notes

* 来源 backlog：归档 PRD 中的“路由与认证守卫规范”“Web/Desktop 共用边界规范”。
* 相关现有规范：`.trellis/spec/frontend/directory-structure.md`、`.trellis/spec/frontend/state-management.md`。
* 相关代码示例：`apps/web/src/routes/router.tsx`、`AppShell.tsx`、Wails desktop 边界代码。
