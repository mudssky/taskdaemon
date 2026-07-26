# 依赖全量升级 — 设计

## 1. 决策

| # | 决策 | 选择 |
|---|---|---|
| D1 | 升级目标 | 直接依赖 latest；间接随 lock 解析 |
| D2 | Breaking major | 先尝试；半日修不好则 pin last-compatible 并记例外 |
| D3 | lockfile 权威 | 本分支；冲突时重生成不手工 merge |
| D4 | Wails | 跟最新 v3 alpha 标签，不做大 API 重写 |
| D5 | 与 T2a | 若 T2a 未合且正在改 Ent：Go 侧先升非 ent/atlas，或等协调者放行后再动生成链 |

## 2. 文件所有权（独占）

- `package.json`（根）
- `apps/**/package.json`
- `services/**/package.json`
- `pnpm-lock.yaml` / `pnpm-workspace.yaml`（仅当必须为升级调整）
- `services/taskdaemon-go/go.mod`
- `services/taskdaemon-go/go.sum`
- 因 API 变更被迫的**最小**源码适配（记录在 HANDOFF）
- Ent 生成物：仅通过 `pnpm generate:go`

禁止：大范围业务重构、无关格式化扫街。

## 3. 升级流水线

```text
1. 基线：记录 pnpm outdated -r 与 go list -m -u（直接依赖）
2. JS：pnpm up -r --latest（或等价）；必要时按包拆开
3. 安装与 typecheck/lint/web test
4. Go：对 go.mod require 直接依赖 go get pkg@latest；go mod tidy
5. 若 ent/atlas 变：pnpm generate:go
6. go test / vet
7. 破口最小修复
8. HANDOFF 对照表
```

## 4. 风险

| 风险 | 缓解 |
|---|---|
| Biome major 规则变严 | 先升再 `biome check --write` 仅本任务触及文件；规则过激则 pin |
| React Router / Query 小版本行为 | 跑 web 单测；关键路径 smoke |
| Wails alpha 破坏 | 编译 `internal/desktop`；失败则 pin 并记 |
| Ent 生成物膨胀冲突 | 单独提交 generate；与 T2a 错开 |
| 全量 go test 预存失败 | 对照 dev 基线；不归罪本任务除非新引入 |

## 5. 验收命令

与路线图 §11 一致 + web test。
