# Go JSON 库选型调研

## 背景

taskdaemon 当前 Go 基线是 `go 1.22.6`，目标是跨平台、低占用、单二进制发布。后端使用 Gin、Ent、Wails、Koanf，并计划通过 HTTP API 服务 Web/Desktop/CLI。当前业务代码没有项目级 JSON 选型约定；测试里直接使用 `encoding/json`，Ent 生成代码也使用标准库 JSON 解码 JSON 字段。`go.mod` 中的 `sonic`、`goccy/go-json`、`json-iterator/go` 都是间接依赖，主要来自 Gin 等依赖链。

## 候选

### 标准库 `encoding/json`

* 来源：Go 官方标准库。
* 优点：零新增依赖、兼容性最高、行为稳定、跨平台风险最低，适合配置、API DTO、测试和 Ent 生成代码。
* 缺点：性能不是最快；复杂 JSON 查询或极端吞吐 API 不如专门库。

### `goccy/go-json`

* 来源：社区高性能 JSON 实现，目标是与 `encoding/json` API 兼容。
* 优点：通常可作为标准库 API 的较低侵入替换，性能较好。
* 缺点：不是标准库；需要验证 Go 版本、反射/unsafe 行为、Ent/Gin/Wails 发布模式兼容性。当前项目没有性能瓶颈证明需要它。

### `json-iterator/go`

* 来源：社区高性能 JSON 实现，宣称可作为 `encoding/json` 替换。
* 优点：生态中使用较久，API 替换成本低。
* 缺点：额外依赖和行为差异风险；维护活跃度与新 Go 版本兼容需要持续确认。

### `bytedance/sonic`

* 来源：字节跳动高性能 JSON 库，强调 SIMD/JIT/高吞吐。
* 优点：性能很强，Gin 间接依赖链已经带入。
* 缺点：更依赖平台能力和 runtime 优化，单二进制跨平台发布需要额外谨慎；对 taskdaemon 这类低吞吐管理 API，性能收益目前不明显。

## 项目约束映射

* 第一版 API 流量低，JSON 不是已知瓶颈。
* 项目更重视跨平台、可维护、Go 版本基线稳定、依赖少。
* Gin 内部可能已经按 build tag/实现选择 JSON engine，但项目业务代码不需要主动绑定到 Gin 的内部 JSON 实现。
* Ent JSON 字段和生成代码继续依赖标准库更安全。
* 前端 API client 未来更依赖稳定 DTO/OpenAPI 合约，而不是 Go JSON 库性能。

## 可行方案

### 方案 A：标准库优先（推荐）

业务代码、测试、配置、Ent 边界统一使用 `encoding/json`。不新增直接 JSON 依赖。只有在 profiling 证明 JSON 成为瓶颈时，再为 HTTP hot path 引入局部替换。

### 方案 B：HTTP 层使用 Gin 默认/高性能 JSON，业务层标准库

不在业务代码直接依赖第三方 JSON；允许 Gin 按其内部实现处理 request/response。若未来 API 压测显示需要优化，只在 `internal/httpapi` 增加可替换 JSON adapter。

### 方案 C：直接选一个高性能库作为项目标准

例如 `goccy/go-json` 或 `sonic`。短期获得性能余量，但会把项目级兼容性和维护责任提前引入；当前缺少必要性证据。

## 初步建议

选择方案 A：标准库优先。它最符合 taskdaemon 的低占用、低复杂度、跨平台单二进制目标；同时不阻碍未来在 HTTP 层按 profiling 做局部优化。

## 参考

* Go 官方 `encoding/json` package docs。
* `goccy/go-json` README。
* `json-iterator/go` README。
* `bytedance/sonic` README。
* 本仓库 `services/taskdaemon-go/go.mod`、`internal/httpapi`、`internal/data/ent`。
