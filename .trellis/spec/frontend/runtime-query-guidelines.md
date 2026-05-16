# 运行时 Query 与轮询规范

> 运行状态、任务列表、执行历史等会随时间变化的页面，需要用可控的短轮询和明确的缓存失效保持一致。

---

## 适用范围

本规范适用于 `apps/web` 中由 TanStack Query 管理、且数据会在用户不主动提交表单时变化的 server state，例如任务运行状态、执行历史、状态概览和未来实时任务视图。

一次性产品范围、具体页面控件清单和某个阶段的临时刷新策略仍放在 `.trellis/tasks/`，不要写入本长期规范。

---

## 职责边界

* API client 只负责请求、响应 envelope 解包、错误映射和 DTO 返回。
* query hook 负责 query key、queryFn、enabled、轮询策略、轻量派生状态和缓存失效入口。
* mutation hook 负责成功后显式 invalidate、remove 或一致的 `setQueryData`。
* 组件负责 loading/empty/error/data 展示、手动刷新按钮状态、toast/dialog/navigation 等 UI 副作用。
* query hook 不直接显示 toast、不打开 dialog、不导航、不写入表单状态，也不读取页面布局细节。

允许放在 query hook 内的派生值必须轻量且与数据获取强相关，例如是否存在 running 项、规范化后的筛选参数、是否启用 query、当前页面是否需要短轮询。复杂展示文本、badge variant、表格行 view model 和用户可见的状态组合应放在 feature 工具函数或组件层。

---

## 自动轮询准入

默认不常驻轮询。只有满足以下任一条件时，才允许开启自动 refetch：

* 当前页面展示的数据会被后端异步改变，且用户需要在页面内及时看到结果。
* 数据中存在 running task 或 running run。
* 页面本身被定义为运行态/实时态页面，例如状态概览或未来实时任务视图。

新增轮询前先确认是否可以通过 mutation 成功后的 invalidate、页面重新进入时 refetch、窗口重新聚焦 refetch 或手动刷新满足需求。轮询是运行时成本，不应作为“数据可能过期”的默认答案。

---

## 停止条件

自动轮询必须有明确停止条件。以下情况应停止或不启动轮询：

* 没有 running task/run。
* 任务列表为空，且页面没有明确的实时空状态需求。
* query `enabled` 为 false，例如未选择 taskId、未登录或路由参数无效。
* 页面不可见或不在当前路由作用域内。
* 组件卸载后不再保留自定义定时器；优先使用 TanStack Query 的 `refetchInterval`，避免手写 `setInterval`。
* 查询已经进入不可恢复错误态，且继续轮询只会制造重复错误；此时优先展示手动恢复入口。

如果未来需要监听页面可见性，逻辑应集中在 query hook 或共享 helper 中，不能让每个页面各自读取 `document.visibilityState` 后拼装轮询条件。

---

## 间隔治理

轮询间隔必须常量化，放在 feature query 文件附近或共享运行时配置中，例如 `TASKS_RUNNING_REFETCH_INTERVAL_MS`。多个页面复用同类运行态数据时，应复用同一常量或从同一个配置导出。

禁止在多个组件或 hook 中散落 `3000`、`5000` 这类毫秒字面量。新增间隔前需要搜索现有常量，确认是否能复用；如果确实需要不同频率，常量命名必须表达业务场景而不是只表达数字。

短轮询频率应保守。任务运行状态这类后台调度状态优先使用秒级间隔；只有用户正在观察且运行态存在时才提高刷新频率。

---

## 手动刷新与自动刷新

手动刷新按钮调用 query 的 `refetch`，但不改变 query hook 的自动轮询策略。手动刷新是用户动作，自动轮询是运行时策略，两者的 UI 状态要分开处理。

* 首次加载使用页面级 loading 或局部 loading。
* 自动 refetch 不应清空已有数据，也不应让页面回到首次加载骨架屏。
* 手动刷新可以让按钮 disabled、显示旋转图标或局部“刷新中”状态，但不要阻塞无关操作。
* 如果同一个 query 正在自动 refetch，手动刷新按钮状态可以复用 `isFetching`，但文案和交互不能暗示用户触发了全局同步。
* mutation 提交中的 busy 状态独立于刷新状态；不要把任务启停、触发、取消的 pending 状态和列表后台刷新混在同一个布尔值里。

页面需要区分 `isLoading`/`isPending`、`isFetching`、mutation `isPending` 和手动 refetch promise。只有首次没有可展示数据时才使用空白 loading；已有数据的后台刷新应保持内容稳定。

---

## 缓存复用与失效

同一后端资源必须共享同一组 query key。状态页、任务页、历史页如果读取任务列表，应复用 `tasksKeys.all`；读取某个任务的执行历史，应复用 `tasksKeys.runs(taskId)`。禁止为同一资源在不同 feature 中创建互不兼容的 key。

mutation 成功后要从动作影响面反推失效范围：

* 创建、更新、启停任务后刷新任务列表。
* 触发或取消任务后刷新任务列表和对应执行历史。
* 删除任务后刷新任务列表，并移除该任务执行历史 cache。
* 影响状态页、任务页和历史页的动作，失效逻辑放在同一个 mutation hook 内，不依赖页面各自补救。

只有在能证明局部更新不会破坏列表、详情、状态页和历史页一致性时，才使用 `setQueryData`。否则优先 invalidate，让服务端成为最终事实来源。

---

## 禁止模式

* 在组件中手写常驻 `setInterval` 请求业务数据。
* 没有 running 数据仍保持高频轮询。
* 在 query hook 中弹 toast、改路由、打开 dialog 或写入表单。
* 为状态页、任务页、历史页分别定义同一资源的 query key。
* 手动刷新时清空已有列表，造成页面闪烁。
* 把自动 refetch、手动刷新和 mutation pending 合成一个泛化的 `loading`。
* 在多个文件写死同一个轮询毫秒值。

