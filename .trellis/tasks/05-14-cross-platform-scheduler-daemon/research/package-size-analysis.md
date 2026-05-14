# 打包体积分析

## 背景

taskdaemon 的关键价值之一是跨平台、低占用、单二进制。技术栈需要避免重型运行时、外部服务依赖和不必要的大型前端包。

## 资料来源

* Context7: `/websites/v3_wails_io`
* Context7: `/websites/pkg_go_dev_modernc_org_sqlite`
* Context7: `/swaggo/swag`
* 本项目已确认技术栈与 `vote-system` 桌面打包复盘经验。

## 当前栈体积判断

### 有利因素

* Wails 使用系统 WebView，不内置 Chromium，比 Electron 路线更符合低体积目标。
* Wails 构建会把前端资源嵌入 Go 二进制，适合单文件发布。
* Wails release 构建可 strip debug symbols，并可配合 `-trimpath` 进一步减少体积。
* Vite + React 是静态前端，不引入 Next.js/Nuxt 这类桌面打包不友好的全栈框架。
* Gin、Koanf、Cobra、robfig/cron、slog 等 Go 依赖对体积影响相对可控。
* 前端 Tailwind/Biome/TypeScript/Vitest 多数是开发期工具，不进入运行时 bundle。

### 主要风险

* SQLite driver：pure-Go SQLite 免 cgo、跨平台更方便，但通常会增加 Go 二进制体积；cgo SQLite 体积可能更小但构建/分发复杂度更高。
* Ent 代码生成会增加代码量，但换来类型安全；需要控制 schema 和生成代码边界。
* `swaggo/swag` 应作为生成工具；Swagger UI/docs route 需要配置开关控制，默认可关闭，开发或自托管需要时显式开启。
* 前端依赖如果引入 Recharts、Monaco、Framer Motion、完整图标包、重型日期库，会明显增加 JS bundle。
* Testify、testcontainers-go、golangci-lint、Vitest、Biome、Testing Library 都应是开发/测试依赖，不进入最终运行时。

## 建议调整

### 保持当前方向

* 保持 Wails + Vite + React + Go 单二进制方向。
* 保持 Gin 替代 Fiber；二者对最终体积不是主矛盾，Gin 生态和团队熟悉度更重要。
* 保持 Ent；类型安全收益大于代码生成体积成本。
* 保持 TanStack Router/Query、Radix、React Hook Form、Zod，它们对管理台体验有明确价值。

### 第一轮避免引入

* 不引入图表库，除非第一轮确实需要图表。
* 不引入 Monaco Editor；cron/脚本配置先用普通输入框/textarea。
* 不引入 Framer Motion；动效用 CSS transition 即可。
* 日期处理使用 dayjs，按需引入插件，避免引入重型日期库或不必要 locale/plugin。
* 不引入日志虚拟化/终端模拟器库；第一轮只展示截断 stdout/stderr。

### 构建策略

* Release 构建使用 `-ldflags="-s -w"` 和 `-trimpath`。
* 前端开启 bundle analysis 作为后续质量门槛。
* 前端图标使用 lucide-react 的 named import，避免整包引入。
* Swagger 文档通过配置开关启用/关闭，避免生产默认暴露 API 文档。
* testcontainers-go 只放测试包路径，避免被运行时代码引用。

## 待 spike

* Ent + SQLite driver 选择：
  * pure-Go SQLite 优先，降低跨平台构建复杂度。
  * 需要验证 Ent 兼容性、体积影响和性能。
  * 如果 pure-Go 体积明显不可接受，再评估 cgo SQLite。
* Wails 版本与 CLI 共存方式：
  * 验证 release binary 体积。
  * 验证 CLI-only 模式是否能避免初始化 Desktop 相关运行逻辑。
* Swagger 文档：
  * 验证 docs route 如何通过配置开关启用/关闭。
  * 验证关闭时不会注册 Swagger UI 路由。

## 决策

当前技术栈不需要大改。重点是约束依赖进入时机和构建方式：

* 继续使用 Wails + Go + Vite/React。
* 第一轮控制前端依赖，不加入图表、编辑器、动画等重型包；日期处理使用 dayjs。
* 开发/测试/文档工具不进入运行时路径。
* SQLite driver 和 Wails release 体积需要在项目骨架子任务中做一次实测。
