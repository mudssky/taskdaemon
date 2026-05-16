# Type Safety

> Type safety patterns in this project.

---

## Overview

前端使用 TypeScript 严格约束业务数据形状，并用 Zod 做运行时输入校验。当前 API 类型由轻量手写 client 维护；未来接入 OpenAPI/类型生成时，也必须保持 DTO、表单值和 UI view model 的边界清晰。

---

## Type Organization

* 后端 API DTO 类型放在 `src/lib/api` 或生成目录中。
* feature 内部类型放在 `src/features/<domain>` 附近，例如 `task.types.ts`。
* 表单值类型由 Zod schema 推导，使用 `z.infer<typeof schema>`。
* UI 专用 view model 与 API DTO 分开，例如执行状态展示字段、格式化后的耗时文本。
* 共享常量使用 `as const` 或枚举对象，并从常量推导 union type。
* Vite 环境与静态资源类型声明放在 `src/vite-env.d.ts`。升级 TypeScript 或 Vite 后，先运行 `pnpm typecheck` 验证 CSS、图片等 side-effect import 仍有声明覆盖。

---

## Validation

* Zod 是前端运行时校验标准。
* 任务表单、runner 配置、cron 输入、登录表单都必须有 schema。
* cron 校验需要表达硬错误和软警告：硬错误阻止提交，软警告需要显式确认后允许提交。
* API 响应如果来自可信生成 client，可依赖生成类型；手写 client 阶段对关键边界做最小运行时校验。
* 从 URL search params 读取的值必须校验并提供默认值。

---

## API Types

* API 错误响应按稳定错误码处理，不在前端解析错误 message。
* `Task`、`TaskRun`、`RunnerConfig` 等 DTO 应和后端命名保持一致。
* 时间字段从 API 进入前端时保持 ISO 字符串或明确类型，在展示层用 dayjs 格式化。
* 执行状态使用 union type，例如 `success | failed | timeout | cancelled | skipped | running`，不要用裸字符串到处传。
* 触发来源使用 union type，例如 `cron | manual | cli | api`，后端新增来源时前端 typecheck 应提醒补 UI。

---

## Common Patterns

```ts
const runStatuses = ["running", "success", "failed", "timeout", "cancelled", "skipped"] as const;
type RunStatus = (typeof runStatuses)[number];
```

* 用 discriminated union 表达不同 runner 配置，例如 `shell`、`python`、`typescript`。
* 用纯函数把 DTO 映射成表单默认值，反向提交时再映射成 API payload。
* 用 exhaustive check 处理任务状态、runner 类型和触发来源，避免新增类型后 UI 静默漏分支。

---

## Forbidden Patterns

* 禁止在业务代码中使用 `any` 逃避类型检查；确实来自未知输入时使用 `unknown` 并校验。
* 禁止用大量 `as SomeType` 修补 API 或表单类型不匹配；修正 schema 或 DTO。
* 禁止把所有 API 返回都定义成 `Record<string, unknown>`。
* 禁止把日期对象、ISO 字符串、展示文本混用同一个字段。
* 禁止在组件里手写重复字符串状态；使用集中常量或类型。
