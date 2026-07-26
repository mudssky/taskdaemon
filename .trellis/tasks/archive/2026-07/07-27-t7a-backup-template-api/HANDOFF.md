# HANDOFF · T7a 备份任务模板后端 API

> 任务：`07-27-t7a-backup-template-api`  
> 分支：`mudssky/t7a-backup-template-api`  
> 日期：2026-07-27  
> 状态：**后端已交付** — 待协调者验收后 merge + archive（本 worker **不 merge**）

## 结论

- 备份模板后端 API 已实现：列表 / 详情 / 渲染草稿（不落库）。
- 首批三模板：`postgres-pg-dump`、`sqlite-file-backup`、`generic-script`。
- 字段级错误与 C-1 同形（`details.fields[{path,reason,code}]`）。
- 命令注入：用户参数经 POSIX 单引号字面量进入 shell；注册/渲染二次校验 runner 白名单。
- 敏感参数：`secret_ref` 只接受 env 名（如 `PGPASSWORD`），不明文进入草稿 `inline`/`env`。

## 交付物

### 代码

| 路径 | 说明 |
|---|---|
| `internal/template/**` | 模型、注册表、校验、shellQuote、渲染、三内置模板 + 测试 |
| `internal/httpapi/template_routes.go` | `GET/POST /api/templates*` |
| `internal/httpapi/template_dto.go` | 定义 DTO |
| `internal/httpapi/template_routes_test.go` | route 成功/失败/注入/401 |
| `internal/httpapi/router.go` | append-only 注册 + Options.Templates |
| `internal/app/serve.go` | 装配 DefaultRegistry |

### 契约 / 文档

- `.trellis/spec/backend/api-contracts.md` — 备份模板 API 节
- `.trellis/spec/backend/error-handling.md` — `TEMPLATE_*` 表
- `docs/roadmap-post-mvp-and-agent.md` — §7.2 T7a **done（待合入）**
- `design.md` / `implement.md` / PRD AC 已勾选

## API 速查（给 W3）

```http
GET /api/templates
GET /api/templates/:id
POST /api/templates/:id/render
Cookie: taskdaemon_session=...
Content-Type: application/json

{"params":{"name":"pg-nightly","host":"127.0.0.1","user":"postgres","database":"app","outputPath":"/backups/app.dump","cronExpression":"30 2 * * *"}}
```

成功 `data`：与 `createTaskRequest` 兼容的草稿 + `commandPreview` + `templateId`；`enabled` 默认 `false`。

错误：`TEMPLATE_*` + 可选 `details.fields[]` + `traceId`。

## 安全摘要

| 项 | 行为 |
|---|---|
| 注入 | `;` `` ` `` `$()` 等进入 `-h '...'` 字面量 |
| 换行路径 | `TEMPLATE_FIELD_INVALID` |
| 密码 | 不进任务定义；宿主提供 `passwordEnv`（默认 `PGPASSWORD`） |
| runner 越界 | 注册期 / 渲染后 `TEMPLATE_RUNNER_UNSUPPORTED` |
| 落库 | **无** — 仅草稿 |

## 验证

```bash
pnpm typecheck   # 绿
pnpm test:go     # 绿（含 internal/template、httpapi template routes）
pnpm vet:go      # 绿
```

## 残留 / 下游

| 项 | 承接 |
|---|---|
| 向导 UI | **W3 / T7b** |
| merge + archive | **协调者** |
| 更多模板（pgBackRest 等） | 后续按 Register 扩展 |

## 给协调者

- 不要 merge 由本 worker 执行；验收后合入。
- W3 可凭 API 契约 mock 并行开发。
