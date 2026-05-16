# Go 文件组织与拆分

> Go 文件应围绕职责、可读性和测试边界组织，而不是单纯追求固定行数。

---

## 概览

当前 Go 后端已经形成稳定包边界，但少数非生成文件承担了过多职责。拆分的目标是让维护者能快速定位行为、减少并发改动冲突，并让测试失败能指向更清晰的业务区域。

生成代码不纳入拆分判断，尤其是 `services/taskdaemon-go/internal/data/ent/`。

---

## 判断标准

优先考虑拆分的信号：

* 单个文件超过约 400 行，并且包含 3 个以上职责簇。
* 文件同时包含公共 API、业务流程、格式转换、底层工具函数和测试辅助。
* 修改一个小功能时必须阅读大量无关函数。
* 测试文件已经很难按行为块定位，或多个测试区域频繁产生冲突。

暂不拆分的情况：

* 文件虽长，但只是同一类小型声明或生成代码。
* 拆分会制造循环依赖或让读者在更多文件间跳转。
* 只有一次性需求导致的临时膨胀，且近期会删除。

---

## 拆分原则

* 先按领域职责拆，再按技术工具拆；不要按“前 300 行 / 后 300 行”机械切。
* 保持 package 不变优先，只有出现明确独立边界时才新建子 package。
* 公共类型、构造函数和核心 service 方法放在最容易被新读者找到的文件中，例如 `service.go`、`config.go`、`logger.go`。
* 私有 codec、formatter、redaction、path resolution 等辅助逻辑可独立成同 package 文件。
* 测试文件跟随行为拆分，例如 `service_lifecycle_test.go`、`cron_validation_test.go`、`middleware_logging_test.go`。
* 每次拆分保持行为等价，优先做纯移动和命名整理；功能变更另起提交或单独任务。

---

## 当前候选

### `internal/scheduler/service.go`

当前混合 cron 校验、任务 CRUD、调度注册、run lifecycle、runner JSON 编解码和转换工具。推荐逐步拆为：

* `service.go`：`Service`、`Options`、构造函数和公开业务方法入口。
* `cron_validation.go`：cron 字段、timezone、高频/秒级 warning。
* `task_lifecycle.go`：创建、更新、启停、删除任务。
* `run_lifecycle.go`：trigger、cancel、begin/finish/finalize run。
* `registration.go`：gocron scheduler 初始化、注册、移除、reconcile。
* `runner_config_codec.go`：runner config 与 Ent JSON 字段互转。

### `internal/config/config.go`

当前混合配置结构、默认值、路径解析、加载合并、env 映射和 scalar 解析。推荐拆为：

* `types.go`：配置结构体与 `LoadOptions`。
* `defaults.go`：默认配置和默认 DSN。
* `paths.go`：默认路径、项目路径、本地覆盖路径解析。
* `load.go`：加载顺序、koanf 合并、文件读取。
* `env.go`：`TASKDAEMON_` 映射、别名和标量解析。

### `internal/logging/logger.go`

当前混合 logger 工厂、文件 handler、fanout handler、pretty handler、HTTP 访问日志格式化和值格式化。推荐拆为：

* `logger.go`：对外构造、关闭、level 解析。
* `file_handler.go`：日志文件可写性、lumberjack handler。
* `fanout_handler.go`：多 handler 分发。
* `pretty_handler.go`：console pretty handler。
* `format.go`：HTTP 单行日志和值格式化。

### `internal/httpapi/middleware.go`

当前混合 trace、响应选项、请求日志、body capture、脱敏、recovery 和 session 校验。推荐拆为：

* `middleware.go`：通用 middleware 注册入口或轻量组合。
* `trace_middleware.go`：trace id 注入与响应选项。
* `request_logger.go`：访问日志主流程。
* `body_capture.go`：有限 body 捕获与恢复。
* `redaction.go`：JSON/text 脱敏。
* `auth_middleware.go`：session 校验。
* `recovery_middleware.go`：panic recovery。

### `internal/cli/command.go`

当前混合 Cobra 根命令、config 输出、daemon task actions、YAML/JSON 输出和 session token context。推荐拆为：

* `command.go`：根命令和全局 flag。
* `config_commands.go`：config path/show/validate/reload。
* `task_commands.go`：task trigger/cancel/runs。
* `output.go`：YAML/JSON 输出和配置脱敏。
* `session.go`：session token context 与解析。

### `internal/app/app.go`

当前混合应用装配、HTTP server、配置重载、daemon HTTP client、migration 和地址工具。推荐拆为：

* `app.go`：`App` 类型、构造、依赖注入。
* `serve.go`：HTTP server 生命周期。
* `reload.go`：运行时配置重载和 daemon reload 调用。
* `daemon_client.go`：CLI 到 daemon HTTP API 的请求封装。
* `migration.go`：schema migration。
* `address.go`：监听地址、端口冲突判断。

---

## 拆分流程

1. 先用 `rg -n "^(type|func|const|var)\\b" <file>` 列出职责簇。
2. 画出目标文件清单，确认不会引入循环依赖。
3. 只移动代码，不改行为；移动后运行 gofmt。
4. 按被拆职责调整测试文件名和测试分组。
5. 运行 `pnpm test:go`；涉及 lint/vet 时运行 `pnpm vet:go`。
6. 在提交说明中写清楚“纯拆分”还是“拆分 + 行为变更”。

---

## 评审清单

* 新文件名是否能表达职责，而不是只表达实现细节。
* `service.go`、`config.go`、`logger.go` 等入口文件是否仍保留新读者需要的主线。
* 私有辅助函数是否放在最接近调用者的位置。
* 测试是否仍按行为断言，而不是只跟随文件名。
* 拆分后是否减少了同文件无关改动冲突。
