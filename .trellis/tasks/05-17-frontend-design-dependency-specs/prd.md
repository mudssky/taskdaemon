# 前端设计系统与依赖治理规范

## Goal

补齐 UI/design system、全局样式增长边界和前端依赖治理规范，避免 shadcn 迁移后再次出现多套控件体系、散落色值或单页面随意引入重型依赖。

## Requirements

* 规范 `styles.css` 的长期增长边界：Tailwind 入口、theme variables、基础布局和过渡期全局 class。
* 规范色彩、间距、radius、字号、阴影等 token 的沉淀方式。
* 规范哪些可复用控件必须进入 `components/ui`，哪些布局样式可以留在全局 CSS。
* 规范图标按钮、危险操作、表格、badge、empty state 等常用 UI 模式的复用边界。
* 规范新增依赖门槛：是否已有本地模式、是否影响 bundle、是否跨 Web/Desktop 可用、是否引入第二套解决方案。
* 将稳定规则写入 `.trellis/spec/frontend`，优先更新 `component-guidelines.md`、`quality-guidelines.md`，必要时新增 design-system/dependency 规范文件。

## Acceptance Criteria

* [ ] `.trellis/spec/frontend` 包含设计系统和样式增长边界规则。
* [ ] `.trellis/spec/frontend` 包含前端依赖治理规则。
* [ ] 明确 shadcn/ui 与过渡期全局 CSS 的关系。
* [ ] 明确禁止单页面临时创造第二套控件体系。
* [ ] 更新 `index.md` 的规范索引或开发前检查项。

## Definition of Done

* 文档主要内容使用中文。
* 规范保持工具型管理台的视觉定位，不写营销页/视觉稿类要求。
* 如只改 spec/Markdown，不需要跑前端测试。

## Out of Scope

* 不做视觉 redesign。
* 不删除现有全局 CSS。
* 不新增或移除业务依赖。
* 不引入完整设计 token 构建系统。

## Technical Notes

* 来源 backlog：归档 PRD 中的“UI / 设计系统 / 样式规范”“依赖治理规范”。
* 相关现有规范：`.trellis/spec/frontend/component-guidelines.md`、`.trellis/spec/frontend/quality-guidelines.md`。
* 相关代码示例：`apps/web/src/styles.css`、`components/ui/*`、`apps/web/package.json`。
