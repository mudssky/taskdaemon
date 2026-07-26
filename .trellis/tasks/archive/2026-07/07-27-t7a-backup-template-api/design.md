# T7a Design · 备份任务模板后端 API

## 1. 边界

| 层 | 职责 |
|---|---|
| `internal/template` | 模板模型、注册表、参数校验、安全渲染、命令预览脱敏 |
| `internal/httpapi` | `GET/POST /api/templates*`、DTO、错误码映射、session 鉴权 |
| `internal/runner` | 只读消费白名单（`Validate` / 类型常量）；不改执行语义 |

独占改动：`internal/template/**`（新建）、`internal/httpapi/template_*.go`（新建）及测试。  
共享 append-only：`httpapi/router.go`（注册一行 + Options 可选）、`error-handling.md` / `api-contracts.md` 增补 `TEMPLATE_*`。

**不落库、不创建任务**：渲染只返回与 `createTaskRequest` 兼容的草稿 JSON。

## 2. 模板模型

### 2.1 参数类型（有限枚举）

| Type | JSON | 语义 |
|---|---|---|
| `string` | string | 普通文本 |
| `number` | number | 整数/浮点（模板自定范围） |
| `boolean` | bool | 开关 |
| `enum` | string ∈ options | 枚举 |
| `path` | string | 本地路径；禁止空、NUL、相对逃逸到控制字符 |
| `secret_ref` | string | **凭据引用名**（环境变量名），不是密钥明文 |

`secret_ref` 与 C-1 对齐的约定：

- 用户**不**提交密码明文进入任务定义。
- 引用形态：合法 env 名（`[A-Za-z_][A-Za-z0-9_]*`），如 `PGPASSWORD`。
- 渲染结果：**不**把引用解析为明文，**不**写入 `runner.inline` / `runner.env` 的值字段。
- 命令依赖 libpq / 工具对宿主环境变量的既有读取（如 `PGPASSWORD`）；帮助文本说明须在运行环境提供该变量。
- 命令预览与日志对任何 `Sensitive` 参数值做掩码（`***`）。

### 2.2 定义结构

```text
Definition {
  ID, Name, Description, Scenario
  RunnerType          // 必须 ∈ runner 白名单
  Params []ParamDef   // Name, Type, Required, Default, EnumOptions, Help, Min/Max, Sensitive
  Render(ctx, values) → TaskDraft
}
```

- 注册期：`RunnerType` 不在白名单 → **拒绝注册**（测试用恶意定义证明）。
- 渲染框架只做：查表 → 校验参数 → 调 `Render` → 再校验草稿 runner → 脱敏预览。
- 新增模板 = 新增定义文件 + `init`/`Register`，**不改** registry/render 核心。

### 2.3 首批模板

| ID | Runner | 要点 |
|---|---|---|
| `postgres-pg-dump` | `shell` | host/port/user/database、`passwordEnv`(secret_ref)、outputPath、compress、cron/timezone/timeout |
| `sqlite-file-backup` | `shell` | sourcePath、destPath、verifyIntegrity、cron/timezone/timeout |
| `generic-script` | `shell` | command（用户命令骨架）、workDir、cron/timezone/timeout |

## 3. 任务草稿契约

与 `createTaskRequest` / `createRunnerRequest` **字段同形**，便于前端直接 `POST /api/tasks`：

```json
{
  "name": "pg-backup-app",
  "description": "Rendered from template postgres-pg-dump",
  "enabled": false,
  "cronExpression": "30 2 * * *",
  "timezone": "Local",
  "confirmCronWarnings": false,
  "runner": {
    "type": "shell",
    "inline": "pg_dump -h '...' ...",
    "scriptPath": "",
    "args": [],
    "workDir": "",
    "env": {},
    "timeoutSeconds": 3600,
    "outputLimitBytes": 0
  },
  "commandPreview": "pg_dump -h 'db' ...  # password via env PGPASSWORD",
  "templateId": "postgres-pg-dump"
}
```

- `enabled` 默认 `false`（草稿安全默认）。
- 渲染成功后再跑 `runner.Validate`；失败 → `TEMPLATE_RUNNER_UNSUPPORTED` 或校验错误。

## 4. API

全部 `requireSession`（管理员 Cookie）。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/templates` | 列表（含完整参数定义）`{ "templates": [...] }` |
| GET | `/api/templates/:id` | 详情 |
| POST | `/api/templates/:id/render` | body: `{ "params": { ... } }` → 草稿 |

### 4.1 错误码（`TEMPLATE_*`）

| code | HTTP | 场景 | details |
|---|---:|---|---|
| `TEMPLATE_NOT_FOUND` | 404 | 未知模板 id | `{ "id" }` |
| `TEMPLATE_INVALID_JSON` | 400 | body 非法 | — |
| `TEMPLATE_FIELD_INVALID` | 400 | 类型/格式/路径/注入字符 | `{ "fields": [{path,reason,code}] }` |
| `TEMPLATE_FIELD_REQUIRED` | 400 | 缺必填 | fields |
| `TEMPLATE_FIELD_OUT_OF_RANGE` | 400 | 数值越界 | fields |
| `TEMPLATE_VALIDATION_FAILED` | 400 | 多类字段错误聚合 | fields |
| `TEMPLATE_RUNNER_UNSUPPORTED` | 400 | 草稿 runner 越界（或注册期） | `{ "runnerType" }` |
| `TEMPLATE_RENDER_FAILED` | 500 | 未预期渲染失败 | — |
| `TEMPLATE_UNAVAILABLE` | 503 | registry 未注入 | — |

字段级结构与 C-1 **同形**：`error.details.fields[]` 每项 `{ "path", "reason", "code" }`。  
`path` 使用 `params.<name>` 点分路径。

## 5. 命令注入防护（头号风险）

原则：**用户输入只进入被 shell 单引号包裹的字面量位置，永不拼进未引用的命令语法。**

1. `shellQuote(s)`：POSIX 单引号转义（`'` → `'\''`）；拒绝含 `NUL`（`\x00`）的值。
2. 路径参数额外拒绝 ASCII 控制字符；允许绝对/相对路径字面量，但一律 quote。
3. `pg_dump` / `sqlite`：整条 `inline` 由固定 argv 骨架 + quote 后的参数拼接；压缩等开关只映射到固定 flag，不接用户自由 flag 串。
4. `generic-script`：`command` 作为用户明确声明的整段 shell（高级模板）；仍拒绝 `NUL`；**不**对 command 做二次宏展开。注入 AC 的主力断言在结构化模板（pg/sqlite）上覆盖 `;` `` ` `` `$()` 换行等载荷。
5. 渲染后 `runner.Validate` 二次兜底类型白名单。

测试载荷至少：

- `'; dropdb evil`
- `` `id` ``
- `$(reboot)`
- 含换行的 path
- `passwordEnv` 非法名 / 尝试塞 `a;b`

期望：载荷原样出现在 **引号内** 字面量中，或字段级拒绝；`runner.inline` 不可被解析为额外命令（通过 quote 后的结构断言）。

## 6. 脱敏

- `ParamDef.Sensitive == true` 或类型 `secret_ref`：预览中值显示 `***`；日志只记 templateId + param keys，不记值。
- 草稿 JSON 中不出现 secret 明文（secret_ref 只保留 env **名** 于 description/help 旁注，不进 env map 值）。

## 7. 包布局

```text
internal/template/
  types.go          // ParamType, ParamDef, Definition, TaskDraft, FieldError...
  errors.go         // TEMPLATE_* 常量, ValidationError
  registry.go       // Registry, Register, Get, List；注册期 runner 检查
  validate.go       // 参数校验 → []FieldError
  shellquote.go     // shellQuote + 安全检查
  render.go         // Render(id, params)
  builtin.go        // 注册三个内置模板
  builtin_pgdump.go
  builtin_sqlite.go
  builtin_script.go
  *_test.go

internal/httpapi/
  template_routes.go
  template_dto.go
  template_routes_test.go
  router.go         // append registerTemplateRoutes
```

`Registry` 默认 `DefaultRegistry()` 含内置三模板；测试可 `NewRegistry()` 隔离。

## 8. 兼容与不做

- 不改任务 CRUD、不改 Ent。
- 不实现向导 UI、restore、对象存储。
- pgBackRest 等不进首批。

## 9. 测试矩阵

| 场景 | 期望 |
|---|---|
| List 三模板 | 含 params 定义 |
| Get 未知 id | 404 NOT_FOUND |
| Render pg_dump 合法 | 草稿可被 taskInput 形状消费；enabled=false |
| Render 缺必填 | 400 + fields path |
| 类型错误 / 越界 | FIELD_INVALID / OUT_OF_RANGE |
| 注入载荷 | quote 或拒绝；无额外命令结构 |
| 非法 runner 注册 | Register 失败 |
| 渲染后伪造 type | RUNNER_UNSUPPORTED（内测 Render 钩子或校验） |
| secret_ref 不进 inline 明文 | 断言 |
| 预览掩码 | `***` |
| 测试专用第四模板 Register | 无需改框架即可 List 到 |
| 无 session | 401 |
