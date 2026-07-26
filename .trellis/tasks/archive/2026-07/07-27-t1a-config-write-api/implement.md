# T1a Implement · C-1 配置写入 API

## 顺序

1. `internal/config/classification.go` + test：三级表与查询 API
2. `internal/config/write.go` + test：ResolveWritePath、MergeAudioSection、Validate、AtomicWrite、mutex
3. `internal/config` 错误类型：`FieldError` / `ValidationError`
4. `httpapi/config_dto.go`：write request/response；对齐 GET classification
5. `httpapi/runtime_config.go`：必要时 Snapshot（若需回滚用 Load 旧文件即可）
6. `app/config_write.go`：`WriteConfigSection` 编排 + 回滚
7. `httpapi/config_routes.go`：`PUT /:section`；Options 注入 writer
8. `router.go` / `serve.go`：append 接线
9. route 测试覆盖 AC
10. 更新 spec + roadmap §9/§7.2
11. 验收命令 + HANDOFF

## 验证

```bash
pnpm typecheck && pnpm lint
cd services/taskdaemon-go && go test ./internal/httpapi/ ./internal/config/ ./internal/app/
pnpm test:go && pnpm vet:go
```

## 回滚点

- 仅新增文件可删；router Options 字段回退
- 不改 Ent schema

## 不做

- 设置页 UI
- 全 section 写入实现（仅 audio + 完整分类表）
- 注释保留 YAML 引擎
