# 增加项目 gitignore

## Goal

为 taskdaemon 仓库增加根 `.gitignore`，避免 Python 缓存、Go 构建产物、Node/Vite/Wails 产物、本地环境文件和编辑器临时文件进入 git 状态。

## Requirements

* 新增仓库根 `.gitignore`。
* 忽略当前已出现的 `.pyc` 和 `__pycache__`。
* 覆盖 Go 常见构建/测试产物。
* 覆盖 Node/pnpm/Vite 前端依赖与构建缓存。
* 覆盖 Wails 构建目录。
* 覆盖本地环境变量、日志、临时文件和编辑器目录。
* 不忽略需要提交的 `go.mod`、`go.sum`、`pnpm-lock.yaml`、Trellis 任务文档或源码。

## Acceptance Criteria

* [x] 根目录存在 `.gitignore`。
* [x] `git status --porcelain` 不再显示当前 `.pyc` 缓存文件。
* [x] `git check-ignore` 能命中 `__pycache__`、Go test binary、`node_modules` 和 Wails build 产物示例。
* [x] 不影响已经需要跟踪的源码、规范和任务文档。

## Definition of Done

* `.gitignore` 已提交。
* 使用 git 命令验证忽略规则生效。
* 不删除用户未明确要求删除的现有未跟踪文件。

## Out of Scope

* 不清理或删除当前未跟踪缓存文件。
* 不调整项目目录结构。
* 不增加 lint/test 配置。

## Technical Notes

* 当前仓库已有两个未跟踪 Python 缓存文件：`.codex/skills/ui-ux-pro-max/scripts/__pycache__/*.pyc`。
* 项目已确认采用 Go 根 module + `web/app` 前端 workspace + Wails 的结构。
