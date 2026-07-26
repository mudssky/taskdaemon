# T2a 通知事件总线与站内 sink — 执行计划

## 前置

- [ ] 读 `.trellis/spec/backend/index.md` 开发前检查第 1、3、5、6、7、8 项
- [ ] 读 `.trellis/spec/backend/database-guidelines.md`（新增 Ent entity）
- [ ] 读 `.trellis/spec/backend/error-handling.md`（`NOTIFY_*` 错误码）
- [ ] 确认 T6 当前**没有**未合并的 Ent schema 改动（路线图 §8.3，本任务优先）
- [ ] 从 `dev` 切分支 `feat/t2a-notification-event-bus`

## 阶段 1 · 决策与骨架

> **本阶段有两个必须先定的决策，定完再写代码，避免返工。**

- [ ] **决策 A**：脱敏规则的包位置（design §2.4）—— 复用 `httpapi/redaction.go` 还是下沉共享包。写进代码注释
- [ ] **决策 B**：scheduler 是否 import `notify.Event`（design §7）—— 直接 import 还是最小结构 + app 层转换。写进代码注释
- [ ] 创建 `internal/notify/` 包
- [ ] `event.go`：`Name` 常量、`Severity` 枚举 + `Valid()`、`Event`、`Subject`
- [ ] `event.go`：事件构造函数，内含 `Detail` 白名单过滤
- [ ] 单测：非法 `Severity` 被拒绝；`Detail` 白名单过滤生效；敏感字段不进 `Detail`

**验证**：`cd services/taskdaemon-go && go test ./internal/notify/...`

## 阶段 2 · Bus

- [ ] `sink.go`：`Sink` 接口、`SinkStatus`
- [ ] `bus.go`：`Bus` 结构、`Register`、`Publish`、`SinkStatuses`、`UpdateConfig`
- [ ] 异步分发 goroutine，缓冲满时丢弃 + 计数 + 日志
- [ ] 每个 sink 调用包 `recover`
- [ ] `Start` / `Shutdown` 生命周期，关闭时排空缓冲
- [ ] 单测：
  - [ ] 注册空 sink 无需改 Bus 代码即可接入
  - [ ] 单 sink 返回 error 不影响其他 sink
  - [ ] 单 sink panic 不影响其他 sink、不崩进程
  - [ ] `Publish` 不阻塞（用慢 sink 验证）
  - [ ] 缓冲满时丢弃计数正确
  - [ ] `Enabled() == false` 的 sink 不被调用

**验证**：`go test ./internal/notify/...`

## 阶段 3 · Ent schema 与持久化

> ⚠️ 本阶段产生 Ent 生成物冲突风险，尽快完成并合并。

- [ ] 新建 `internal/data/ent/schema/notification.go`（design §4.1 字段与索引）
- [ ] `pnpm generate:go`
- [ ] `internal/notify/sink_store.go`：`StoreSink` 实现 `Sink`
- [ ] `event_id` 唯一约束保障幂等：重复投递不产生重复记录
- [ ] `prune.go`：条数 + 天数双维度淘汰，参考 `audio/service.go:408 pruneHistory`
- [ ] 淘汰在写入后异步触发，不在写入事务内
- [ ] 单测：
  - [ ] 事件落库字段正确
  - [ ] 同一 `event_id` 重复投递只落一条
  - [ ] 条数上限淘汰正确，配置 0 不限
  - [ ] 天数淘汰正确，配置 0 不限
  - [ ] 两维度同时生效

**验证**：`go test ./internal/notify/... ./internal/data/...`

## 阶段 4 · 配置

- [ ] `internal/config/types.go` **尾部追加** `NotifyConfig` / `NotifyStoreConfig`（append-only，标注 `// T2a`）
- [ ] `internal/config/defaults.go` 追加默认值：`BufferSize=256`、`MaxRecords=200`、`RetainDays=30`
- [ ] `internal/config/env.go` 追加 `TASKDAEMON_NOTIFY_*` 映射
- [ ] `Bus.UpdateConfig` 支持热更新（参考 `audio.Service.UpdateConfig`）
- [ ] 单测：默认值、env 覆盖、热更新生效

**验证**：`go test ./internal/config/... ./internal/notify/...`

## 阶段 5 · HTTP API

- [ ] `internal/httpapi/notification_dto.go`：请求/响应 DTO
- [ ] `internal/httpapi/notification_routes.go`：design §5 的 8 个端点
- [ ] `internal/httpapi/router.go` **追加**注册行（append-only，`// T2a` 分组注释）
- [ ] `NOTIFY_*` 错误码定义
- [ ] 全部端点走管理员认证
- [ ] route 测试（参考 `audio_routes_test.go` 结构）：
  - [ ] 列表分页、已读筛选、级别筛选
  - [ ] 未读计数
  - [ ] 单条 / 批量 / 全部已读
  - [ ] 删除单条、清空已读
  - [ ] sink 状态查询
  - [ ] 未认证返回 401
  - [ ] 非法分页参数、非法 severity 返回对应错误码
  - [ ] 响应含 traceId

**验证**：`go test ./internal/httpapi/...`

## 阶段 6 · 接入调度器

- [ ] 按**决策 B** 在 `internal/scheduler/service.go` 的 `Options` 追加 `Publisher` 字段，默认 noop
- [ ] `run_lifecycle.go` 三处终态发布事件（design §7）
- [ ] `internal/app/serve.go` 装配 Bus 与 StoreSink（与 audio service 装配位置一致）
- [ ] 单测：
  - [ ] 四类终态各产生正确事件
  - [ ] 无 Publisher 时调度器正常工作
  - [ ] 通知失败不影响 run 结果

**验证**：`go test ./internal/scheduler/... ./internal/app/...`

## 阶段 7 · 安全审查

- [ ] 人工核对：`Title` / `Body` / `Detail` 中无 token、密码、完整命令行、URL query
- [ ] 单测：构造一个含敏感值的 run 结果，断言产生的通知中不含该值
- [ ] 核对日志输出无敏感值

## 阶段 8 · 契约冻结（C-2）

- [ ] 事件名清单、severity 枚举、`Event` 字段表写入 `.trellis/spec/backend/` 对应规范
- [ ] `Sink` 接口、注册方式、失败语义、新增 sink 步骤写入同处
- [ ] API 形状写入 `.trellis/spec/backend/api-contracts.md`
- [ ] `NOTIFY_*` 错误码写入 `.trellis/spec/backend/error-handling.md`
- [ ] 回写路线图 §9 表格 C-2 行的产物路径
- [ ] 回写路线图 §7.2 状态为 done

> **冻结完成即通知 W2 / T4 / D2 可以开工。**

## 全量验收

```bash
pnpm typecheck
pnpm lint
pnpm test:go
pnpm vet:go
cd services/taskdaemon-go && go test ./...
```

- [ ] 上述命令全绿
- [ ] `prd.md` 全部 Acceptance Criteria 勾选

## 评审门

| 门 | 时机 | 检查 |
|---|---|---|
| G-1 | 阶段 1 结束 | 决策 A/B 已定并写进注释；事件模型是否够 T4/D2 用 |
| G-2 | 阶段 3 结束 | Ent schema 定型，**尽快合并**降低冲突窗口 |
| G-3 | 阶段 8 前 | 契约是否稳定到可以让三个下游任务依赖 |

## 回滚点

| 阶段 | 回滚方式 |
|---|---|
| 1–2 | 纯新增包，直接删除 |
| 3 | revert commit；Ent 新表留空不影响其他功能 |
| 4–5 | revert；配置与路由均为 append，删除追加部分即可 |
| 6 | Publisher 置 noop 即完全停用，无需 revert |
| 运行时 | `notify.store.enabled = false` 热关闭 |

## 与其他任务的协调

- **T6**：Ent 生成物冲突。本任务优先，阶段 3 完成后尽快合并并通知 T6
- **W2 / T4 / D2**：阶段 8 完成后通知它们开工
- 若 T4/D2 发现契约不够用，**回到本任务改契约**，不要在下游绕过
