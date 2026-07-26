# HANDOFF · T0 外部音频播放接口收口

> 任务：`05-28-hermes-agent-audio-api`（路线图 T0）  
> 日期：2026-07-27  
> 状态：**协调者收口** — 主体已在 `dev`；设置页**写配置**移交 T1a/T1b

## 已交付（代码在 dev）

| 能力 | 路径 |
|---|---|
| 入站 URL / upload | `internal/httpapi/audio_routes.go` · `/api/inbound/audio-play-requests` |
| 播放队列 / Oto | `internal/audio/**` |
| 历史与重放 | history API + `apps/web` 最近记录 |
| 配置只读 | `GET /api/config/audio` · `AudioSettingsPage` |
| Token 哈希 | 不明文落盘/回显 |

## 残留（明确移交，不阻塞 T0 archive）

| 残留 | 承接任务 |
|---|---|
| 配置写入 API（热生效/需重启/分级） | **T1a · C-1** |
| 设置页可写：常用配置、Token 生成一次显示、重启提示、诊断按钮完善 | **T1b（W1）** 消费 C-1 |
| 正式发行 FFmpeg 全平台内置打包矩阵 | 后续发行任务（非阻塞 API 契约） |

## 验收（协调者 2026-07-27）

- `pnpm typecheck` 绿（合入波次基线）
- `go test ./internal/audio/...` 与 httpapi 相关路径以仓库现态为准
- PRD 未勾 AC 中「设置页写配置 / Token 面板生成」归 T1a+T1b，不在本任务重复实现

## 下游解锁

- **T1a** 可开工：以音频 section 为 C-1 首个真实用例
- 路线图 §7.2 T0 → **done（主体）**；写配置闭环在 T1a/T1b
