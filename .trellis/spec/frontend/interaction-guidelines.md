# 交互状态、错误恢复与可访问性

> taskdaemon 前端在 loading、empty、error、pending、disabled、确认对话框和调试信息上的长期约定。

---

## 概览

前端页面是工具型管理台，交互状态需要优先保证可恢复、可扫描和可解释。页面组件负责展示 query/mutation 状态，API client 和 query hook 负责提供稳定错误对象与缓存边界；不要让组件解析后端 envelope，也不要把一次性文案或临时 DOM 结构写成长期规则。

---

## 状态展示边界

* `loading`：用于首屏或局部数据仍不可用的状态。首屏 loading 可以替换主体内容；局部 loading 应保留页面上下文，避免整个页面闪烁。
* `empty`：用于请求成功但没有可展示数据，例如暂无任务或暂无执行历史。empty 文案应告诉用户当前状态，而不是暗示错误。
* `error`：用于请求失败或动作失败。页面级 error 必须给出可恢复方向，例如刷新、重新登录、检查服务状态或回到列表。
* `pending` / `isSubmitting`：用于用户已经触发动作且请求未结束。触发按钮需要 `disabled` 或明确 busy 行为，避免重复提交。
* `disabled`：用于动作当前不允许执行，例如 running 任务不可删除。禁用态必须来自业务状态或请求状态，不要只用样式表达。
* `success feedback`：只有当成功结果不会被数据刷新自然表达时才额外提示。任务创建、更新、触发等动作优先通过跳转、列表刷新或状态变化反馈。

---

## 错误分层

* 字段级错误来自 Zod schema、表单状态或稳定 API error code，展示在对应字段附近。
* 页面级错误来自 query 失败，展示在页面主体或相关 panel 内，文案描述用户能采取的下一步。
* mutation 错误展示在动作所在区域，例如表单提交失败显示在表单附近；不要让行内动作失败导致数据直接消失。
* 组件不能依赖 API `message` 做稳定逻辑分支；稳定分支使用 `ApiClientError.code`，泛化文案可使用固定中文提示。
* API client 和 query hook 不吞错误。组件决定显示页面级、局部或字段级错误，hook 只维护请求和缓存一致性。
* 破坏性 mutation 在服务端成功前不要从 UI/cache 中提前删除数据，除非实现了明确回滚。

---

## 恢复动作

* 列表、概览、历史等页面级 error 应尽量提供刷新入口，或在已有刷新按钮明显可用时保持其可点击。
* 认证失败、认证状态查询失败应引导用户重新登录、检查服务状态或等待后端恢复。
* 编辑页找不到对象时使用 empty 状态；ID 不合法、加载失败使用 error 状态。
* 表单提交失败后保留用户输入，不重置表单，不跳转离开当前页面。
* 自动轮询失败不应无限制造视觉噪音；页面可显示稳定 error 状态，由用户手动刷新或后续 query 恢复。

---

## 破坏性确认

* 删除任务、取消 running 任务、高频/秒级 cron 确认等风险操作必须有明确确认路径。
* 破坏性确认优先使用 `AlertDialog` 或项目统一 Confirm 组件，不使用 `window.confirm` 作为长期实现。
* `AlertDialog` 必须包含 `AlertDialogTitle` 和 `AlertDialogDescription`；文案需要说明对象名称和主要后果。
* 确认和取消路径都要可达，取消不能触发 mutation。
* 对话框打开后，确认按钮触发 mutation；mutation 完成前如有重复提交风险，应禁用确认按钮或关闭后由行级 busy 态兜底。
* 破坏性操作测试需要覆盖取消路径和确认路径，避免只测确认。

---

## 可访问性基础

* 页面和 panel 使用 `aria-labelledby` 关联可见标题；没有可见标题时再使用 `aria-label`。
* 图标按钮必须有可访问名称，例如 `aria-label` 或清晰文本；装饰性图标使用 `aria-hidden="true"`。
* Dialog、AlertDialog、Popover、Select 等交互优先使用 Radix/shadcn 原语，保留键盘导航、焦点管理和 Escape 行为。
* 状态不能只靠颜色表达；badge、错误、警告都需要文字。
* 表单字段必须有 label。字段错误应靠近字段展示；后续若引入复杂表单组件，需要补齐 `aria-invalid` 和错误描述关联。
* 日志、stdout/stderr、错误摘要等文本必须可选择复制，不渲染成图片。

---

## 文案与本地化

* 当前 UI 文案以中文为主；技术词例如 cron、runner、traceId 可保留英文原词。
* 业务状态、runner 类型、触发来源等展示文本应集中映射，避免多个页面重复写不同中文。
* API `message` 不作为长期 UI 文案来源；用户可见文案优先由前端根据稳定 code 或通用失败场景提供。
* 错误文案应避免暴露内部路径、token、Cookie、完整请求体、密码或命令密钥。

---

## TraceId 与调试信息

* `traceId` 用于排查跨层问题，可以在高级错误详情、复制按钮或日志入口中展示。
* 默认错误文案不要求显示 `traceId`；当失败需要用户反馈给开发者或管理员时，可附带 `traceId`。
* 展示或复制调试信息时，只包含错误码、HTTP 状态、traceId 和安全摘要。
* 前端不记录敏感信息到 `console`，包括 token、密码、Cookie、完整请求体、环境变量和命令输出。
* 临时调试 `console.log` 不应提交；确需长期日志时先封装受控 logger，并明确生产环境行为。

---

## 禁止模式

* 用 `window.confirm` 承载长期破坏性确认。
* 只靠颜色表达状态或风险。
* 点击后仍可重复提交同一 mutation。
* 表单提交失败后清空用户输入。
* 在组件中解析 API envelope 或错误 body。
* 依赖 API `message` 做稳定分支。
* 把 token、密码、Cookie、完整请求体或敏感命令输出放进 UI 错误详情或控制台。
