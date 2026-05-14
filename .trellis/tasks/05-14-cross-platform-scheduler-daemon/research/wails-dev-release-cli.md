# Wails 开发/发布/CLI 约束调研

## 背景

用户希望项目满足：

* 开发期前端和后端可以单独启动。
* 发布期前端资源打包进最终二进制。
* 同一个二进制还要支持 CLI 执行。

## 资料来源

* Context7: `/websites/v3_wails_io`
* 查询主题：`前后端需要能单独启动，然后最后发布的时候还是把前端打包到二进制，并且二进制也支持cli执行`

## 关键结论

* Wails 的定位匹配“Go 后端 + Web 前端 + 单二进制发布”，适合作为桌面壳和前端资源打包层。
* Wails 构建系统支持前端构建步骤、构建变量、跳过部分构建等能力，说明可以把前端构建作为发布流水线的一部分管理。
* 为了同时支持 CLI，主程序入口不应直接无条件启动 Wails 窗口；应先解析启动模式，再选择进入 `desktop`、`serve` 或具体 CLI 子命令。

## 映射到本项目

推荐把项目拆成“核心能力 + 多入口”：

* `internal/core`：任务调度、任务执行、通知、数据库访问、配置等核心能力。
* `internal/api`：Fiber HTTP API，供前端、Webhook、本机调用。
* `cmd/taskdaemon`：唯一发布二进制入口，按子命令切换模式。
* `apps/web`：Vite/React 前端，开发期独立启动，发布期由 Go/Wails 嵌入。

推荐启动模式：

* `taskdaemon serve`：只启动后端 API、调度器和静态资源服务；开发期前端可单独跑 Vite 并代理 API。
* `taskdaemon desktop` 或无参数默认：启动 Wails 桌面窗口，并复用同一套 Go 核心和 API 能力。
* `taskdaemon run <task>` / `taskdaemon trigger <task>`：CLI 触发任务或执行管理命令，不启动桌面窗口。

## 风险与待确认

* Wails 版本需要后续确定。v3 文档显示构建系统更灵活，但具体稳定性、生态和当前推荐版本需要在实施前再确认。
* 如果 Wails 桌面模式和 Fiber HTTP API 同时存在，需要明确端口、鉴权、CSRF/本机访问边界，避免桌面本地 API 被局域网误用。
