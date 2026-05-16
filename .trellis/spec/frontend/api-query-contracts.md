# API 与 Query 契约

> 前端所有后端通信都经过 API client 和 feature query hook 边界。

---

## 概览

当前前端使用手写轻量 API client。`src/lib/api/client.ts` 负责 fetch、Cookie、JSON envelope 解包和错误映射；`src/lib/api/types.ts` 负责后端 DTO 类型；feature 内的 `*.queries.ts` 负责 TanStack Query key、query hook、mutation hook 和缓存失效。

任何 API 契约变化都应同时检查类型、client、query hook、schema/payload mapper、组件交互和测试。

---

## 分层职责

* `src/lib/api/types.ts`：后端 DTO、请求 payload、API envelope、稳定 union 类型。
* `src/lib/api/client.ts`：唯一业务 fetch 边界，统一 `credentials: "include"`、JSON body、envelope 解包、`ApiClientError`。
* `src/features/<domain>/*.queries.ts`：query key、queryFn、mutationFn、缓存失效、轮询策略。
* `src/features/<domain>/*.schema.ts`：表单输入校验、默认值、form -> payload 映射。
* 组件：展示 loading/empty/error/data，触发 hook action，不直接拼业务 API 请求。

---

## API Client 规则

* 业务代码不得直接调用 `fetch("/api/...")`；新增 endpoint 先加到 `apiClient`。
* `apiClient` 方法名用业务动作，例如 `listTasks`、`triggerTask`、`authStatus`。
* 成功响应统一解包 `ApiEnvelope<T>` 并返回 `data`。
* 非 2xx 响应统一抛出 `ApiClientError`，包含 `status`、稳定 `code`、`details`、`traceId`。
* 解析错误响应失败时使用保守 fallback，例如 `unknown_error` 和响应状态文本。
* API message 不作为 UI 稳定分支；组件只能根据 `ApiClientError.code` 或泛化失败态分支。
* 认证相关请求默认携带 Cookie；不要在 feature 里重复写 credentials。

---

## DTO 与类型

* 后端 DTO 类型放在 `src/lib/api/types.ts` 或未来生成目录。
* 执行状态、runner 类型、触发来源等稳定枚举使用 `as const` 数组推导 union。
* 时间字段保持 API 返回的 ISO 字符串，在展示层用格式化函数处理。
* UI view model 不污染 DTO；格式化后的文本、badge variant、派生状态放在 feature 工具函数。
* 手写 client 阶段，对关键跨层边界保持最小运行时校验意识；如果后端字段变更，至少更新 API client 测试和相关 schema/组件测试。

---

## Query Key

* query key 在 feature 附近集中定义，例如 `tasksKeys`。
* key 必须稳定、可组合，避免在组件里临时拼数组。
* 列表、详情、执行历史等不同数据要有明确层级，例如：

```ts
export const tasksKeys = {
  all: ["tasks"] as const,
  runs: (taskId: number) => ["tasks", taskId, "runs"] as const,
};
```

* 如果引入筛选、分页或搜索参数，key 中必须包含经过校验和规范化的参数对象。
* 禁止多个 feature 为同一后端资源定义互不兼容的 key。

---

## Mutation 与缓存失效

* mutation 成功后必须显式维护相关 query cache。
* 创建、更新、启停任务后刷新任务列表。
* 触发或取消任务后刷新任务列表和对应执行历史。
* 删除任务后刷新任务列表，并移除该任务执行历史 cache。
* 当 action 同时影响多个页面，例如状态页、任务页、历史页，优先把失效逻辑放在同一个 mutation hook 内。
* 只有在能保持一致性时才用 `setQueryData` 局部更新；否则使用 invalidate。

---

## 轮询与刷新

* 默认不常驻轮询；只有存在 running task/run 或页面明确需要实时状态时开启。
* 轮询条件放在 query hook，不放在组件里散落定时器。
* 轮询间隔需要集中常量化；不要在多个文件写死同一个数字。
* 没有任务、没有 running 项或 query disabled 时停止轮询。
* 手动刷新按钮调用 query 的 `refetch`，但不改变全局轮询策略。
* 后续若要监听页面可见性，逻辑应集中在 query hook 或共享 helper。

---

## 错误处理与展示

* query hook 不吞错误；组件负责展示页面级或局部错误。
* 字段级错误来自 Zod schema 或稳定 API error code。
* 页面级错误需要给出可恢复路径，例如刷新、重新登录或检查服务状态。
* `traceId` 可用于调试展示或复制，但不能暴露 token、密码、Cookie 或完整请求体。
* 破坏性操作失败时保持 UI 状态可恢复，不从 cache 中提前删除数据，除非有回滚逻辑。

---

## API 变更清单

新增或修改 endpoint 时检查：

* `src/lib/api/types.ts` DTO/payload 是否更新。
* `src/lib/api/client.ts` 是否新增/更新方法。
* feature `*.queries.ts` 是否更新 query key、mutation、失效逻辑。
* 相关 `*.schema.ts` 是否更新默认值和 payload mapper。
* 组件是否处理 loading、empty、error、disabled。
* API client 测试是否覆盖成功、失败、traceId 和错误码。
* 涉及表单或交互的组件测试是否覆盖关键分支。
* 跨后端契约变更是否同步后端 spec 或任务 PRD。

---

## 禁止模式

* 组件直接 `fetch` 业务接口。
* 组件直接解析 API envelope 或错误 body。
* 在多个组件里重复写 runner/status/trigger 字符串。
* mutation 成功后只依赖页面刷新让数据“碰巧正确”。
* query key 在组件中临时创建，导致缓存无法统一失效。
* 依赖 API `message` 做稳定逻辑分支。
