# Go 后端 scheduler 大文件拆分

## Goal

拆分 `services/taskdaemon-go/internal/scheduler/service.go`，把 cron 校验、任务生命周期、运行生命周期、gocron 注册和 runner JSON 编解码分离到职责清晰的同 package 文件中，降低核心调度文件的阅读和维护成本。

## Requirements

* 保持 `scheduler` package 不变，不创建子 package。
* 只做纯移动和必要 import 整理，不修改调度、runner、执行历史或 overlap 行为。
* 拆分目标：
  * `service.go`：`Service`、`Options`、构造函数和核心公开入口。
  * `cron_validation.go`：cron 字段、timezone、秒级/高频 warning。
  * `task_lifecycle.go`：创建、更新、启停、删除任务。
  * `run_lifecycle.go`：trigger、cancel、begin/finish/finalize run。
  * `registration.go`：gocron scheduler 初始化、注册、移除、reconcile。
  * `runner_config_codec.go`：runner config 与 Ent JSON 字段互转。
* 复杂业务注释保留中文；函数参数和返回值说明不丢失。

## Acceptance Criteria

* [x] `internal/scheduler/service.go` 明显收敛为入口和主结构。
* [x] 新文件按职责命名，未引入循环依赖。
* [x] `go test ./internal/scheduler` 通过。
* [x] `pnpm test:go` 通过。
* [x] 提交说明标明这是纯拆分。

## Out of Scope

* 不改变 gocron 版本或调度策略。
* 不改变 runner JSON 字段、run status 或 API DTO。
* 不重写 scheduler 测试结构，除非 import/package 编译需要。

## Technical Notes

* 来源规范：`.trellis/spec/backend/file-organization.md`。
* 相关规范：`.trellis/spec/backend/scheduler-runner-guidelines.md`、`.trellis/spec/backend/quality-guidelines.md`。
* 实际拆分保持 `scheduler` package 不变，未创建子 package。
* 已额外验证：`pnpm vet:go`、`git diff --check`。
