# 路由认证与平台边界规范

> 路由负责页面入口和可分享状态，认证守卫负责入口分流，Web/Desktop 差异必须被封装在边界层。

---

## 适用范围

本规范适用于 `apps/web` 的 TanStack Router route tree、受保护入口、URL 参数、页面标题/导航，以及 Web 与 Wails Desktop 共用前端时的平台能力边界。

单个页面的控件清单、阶段性导航项和某个 Desktop 能力的完整产品需求应保留在 `.trellis/tasks/`，不要写入本长期规范。

---

## Route Tree 与页面入口

* `src/routes` 只做 route 定义、受保护入口组合、loader/action 绑定和页面组件挂载。
* 页面组件放在 `src/features/<domain>`，不要在 route 文件里塞复杂业务逻辑。
* 根路由负责装配受保护入口；当前模式是 `ProtectedApp` 查询认证状态后决定展示初始化、登录或业务 shell。
* 顶级导航目标必须对应清晰页面，例如 `/`、`/tasks`、`/runs`。
* 任务创建/编辑等复杂流程使用独立路由，例如 `/tasks/new` 和 `/tasks/$taskId/edit`，列表页只提供入口。
* 不为了个人偏好创建第二套 route tree 组织方式；新增路由沿用当前 TanStack Router 手写 route 定义模式，直到项目统一迁移。

页面标题和导航状态可以由 layout 根据 pathname 派生，但复杂到需要业务上下文时应下沉到 route/page 配置或 feature 工具函数，避免在 layout 中维护大量业务分支。

---

## 认证守卫

认证状态查询应集中在受保护入口或 app shell 附近，不在每个业务页面重复判断。当前认证分流顺序保持为：

1. 查询 `GET /api/auth/status`。
2. 未初始化时展示首次管理员设置。
3. 已初始化但未登录时展示登录。
4. 已认证时进入业务 `AppShell` 并渲染子路由。

`/api/auth/status` 是首次分流的基础 query，避免只依赖 `/api/auth/me` 的 401，把“未初始化”和“已初始化但未登录”折叠成同一种状态。登录、初始化管理员和退出登录 mutation 成功后必须 invalidate `authKeys.status`，如有 session 详情缓存也同步 invalidate。

业务页面默认假设已经通过受保护入口，不重复渲染登录页或初始化页。页面级认证错误只展示后端不可用、会话失效后的恢复路径或局部错误态。

---

## URL 参数与 Search Params

URL state 用于筛选、分页、选中 tab、详情 ID 和可分享页面状态。进入组件前必须完成类型校验、默认值处理和非法值兜底。

* path params 进入业务逻辑前转换成明确类型，例如正整数 taskId。
* search params 使用 TanStack Router 的校验能力、Zod schema 或 feature parser 统一规范化。
* 非法 URL 不应让页面崩溃；应展示错误态、回退默认值或导航到安全路由。
* dialog 开关默认不放 URL，除非它代表可独立访问和分享的详情状态。
* 表单内部 dirty/valid/临时输入不放 URL。

不要在多个组件里重复解析同一个 search param。参数增长到两个以上、或被多个页面复用时，应提取到 feature 附近的 parser/schema，并覆盖纯函数测试。

---

## Web/Desktop 共用边界

Web 和 Desktop 共用同一套 React 页面组件。Desktop 专属能力必须通过边界 hook、service 或 `src/lib/desktop` 一类模块封装，业务组件不直接读取 Wails 全局对象或调用 runtime API。

边界层需要提供 Web fallback：

* 能力可用时返回明确 capability 和方法。
* Web 环境不可用时返回 disabled/noop 或可解释错误。
* 组件只根据边界 hook 返回的状态展示按钮、禁用态或错误，不关心底层是 Wails、浏览器还是 mock。

只有多个页面都需要的 Desktop 能力才进入共享边界；单个 feature 的实验性能力先放在 feature 内部 service，稳定后再上移。

---

## API 网络路径

前端业务 API 统一走 `/api/...` 相对路径，并通过 `apiClient` 发送请求。Web 和 Desktop 共享这一路径假设：

* 普通 Web 开发由 Vite proxy 把 `/api` 转发到后端 API origin。
* Wails dev 模式下桌面窗口直连 Vite dev server，仍使用 `/api` 走 proxy。
* 非 dev Desktop 窗口访问后端托管的前端入口，`/api` 由同一个本机 HTTP API server 提供。
* 不把 `wails.localhost` AssetServer 当作业务 API POST/PUT/DELETE 转发层。

如需覆盖端口或 API origin，使用既有环境变量和配置入口，例如 `WAILS_VITE_PORT`、`TASKDAEMON_WEB_PORT`、`TASKDAEMON_API_ORIGIN`、`VITE_TASKDAEMON_API_ORIGIN`。不要在组件或 feature 中拼绝对 API origin。

---

## 禁止模式

* 每个页面各自调用认证 query 并重复实现登录/初始化分流。
* 在 route 文件中编写复杂业务流程、表单映射或 API payload 拼装。
* 在组件中直接读取 `window.runtime`、Wails 全局对象或 Desktop 绑定。
* 为 Desktop 和 Web 分叉两套业务组件。
* 在组件里手写 URL search param 解析并散落默认值。
* 在业务代码里直接 `fetch("/api/...")` 或拼绝对后端地址。
* 把 Desktop resource server/AssetServer 当业务 API 转发层。
