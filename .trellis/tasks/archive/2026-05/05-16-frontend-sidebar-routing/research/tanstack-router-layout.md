# TanStack Router Layout 调研

## 结论

项目已经使用 `@tanstack/react-router`，不需要切换路由库。当前问题更像是 route component 划分不清：`/`、`/tasks`、`/runs` 都渲染 `TaskDashboard`，因此侧边栏点击虽然会改 URL，但页面内容没有分离。

## 官方模式

TanStack Router v1 的常见结构是：

* root route 负责全局布局和导航。
* root layout 内放 `<Outlet />` 渲染子路由。
* 子路由分别提供自己的 component。
* `<Link to="...">` 用于导航，`activeProps` / `activeOptions` 控制激活态。

## 映射到本项目

当前 `AppShell` 已经包含 sidebar、topbar 和 `<Outlet />`，适合作为 root layout。`ProtectedApp` 同时承担认证分流和布局，仍可保留，但认证通过后的子页面应拆成独立 route component：

* `/`：运行状态页，应展示概览状态，不应包含任务表单和历史完整面板。
* `/tasks`：任务管理页，展示任务列表和创建/编辑表单。
* `/runs`：执行历史页，展示运行历史。

## 风险

如果继续让多个路由共用 `TaskDashboard`，用户会感觉 sidebar 没生效；如果只改导航高亮而不拆页面，问题不会真正解决。

## 建议

使用现有 TanStack Router，先做轻量页面拆分：

1. 保留 `AppShell` 作为布局。
2. 拆出 `TaskManagementPage` 与 `RunHistoryPage`。
3. 将 `/` 改成 `StatusPage` 或轻量 dashboard。
4. 为 router 增加组件测试，验证导航点击后只出现目标页面内容。
