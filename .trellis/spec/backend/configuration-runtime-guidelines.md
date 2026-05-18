# 配置、CLI 与运行时重载规范

> 配置层负责启动期输入合并，运行时重载只应用明确支持的安全子集。

---

## 概览

`services/taskdaemon-go/internal/config` 是配置结构、默认值、文件发现、环境变量映射和覆盖顺序的唯一边界。`internal/cli` 负责配置排障命令，`internal/app` 和 `internal/httpapi` 负责 daemon 运行时重载。

配置规则必须稳定、可解释、可测试。不要让 CLI、Desktop、HTTP handler 或业务 service 各自解析配置文件和环境变量。

---

## 配置加载顺序

未显式传入 `--config` 时，加载顺序固定为：

```text
defaults < project/user file < project local file < TASKDAEMON_ env < overrides
```

显式传入 `--config <path>` 时，加载顺序固定为：

```text
defaults < explicit file < TASKDAEMON_ env < overrides
```

规则：

* `LoadOptions.ConfigPath` 非空表示显式配置，只读取指定文件，不叠加项目内配置或 local 配置。
* 未显式配置时，开发工作区可自动查找项目内配置；非工作区环境回退到用户配置目录。
* local 配置只用于本机开发覆盖，放在基础项目配置之后、环境变量之前。
* `LoadOptions.Overrides` 是最后一层，通常来自 CLI flag 或测试，不应被文件/env 覆盖。
* 新增配置来源前必须更新 `ResolvePaths` 测试和本规范。

---

## 文件位置

* 默认用户配置路径为 `os.UserConfigDir()/taskdaemon/config.yaml`。
* 配置示例归属 Go service，放在 `services/taskdaemon-go/taskdaemon.example.yaml`。
* 开发期可复制为 `services/taskdaemon-go/taskdaemon.yaml`。
* 本机私有覆盖写入 `services/taskdaemon-go/taskdaemon.local.yaml`。
* 不提交真实用户配置、local 配置、日志路径凭证或数据库密码。

项目配置文件发现顺序：

```text
taskdaemon.yaml
taskdaemon.yml
config.yaml
config.yml
```

项目 local 配置文件发现顺序：

```text
taskdaemon.local.yaml
taskdaemon.local.yml
config.local.yaml
config.local.yml
```

---

## 环境变量映射

* 环境变量统一使用 `TASKDAEMON_` 前缀。
* 配置键保持 dot path，例如 `logging.http.maxBodyBytes`。
* 环境变量使用大写 snake case，例如 `TASKDAEMON_LOGGING_HTTP_MAX_BODY_BYTES`。
* 新增环境变量映射时，同时更新默认值、env alias、配置测试和示例配置。
* 环境变量值经过标量解析后进入配置，布尔、整数、列表等类型要有测试覆盖。
* 不在日志、错误响应或 CLI `config show` 中输出敏感环境变量值。

---

## 新增配置键流程

新增配置键时按这个顺序修改：

1. 在 `internal/config` 的配置结构体中新增字段。
2. 在 `Default()` 和 `defaultMap()` 中设置安全默认值。
3. 在 `Load()` 中从 koanf 读取并填充字段。
4. 如需 env 支持，更新 `envMap()` alias 和测试。
5. 更新 `taskdaemon.example.yaml`。
6. 判断是否支持运行时 reload；不支持时加入 `RestartRequired`。
7. 更新相关 spec 和前端/CLI 展示类型。

安全默认值优先保证本地开发可用、生产不意外暴露能力。例如 Swagger 默认关闭，HTTP body 日志默认关闭。

---

## CLI 配置命令

`taskdaemon config` 子命令用于排障，不承载业务状态变更，除 `reload` 外不需要运行中 daemon。

* `config path` 输出本次会使用的基础配置和 local 覆盖路径。
* `config show` 输出合并后的安全配置，必须脱敏 DSN、token、Cookie、密码等敏感字段。
* `config validate` 只验证配置能加载，不启动 HTTP server、Desktop 或 scheduler。
* `config reload` 通过运行中 daemon HTTP API 触发重载，需要管理员 session token。
* CLI 命令失败返回非零 exit code，错误说明要可操作。

CLI task/config reload 这类会影响运行中 daemon 的动作必须通过 daemon HTTP API，不在短生命周期 CLI 进程里启动临时 scheduler 或直接改运行态。

---

## Session Token 传递

CLI 调用受保护 daemon API 时，session token 来源：

1. `--session-token <token>`
2. `TASKDAEMON_SESSION_TOKEN`

规则：

* flag 优先于环境变量。
* token 仅用于构造 `taskdaemon_session` Cookie 调用本机 daemon。
* 缺失 token 时返回可操作错误，不静默降级为未认证请求。
* token 不写入日志、错误响应、`config show` 或测试快照。

---

## 运行时重载边界

当前 daemon 运行时只热更新：

* `logging.http`
* `observability.traceId`

这些字段由 `httpapi.RuntimeConfig` 保存快照，middleware 每次请求读取当前值。

需要重启的配置包括：

* `server.*`：监听地址、端口、Swagger route 注册。
* `database.*`：连接池、driver、DSN 和 migration 边界。
* `logging.level`、`logging.output`、`logging.file.*`、`logging.console.*`：handler、文件句柄和输出目标。
* 任何会影响 app 装配、scheduler 初始化或 Desktop runtime 的字段。

新增可热更新配置时必须满足：

* 有线程安全的 runtime holder。
* middleware/service 每次读取快照，而不是启动时捕获旧值。
* `ReloadResult.Applied` 和 `RestartRequired` 清楚列出配置组。
* 有测试证明 reload 后新请求读取新配置。

---

## 执行契约速查

### 1. Scope / Trigger

触发条件：新增配置字段、修改默认值、增加 `TASKDAEMON_` 环境变量、修改 `taskdaemon config` 命令、改变运行时 reload 支持范围或 CLI daemon API 调用方式。

### 2. Signatures

* 配置入口：`Load(opts LoadOptions) (Config, error)`。
* 路径解析：`ResolvePaths(opts LoadOptions) (ResolvedPaths, error)`。
* 默认值：`Default() Config` 和 `defaultMap() map[string]any`。
* 运行时应用：`RuntimeConfig.Apply(cfg config.Config) config.ReloadResult`。
* CLI reload hook：`ConfigReload func(context.Context, config.Config) (config.ReloadResult, error)`。

### 3. Contracts

* 配置键：Go struct 字段、koanf dot path、YAML 示例和 env alias 必须一致。
* env：统一 `TASKDAEMON_` 前缀和大写 snake case。
* reload：返回 `applied` 与 `restartRequired`，字段名为 camelCase JSON / YAML tag。
* CLI：`config show` 输出必须脱敏；`config reload` 必须携带 session token。

### 4. Validation & Error Matrix

| 条件 | 行为 | 错误/输出 |
|---|---|---|
| 显式 `--config` 不存在 | fail fast | 配置读取错误 |
| 默认配置不存在且 optional | 使用 defaults/env/overrides | 不报错 |
| env 标量非法或空值 | 按解析规则回退或保留字符串 | 测试记录边界 |
| reload hook 缺失 | CLI 失败 | `config reload hook is not configured` |
| session token 缺失 | daemon API 调用失败 | 提示 `--session-token` 或 `TASKDAEMON_SESSION_TOKEN` |
| 热更新不支持的配置变化 | 不应用运行态 | 出现在 `restartRequired` |

### 5. Good / Base / Bad Cases

* Good：新增 `logging.http.xxx` 时同步 struct、defaults、defaultMap、Load、env alias、example YAML、runtime Apply 和测试。
* Base：新增只启动期生效字段时，同步配置加载和示例，并把配置组列入 `RestartRequired`。
* Bad：在 HTTP handler 里直接读取 env，或 reload 时替换 listener/database/log file handler。

### 6. Tests Required

* 配置合并顺序：defaults、file、local、env、overrides。
* env alias：布尔、整数、列表和敏感字段。
* CLI：`config path/show/validate/reload` 的输出、脱敏和 session token 传递。
* reload：`Applied` / `RestartRequired` 和 middleware 读取新配置。

### 7. Wrong vs Correct

#### Wrong

```go
port, _ := strconv.Atoi(os.Getenv("TASKDAEMON_SERVER_PORT"))
```

#### Correct

```go
cfg, err := config.Load(config.LoadOptions{Overrides: overrides})
```

---

## 端口与启动行为

* 开发默认端口固定为前端 `127.0.0.1:9245`、后端 `127.0.0.1:39245`。
* 端口冲突时 fail fast，不自动漂移。
* 并行多实例通过 `TASKDAEMON_SERVER_PORT`、配置文件 `server.port`、Wails `-port`、`TASKDAEMON_WEB_PORT` 和前端代理目标显式覆盖。
* `serve` 启动 HTTP API，不初始化 Desktop。
* `desktop` 启动 Wails runtime，并启动本机 HTTP API server。
* `db migrate` 是 CLI-only 入口，不初始化 Desktop 或 scheduler。

---

## 测试要求

* 配置加载测试覆盖默认值、显式 `--config`、项目配置、local 覆盖、env 覆盖和 overrides 优先级。
* env 映射测试覆盖布尔、整数、列表、别名和非法/空值边界。
* `config show` 测试必须证明敏感字段被脱敏。
* `config reload` 测试覆盖 session token 传递、daemon API 路径、返回的 `Applied` / `RestartRequired`。
* runtime config 测试覆盖 reload 后 middleware 读取新值。
* 配置文件本身不需要单元测试；承载行为的 Go 代码需要测试。

---

## 禁止模式

* 不在 `internal/app`、`internal/httpapi`、`internal/desktop` 或业务 service 中重新解析配置文件。
* 不让发布版自动读取当前目录的同名配置文件。
* 不提交 `taskdaemon.local.yaml` 或包含真实凭证的配置。
* 不在 reload 中替换 server listener、数据库连接或日志文件句柄。
* 不把 session token、数据库密码、Cookie 或 runner env 打印到 CLI 输出。
