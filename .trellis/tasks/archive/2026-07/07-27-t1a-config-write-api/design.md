# T1a Design · C-1 配置写入 API

## 1. 边界

| 层 | 职责 |
|---|---|
| `internal/config` | 字段三级分类表、section 部分合并、校验、原子落盘、写锁 |
| `internal/httpapi` | `PUT /api/config/:section`、DTO、错误码映射、session 鉴权 |
| `internal/app` | 编排：校验→备份→落盘→Load→Apply→失败回滚；注入 LoadOptions |

独占改动：`config_routes.go` / `config_dto.go` / `runtime_config.go` 及测试。  
共享 append-only：`config` 新文件 + types 注释、`router.go` Options 字段、`app` 写配置入口。

## 2. 三级分类（契约）

| 级别 | API | 语义 |
|---|---|---|
| `hot` | 可写 | 落盘后立即 `RuntimeConfig.Apply` + 相关 subsystem Update |
| `restart` | 可写 | 落盘；响应 `restartRequired` 列出字段；运行态不变直到进程重启 |
| `file_only` | **拒绝** | 安全/路径/部署类；提交 → `CONFIG_FIELD_NOT_WRITABLE` |

缺省级别：**不是** hot。新增字段必须在 `classification.go` 显式登记。

### 2.1 全量字段表（首版）

见实现 `internal/config/classification.go` 与 spec 增补节；摘要：

- **hot**：`logging.http.*`、`observability.traceId.*`、`audio.autoplay|playback|inbound|history.*`（token 明文入口映射为 hash）、`notify.store|webhook|email|desktop.*`（不含 secret 明文回显）、`agentBridge.allowedTaskIds|maxLoopDepth`、`desktop.minimizeToTray`
- **restart**：`logging.level|output|console.*|file.*`（path 除外）、`audio.ffmpeg.transcodeTimeoutSeconds`、`notify.bufferSize`、`runlog.*`、`agentBridge.enabled|gateway*|requestTimeoutSeconds`、`desktop.tray|singleInstance|windowState|autostart*`
- **file_only**：`server.*`、`database.*`、`logging.file.path`、`audio.ffmpeg.path|probePath`、`agentBridge.inboundTokenHash`（API 只接受 `inboundToken` 明文并哈希）、路径类与 DSN

首版 **实现写入** 的 section：`audio`（T0 首个真实用例）。其他 section 分类表完整，PUT 返回 `CONFIG_SECTION_NOT_IMPLEMENTED` 或仅文档开放、实现 stub 拒绝——为避免假完成，**仅 audio 全通**；其余 section 返回明确 `CONFIG_SECTION_UNKNOWN` / 未开放，分类表仍冻结供 W1 mock。

**决策**：未实现写入的 section → `404 CONFIG_SECTION_UNKNOWN`（与未知 section 相同），分类表文档供下游 mock；后续任务按表放开。

## 3. API 形状

### 3.1 `PUT /api/config/:section`

- 认证：`requireSession`（管理员 Cookie）。机器 Bearer **不能**调用。
- 语义：部分更新；JSON 中**出现的字段**才改；省略字段保持原值。
- 不接受 file_only 字段（含 `tokenHash` 直接写哈希——应走 `token` 明文一次）。

请求（audio 示例）：

```json
{
  "autoplay": { "enabled": true, "target": "backend" },
  "playback": { "queueLimit": 10 },
  "inbound": {
    "token": "plaintext-once",
    "maxBytes": 1048576,
    "url": {
      "allowedSchemes": ["https"],
      "allowPrivateNetworks": false,
      "allowedHosts": [],
      "downloadTimeoutSeconds": 60,
      "maxRedirects": 3
    }
  },
  "history": { "limit": 50 },
  "ffmpeg": { "transcodeTimeoutSeconds": 180 }
}
```

响应 `data`：

```json
{
  "config": { /* 与 GET 同形的安全 DTO */ },
  "applied": ["audio.autoplay", "audio.playback"],
  "restartRequired": ["audio.ffmpeg.transcodeTimeoutSeconds"],
  "reload": {
    "applied": ["audio.autoplay", "audio.playback", "audio.inbound", "audio.history"],
    "restartRequired": ["audio.ffmpeg"],
    "subsystems": [
      { "name": "runtime", "status": "ok" },
      { "name": "audio", "status": "ok" }
    ]
  }
}
```

### 3.2 错误（`CONFIG_*` + traceId）

| code | HTTP | 场景 | details |
|---|---:|---|---|
| `CONFIG_SECTION_UNKNOWN` | 404 | 未知或未开放 section | `{ "section" }` |
| `CONFIG_INVALID_JSON` | 400 | body 非法 | — |
| `CONFIG_FIELD_INVALID` | 400 | 类型/格式非法 | `{ "fields": [{ "path", "reason", "code" }] }` |
| `CONFIG_FIELD_OUT_OF_RANGE` | 400 | 越界 | 同上 |
| `CONFIG_FIELD_CONFLICT` | 400 | 字段冲突 | 同上 |
| `CONFIG_FIELD_NOT_WRITABLE` | 400 | file_only 或未知字段 | 同上 |
| `CONFIG_VALIDATION_FAILED` | 400 | 多类错误聚合（可选顶码） | fields 数组 |
| `CONFIG_PATH_UNAVAILABLE` | 503 | 无法解析可写路径 | — |
| `CONFIG_WRITE_FAILED` | 500 | 落盘失败 | — |
| `CONFIG_RELOAD_FAILED` | 500 | Apply 失败且已回滚文件 | `{ "subsystems" }` |

多字段校验失败：HTTP 400，优先用最具体单一码；若混合类型用 `CONFIG_VALIDATION_FAILED`，`details.fields[]` 每项带 `code`。

既有 `config_reload_failed` / `config_reload_unavailable` 保留兼容；新写入路径统一 `CONFIG_*`。

## 4. 落盘与并发

1. `ResolveWritePath(LoadOptions)`：显式 `--config` → 该文件；否则若存在 local 配置 → **local**（最高优先级文件源）；否则 base `ConfigPath`（不存在则创建）。
2. 读现有 YAML → `map[string]any` 深度合并 section → `yaml.Marshal` 写临时文件 → `Rename` 原子替换。
3. **注释/格式不保留**（文档化限制）：写入重写目标文件；用户注释会丢失。提示：重要注释放在仓库内 example，或手工维护 local 前备份。
4. 进程内 `sync.Mutex` 串行化写入。
5. env / overrides 仍覆盖文件；API 写入不清除 env。响应返回 **Load 后有效配置**（含 env）。

## 5. 热生效与回滚

```text
lock → validate(no write) → read file backup → atomic write
  → Load(opts) → Apply runtime + subsystem Update
  → on Load/Apply fail: restore backup file, re-Apply previous snapshot → error
unlock
```

- 沿用 `RuntimeConfig.Apply` + `audioService/audioQueue.UpdateConfig`（与 `ReloadRuntimeConfig` 同路径）。
- Apply 当前不返回 error；subsystem 包装仍上报 ok/failed。
- 校验失败：**零写入**。

## 6. 安全

- 响应永不回显 token/hash/password/path 明文（沿用 GET audio DTO：`tokenConfigured`、`pathConfigured`）。
- 请求 `inbound.token` 明文 → `audio.HashToken` → 落盘 `tokenHash`；日志只记 section + 字段路径，不记 token。
- 仅 session；不注册入站 Bearer。

## 7. 兼容

- `GET /api/config/audio`、`POST /api/config/reload` 行为不变。
- GET 的 `configuration.runtimeEditable/restartRequired` 与分类表对齐（ffmpeg path 从 restart 改为 file_only 展示；transcodeTimeout 仍 restart）。

## 8. 测试矩阵

| 场景 | 期望 |
|---|---|
| 部分更新 autoplay | 其他字段不变 |
| 提交 `ffmpeg.path` | 400 NOT_WRITABLE |
| queueLimit -1 | 400 OUT_OF_RANGE |
| 非法 target | 400 INVALID |
| 无 session | 401 |
| Bearer only | 401 |
| token 写入 | 响应 tokenConfigured=true，body 无明文 |
| 热字段 | RuntimeConfig 立即变化 |
| restart 字段 | 文件有值，runtime ffmpeg 可不立即换 path（path 不可写） |
| 并发写 | 串行无损坏 |
| 原子写 | 中断不留半文件（rename 语义） |
