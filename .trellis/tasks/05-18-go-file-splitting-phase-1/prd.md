# Go 后端大文件拆分第一批

## Goal

继续落实 `.trellis/tasks/archive/2026-05/05-16-code-spec-go-file-splitting/prd.md` 和 `.trellis/spec/backend/file-organization.md` 中记录的 Go 大文件拆分工作。第一批选择低行为风险、职责边界清晰的文件做纯拆分，降低后续维护和并发改动成本。

## Requirements

* 本轮优先拆分 `services/taskdaemon-go/internal/config/config.go`。
* 拆分保持 package 不变，不改公开 API，不改配置加载行为。
* 目标文件按现有规范拆为类型/defaults、路径解析、加载合并、env/scalar 解析等职责。
* 移动后运行 gofmt 和 Go 测试，确保行为等价。
* 若发现拆分引入行为变更风险，停止并改为更小范围。

## Acceptance Criteria

* [x] `internal/config/config.go` 不再承载全部配置职责。
* [x] 新文件名能表达职责，并保持同 package 内聚。
* [x] 配置相关测试通过。
* [x] `pnpm test:go` 通过。
* [x] 提交说明明确这是纯拆分。

## Definition of Done

* 只移动/整理代码，不做功能变更。
* gofmt 已运行。
* 相关测试通过。
* Conventional Commit 提交完成。

## Out of Scope

* 本轮不拆 scheduler、logging、httpapi、cli、app。
* 本轮不调整配置加载语义、默认值、env key 或 reload 行为。
* 本轮不修改前端代码。

## Technical Notes

* 来源任务：`.trellis/tasks/archive/2026-05/05-16-code-spec-go-file-splitting/prd.md`。
* 规范：`.trellis/spec/backend/file-organization.md`。
* 第一批选择 `internal/config/config.go`，因为它是纯配置层，已有 `config_test.go` 覆盖默认路径、加载顺序、local 覆盖、env 映射和显式配置路径。
* 已拆分为 `types.go`、`defaults.go`、`paths.go`、`load.go`、`env.go`。
* 已运行 `go test ./internal/config` 和 `pnpm test:go`，均通过。

## Follow-up Splitting Tasks

* `.trellis/tasks/05-18-go-file-splitting-scheduler`
* `.trellis/tasks/05-18-go-file-splitting-logging`
* `.trellis/tasks/05-18-go-file-splitting-httpapi-middleware`
* `.trellis/tasks/05-18-go-file-splitting-cli-command`
* `.trellis/tasks/05-18-go-file-splitting-app-lifecycle`
