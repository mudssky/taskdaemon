# Directory Structure

> How backend code is organized in this project.

---

## Overview

taskdaemon 采用根 Go module + 轻量前端 workspace 的结构。项目主轴是一个 Go 单二进制，负责 daemon、HTTP API、CLI 和 Wails Desktop 模式分发；前端是该核心产品的管理界面，放在 `web/app`，后续需要共享前端包时再扩展 `web/packages/*`。

---

## Directory Layout

```text
.
├── cmd/
│   └── taskdaemon/          # 单二进制入口：serve / desktop / CLI 子命令分发
├── internal/
│   ├── app/                 # 应用装配、依赖注入、生命周期
│   ├── config/              # 配置加载、默认路径、flag/env 合并
│   ├── httpapi/             # Gin router、middleware、handler
│   ├── scheduler/           # cron 调度核心与 trigger 边界
│   ├── runner/              # shell/bash/pwsh/python/node/typescript runner
│   ├── data/                # Ent client、repository、migration 边界
│   ├── auth/                # 单管理员、session、CSRF/Host 安全边界
│   └── desktop/             # Wails 绑定、桌面生命周期与原生能力
├── web/
│   ├── app/                 # Vite + React 管理台，Web/Desktop 共用
│   └── embedded/            # Go embed 包或构建产物挂载点，可按实现调整
├── docs/                    # 开发、构建、release 体积记录
├── scripts/                 # 本地开发与构建脚本
├── go.mod
├── wails.json
├── package.json
├── pnpm-workspace.yaml
└── README.md
```

---

## Module Organization

* `cmd/taskdaemon` 只负责入口分发、命令行绑定和调用应用装配，不承载业务逻辑。
* `internal/app` 负责组合配置、数据层、调度器、runner、HTTP API 和 Desktop 生命周期。
* `internal/httpapi` 放 HTTP router、middleware、request/response DTO 和 handler；handler 调用 service 边界，不直接散落数据库方言。
* `internal/scheduler` 放 cron 调度、任务注册、手动触发和 overlap 处理边界。
* `internal/runner` 放内置结构化 runner、进程执行、timeout/cancel、输出截断。
* `internal/data` 放 Ent client、repository、migration 和数据库方言隔离。
* `internal/auth` 放单管理员账号、登录态、CSRF/Host 校验等认证安全能力。
* `internal/desktop` 只放 Wails 绑定与桌面专属能力，不能复制 HTTP/API/调度业务逻辑。

---

## Naming Conventions

* Go package 目录使用小写单词，必要时使用短横线以外的自然组合；优先选择清晰的单词名，例如 `httpapi`、`scheduler`。
* 对外发布入口统一为 `cmd/taskdaemon`；第一版不创建多个 Go command 入口。
* 前端主应用固定为 `web/app`，不使用 Wails 默认 `frontend` 目录名。
* 完整 `apps/*`、`packages/*` 重 monorepo 不是第一版默认结构；如需前端共享包，优先在 `web/packages/*` 扩展。
* spike 代码如果服务后续正式模块，优先放在目标模块附近，例如 gocron spike 放在 `internal/scheduler` 附近。

---

## Examples

* gocron 调度库 spike：`internal/scheduler` 附近的测试或实验代码，结论同步到调度核心任务。
* Web/Desktop 共用 UI：`web/app`，Wails 通过 `wails.json` 指向其构建产物。
