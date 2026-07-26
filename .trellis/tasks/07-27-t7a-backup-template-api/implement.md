# T7a Implement · 备份任务模板后端 API

## 顺序

1. `internal/template/types.go` + `errors.go`：模型与 `TEMPLATE_*` / FieldError（C-1 同形）
2. `shellquote.go` + test：注入防护与路径检查
3. `validate.go` + test：参数类型/必填/范围/secret_ref
4. `registry.go` + test：Register/Get/List；注册期 runner 白名单
5. `render.go` + test：编排校验→Render→runner.Validate→脱敏预览
6. 三个 builtin 模板 + 单元测试（含注入载荷）
7. `httpapi/template_dto.go` + `template_routes.go`
8. `router.go` append-only 注册
9. `template_routes_test.go`：成功/失败/401/字段错误
10. 更新 `error-handling.md`、`api-contracts.md`；路线图 §7.2 T7a 状态
11. 验收：`pnpm typecheck && pnpm test:go && pnpm vet:go`
12. HANDOFF.md

## 验证

```bash
cd services/taskdaemon-go && go test ./internal/template/ ./internal/httpapi/ -count=1
pnpm typecheck && pnpm test:go && pnpm vet:go
```

## 回滚点

- 删除 `internal/template/**` 与 `httpapi/template_*`
- 回退 `router.go` 注册行

## 不做

- 向导 UI（T7b/W3）
- 任务自动创建
- pgBackRest
