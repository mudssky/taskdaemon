# 表单校验与测试分层

> taskdaemon 前端复杂表单、软警告、schema/payload 映射和测试分层的长期约定。

---

## 概览

前端表单优先使用 React Hook Form + Zod 管理输入状态和校验。复杂表单必须把“输入边界、默认值、DTO 映射、提交 payload、UI 字段展示”分开维护，避免组件里混入大量字符串解析、API payload 拼装和业务校验。测试按风险分层：纯函数和 schema 覆盖业务边界，组件测试覆盖用户行为，router 测试覆盖跨页面流程。

---

## 表单复杂度分级

简单表单：

* 字段少、没有跨字段校验、没有 DTO 映射或软警告，例如登录表单。
* 可以使用局部 state，但字段增长、校验变复杂或需要复用时应迁移到 React Hook Form + Zod。
* 提交前仍要有最小校验，不能只依赖后端错误。

复杂表单：

* 存在跨字段校验、条件字段、数组/键值输入、DTO -> form 默认值或 form -> API payload 映射。
* 必须使用 Zod schema 推导表单值类型，并用 React Hook Form 接入。
* schema、默认值和 payload mapper 放在 feature 附近的 `*.schema.ts` 或等价文件，不写在 JSX 主体里。
* UI section 只绑定字段、展示错误和触发 `form.setValue`，不直接构造 API payload。

---

## 分层职责

复杂表单按以下层次组织：

* `*.schema.ts`：Zod schema、`z.infer` 表单值类型、DTO -> form 默认值、form -> API payload 映射。
* `defaultXxxValues(dto)`：只负责把后端 DTO 或空值变成可编辑表单值。
* `toXxxPayload(values)`：只负责裁剪、trim、解析和构造后端 payload。
* 主表单组件：创建 RHF form、组合 section、处理 submit 和 pending 状态。
* 字段 section：渲染字段、label、错误信息和少量 UI 条件分支。
* 纯工具函数：解析 env、args、cron warning、格式化 label 等可独立测试的逻辑。

组件禁止直接拼业务请求体；API payload 必须经 schema/mapper 边界产生。

---

## React Hook Form 与 shadcn 控件

* 原生输入型控件（`Input`、`Textarea`）优先使用 `register`。
* Radix/shadcn 非原生控件（`Select`、`Checkbox` 等）使用受控值加 `form.setValue(..., { shouldDirty: true, shouldValidate: true })`。
* 需要重置编辑对象时，通过 `form.reset(defaultXxxValues(dto))` 保持 DTO -> form 单一入口。
* submit 按钮使用 `isSubmitting` 或 mutation pending 状态禁用，避免重复提交。
* 字段错误从 `form.formState.errors` 展示，不在组件里重复维护一套错误 state。

---

## 硬错误与软警告

* 硬错误阻止提交，例如必填字段缺失、cron 表达式非法、runner 必填字段为空。
* 软警告表示可提交但有风险，例如秒级/高频 cron。软警告必须要求显式确认后进入 payload。
* 软警告确认字段属于表单值的一部分，例如 `confirmCronWarnings`，并由 schema 校验。
* `toXxxPayload` 必须把确认字段传给后端，不能只在 UI 里吞掉。
* 测试需要同时覆盖硬错误阻断、软警告未确认阻断、软警告确认后允许提交。

---

## 测试分层

纯函数测试：

* 覆盖格式化、解析、默认值、payload mapper、cron/env/args 等不需要 DOM 的逻辑。
* 优先断言输入输出，不模拟用户交互。

Schema 测试：

* 覆盖必填、跨字段、条件字段、软警告和默认值。
* 与 UI 组件测试互补，不把所有校验只放在组件测试里验证。

API client 测试：

* 覆盖 envelope 成功解包、稳定错误码、traceId、认证失败和关键业务动作。
* 不依赖组件渲染来证明 API client 行为。

组件交互测试：

* 使用 Testing Library 从用户行为出发，覆盖填写、提交、错误展示、禁用态和确认路径。
* 不依赖 className、DOM 层级或纯 CSS 样式断言。
* 对 shadcn/Radix 控件，按真实用户可访问名称操作。

Router 集成测试：

* 覆盖认证守卫、导航、独立路由、跨页面流程和关键 mutation 后的页面结果。
* 不把所有 feature 分支塞进一个 router 测试；超过约 200 行或覆盖多个独立流程时按行为拆分。

---

## 测试文件拆分

优先按行为和风险拆分，而不是按实现文件机械拆分：

* `*.schema.test.ts`：schema、默认值、payload mapper、纯解析函数。
* `*.test.tsx`：单个组件或 feature 的用户交互。
* `router.<flow>.test.tsx`：auth gate、navigation、task routes 等跨页面行为。
* `client.test.ts`：API client 请求和错误映射。

当一个测试文件超过约 200 行，并且包含 2 个以上独立用户流程时，应拆分；如果只是同一流程的多个边界用例，可以保留在同一文件。

---

## 禁止模式

* 在 JSX 中直接构造 API payload。
* 在组件里复制 schema 已经表达的业务校验。
* 用 `any` 或大段类型断言修补表单值与 DTO 不匹配。
* 软警告只显示文案但没有显式确认字段。
* 只靠端到端/路由测试覆盖所有表单校验。
* 测试纯 CSS、静态页面结构或快照，代替业务行为断言。
