# 前端设计方向

## 背景

taskdaemon 是一个长期使用的任务调度与执行管理工具，不是营销站点。界面应服务于高频查看、配置、排错和运维判断，优先考虑清晰、低干扰、高信息密度和可访问性。

## 资料来源

* `ui-ux-pro-max` design system 查询：
  * `task scheduler daemon admin dashboard developer tool operations monitoring`
  * `devops admin dashboard operations console minimal dense professional`
* `ui-ux-pro-max` React / shadcn stack guidelines。
* 参考项目：`D:\coding\Projects\frontend\vibe-coding\vote-system`

## 设计原则

* 设计定位：运维/开发工具型管理台，而不是 landing page。
* 信息架构：任务、执行历史、运行状态、告警/提示要可快速扫描。
* 视觉风格：克制、专业、低装饰，避免营销式 hero、夸张渐变和大面积装饰卡片。
* 密度：桌面端可适度高密度，适合长期使用；移动端保证可读和可操作。
* 可访问性：表单错误需要 `role=alert` 或等价语义；对话框需要正确 focus 管理；键盘可操作。
* 动效：只用于状态反馈和上下文过渡，避免大幅动画干扰运维判断。
* 图标：统一使用 lucide-react，不用 emoji 作为 UI 图标。

## 推荐视觉方向

### Layout

* App shell：左侧 sidebar + 顶部状态/操作栏 + 主内容区。
* 主内容区优先使用表格、详情抽屉、表单面板，不做卡片套卡片。
* 任务列表默认是核心首页，不做营销首页。
* Desktop 与 Web 使用同一套页面；Desktop 只在通知能力上增强。

### Pages

第一轮建议页面：

* 登录/初始化页。
* 任务列表页：状态、调度、runner、下次运行、最近结果、启停、触发。
* 任务创建/编辑页或抽屉：runner 表单、cron 表单、timeout、环境变量、工作目录。
* 执行历史页/任务详情中的历史标签：状态、触发来源、耗时、退出码、stdout/stderr 摘要。
* 运行中任务状态区：可取消、可查看实时/最近状态。

第二轮页面：

* 通知中心。
* 设置页。
* 备份模板。
* 高级 runner 配置。

### Color

推荐以浅色专业管理台为默认：

* Background: `#F8FAFC`
* Surface: `#FFFFFF`
* Text: `#0F172A`
* Muted text: `#475569`
* Border: `#E2E8F0`
* Primary: `#2563EB`
* Success/run: `#16A34A`
* Warning: `#D97706`
* Danger: `#DC2626`

可以保留深色模式作为后续增强，但第一轮优先把浅色模式做稳。避免大面积深蓝/紫色渐变，避免单一色系占满界面。

### Typography

* 主字体：Inter 或系统 sans-serif。
* 代码/日志/cron 表达式：JetBrains Mono 或等宽系统字体。
* 管理台内标题尺寸克制，避免 hero 级大字。

### Components

* Table：任务列表、执行历史使用表格；移动端用横向滚动或紧凑卡片替代。
* Form：React Hook Form + Zod；字段错误要明确且可访问。
* Dialog/Sheet：编辑任务、确认危险操作、查看长输出。
* Tooltip：解释 cron 秒字段、runner 细节、状态图标。
* Badge：任务状态、runner 类型、执行结果。
* Tabs：任务详情内切换概要/历史/配置。
* Toast：保存、触发、取消等短反馈。

## 关键交互

* 高频 cron 属于软警告：前端提示风险，用户勾选/点击确认后可提交。
* running 任务显示取消按钮；取消需要确认。
* 重叠触发的 `skipped` 状态要能在历史中被看见，而不是静默丢弃。
* 表单提交、触发、取消等异步操作需要 loading 状态和成功/失败反馈。
* 执行输出默认截断展示，支持展开查看已保存内容。

## 避免项

* 不做 landing page/hero 作为第一屏。
* 不用 emoji 作为功能图标。
* 不用过度装饰的渐变、玻璃拟态或大面积暗色氛围背景。
* 不用 div grid 假装表格展示复杂数据。
* 不让 hover/active 状态改变布局尺寸。
* 不只用颜色表达状态，状态文字和图标也要存在。
