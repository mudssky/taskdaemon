# HANDOFF · T1a 配置写入 API（C-1）

> 任务：`07-27-t1a-config-write-api`  
> 分支：`mudssky/t1a-config-write-api`  
> 日期：2026-07-27  
> 状态：**C-1 已冻结** — 待协调者验收后 merge + archive（本 worker **不 merge**）

## 结论

- **C-1 配置写入 API 形状已冻结**并实现后端。
- 首个真实用例：**`PUT /api/config/audio`**（T0 音频设置写配置）。
- W1 可仅凭 spec 用 mock 并行开发。

## 交付物

### 代码

| 路径 | 说明 |
|---|---|
| `internal/config/classification.go` | 全量字段三级分类表（单一事实源） |
| `internal/config/write.go` + `write_errors.go` + tests | 部分合并、校验、原子落盘、写锁、`ResolveWritePath` |
| `internal/httpapi/config_routes.go` | `PUT /api/config/:section` + `CONFIG_*` 映射 |
| `internal/httpapi/config_dto.go` | 写请求/响应 DTO；GET 增加 `fileOnly` |
| `internal/httpapi/runtime_config.go` | 既有热更新（未改语义） |
| `internal/app/config_write.go` | 校验→落盘→Load→Apply→失败回滚 |
| `internal/app/serve.go` / `httpapi/router.go` | append-only 接线 `WriteConfig` |

### 契约文档（C-1）

- `.trellis/spec/backend/configuration-runtime-guidelines.md` — **C-1 节**
- `.trellis/spec/backend/api-contracts.md` — 配置写入 API 节
- `.trellis/spec/backend/error-handling.md` — `CONFIG_*` 表
- `.trellis/spec/backend/index.md` — C-1 已冻结
- `docs/roadmap-post-mvp-and-agent.md` — §7.2 T1a done、§9 C-1 产物路径

### 设计/实现文档

- `.trellis/tasks/07-27-t1a-config-write-api/design.md`
- `.trellis/tasks/07-27-t1a-config-write-api/implement.md`
- PRD AC 已全部勾选

## API 速查（给 W1）

```http
PUT /api/config/audio
Cookie: taskdaemon_session=...
Content-Type: application/json

{"autoplay":{"enabled":true},"inbound":{"token":"<plain-once>"}}
```

成功 `data`：

- `config`：与 GET 同形安全 DTO（`tokenConfigured` / `pathConfigured`，无明文）
- `applied` / `restartRequired`
- `reload.subsystems[]`：`{ name, status, error? }`

错误：`CONFIG_FIELD_*` / `CONFIG_SECTION_UNKNOWN` / … + `details.fields[{path,reason,code}]` + `traceId`

## 分级策略（摘要）

| 级别 | audio 示例 |
|---|---|
| hot | autoplay / playback / inbound（含 token 明文入口）/ history |
| restart | `ffmpeg.transcodeTimeoutSeconds` |
| file_only | `ffmpeg.path` / `probePath` / 直写 `tokenHash` |

完整表：`classification.go`。其它 section 已登记分类，PUT 暂 `404 CONFIG_SECTION_UNKNOWN`。

## 文档化限制

- YAML **注释与格式不保留**（原子重写目标文件）。
- 写路径：显式 `--config` > local 配置 > base `ConfigPath`。
- env 仍覆盖文件；响应是 Load 后有效配置。

## 验证

```bash
# 本任务相关（全绿）
cd services/taskdaemon-go && go test ./internal/config/ ./internal/httpapi/ ./internal/app/ -count=1
pnpm vet:go

# 全量
pnpm typecheck   # 绿
pnpm test:go     # 绿
pnpm vet:go      # 绿
# pnpm lint：agent-web 既有 biome warning（非本任务引入）
```

## 残留 / 下游

| 项 | 承接 |
|---|---|
| 设置页 UI | **W1 / T1b** 消费 C-1 |
| 其它 section 写入实现 | 后续任务按分类表放开 |
| merge + archive | **协调者**验收后执行 |

## 给协调者

- 不要 merge 由本 worker 执行；验收后合入 `dev`。
- PRD AC 已覆盖；C-1 产物路径已回写 §9。
- 建议：合入后 W1 即可开工 mock。
