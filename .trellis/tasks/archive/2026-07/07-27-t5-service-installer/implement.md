# T5 系统服务安装器 — 执行计划

## 前置

- [ ] 读 `.trellis/spec/backend/index.md` 开发前检查第 1、2、4、6 项
- [ ] 读 `.trellis/spec/backend/configuration-runtime-guidelines.md`（配置路径解析是本任务核心）
- [ ] 读 `.trellis/spec/backend/error-handling.md`（`SERVICE_*` 错误码）
- [ ] 读 `internal/cli/command.go` 与 `config_commands.go`，确认子命令分发模式
- [ ] 从 `dev` 切分支 `feat/t5-service-installer`

## 阶段 1 · 接口与 Render（纯函数优先）

> 先做纯函数部分，它可测试、无副作用、且能尽早验证单元内容是否正确。

- [ ] 创建 `internal/service/` 包
- [ ] `manager.go`：`Manager` 接口、`Spec`、`Unit`、`Result`、`Status`、`Scope` 类型
- [ ] `SERVICE_*` 错误码定义
- [ ] 三平台的 `Render` 实现：
  - [ ] `launchd.go` — plist 生成
  - [ ] `systemd.go` — unit 文件生成
  - [ ] `windows.go` — SCM 参数生成
- [ ] 单测（golden file 对比）：
  - [ ] 三平台各生成一份单元，内容正确
  - [ ] `--config` 指定时写绝对路径
  - [ ] 未指定时写配置解析出的绝对路径
  - [ ] 可执行文件路径为绝对路径且已解析 symlink
  - [ ] 单元内容不含任何 token/密码

**验证**：`cd services/taskdaemon-go && go test ./internal/service/...`

## 阶段 2 · 配置路径一致性（本任务最易出错处）

- [ ] 复用 `internal/config/paths.go` 与 `load.go` 的解析逻辑，**不重新实现**
- [ ] 表驱动测试覆盖：
  - [ ] 显式 `--config /abs/path`
  - [ ] 显式 `--config ./relative`（应转绝对）
  - [ ] 无 `--config`，项目配置文件存在
  - [ ] 无 `--config`，回退用户配置目录
- [ ] 断言：写入单元的路径 == 服务启动后实际会加载的路径

**门**：这一步不过，后面全是白做 —— 装完读错配置是本任务的头号失败模式。

## 阶段 3 · 权限检测

- [ ] 各平台的提权状态检测
- [ ] 权限不足返回 `SERVICE_INSTALL_PERMISSION_DENIED`
- [ ] 错误信息含**可直接复制执行的补救命令**
- [ ] **不擅自提权**：不调 sudo、不弹 UAC
- [ ] 单测：模拟无权限场景返回正确错误码与提示

## 阶段 4 · Install / Uninstall / Status

- [ ] `Install`：按 design §5 分步执行，记录副作用
- [ ] **逆序回滚**：任一步失败清理前序副作用
- [ ] 回滚失败时明确告知残留内容与手动清理方式
- [ ] 重复 `install` **默认拒绝**，`--force` 显式覆盖
- [ ] `Uninstall`：对未安装状态安全（no-op 或明确提示）
- [ ] `Uninstall` 后无残留单元文件
- [ ] `Status`：注册状态 + 运行状态
- [ ] 不支持的平台/发行版返回明确错误，不静默失败
- [ ] 单测（注入失败的假实现）：
  - [ ] 每个步骤失败时的回滚行为
  - [ ] 重复 install 被拒绝
  - [ ] `--force` 可覆盖
  - [ ] uninstall 幂等

**验证**：`go test ./internal/service/...`

## 阶段 5 · CLI 层

- [ ] `internal/cli/service_commands.go`：`install` / `uninstall` / `status` 子命令
- [ ] 沿用现有分发模式（参考 `config_commands.go`）
- [ ] `--dry-run`：调用 `Render` 打印 design §8 的格式，**零副作用**
- [ ] `--scope user|system`、`--config`、`--force` 标志
- [ ] **核对：CLI 层无任何 `runtime.GOOS` 判断**（design §1 硬约束）
- [ ] 输出格式沿用 `internal/cli/output.go`
- [ ] 单测：参数解析、dry-run 零副作用

**验证**：`go test ./internal/cli/...`

## 阶段 6 · 平台特有提示

- [ ] **Linux**：输出中说明用户级 systemd 注销即停，给出 `loginctl enable-linger` 提示
- [ ] **Windows**：明确提示需要管理员权限
- [ ] **macOS**：说明用户级 LaunchAgent 的生效时机
- [ ] 三平台的作用域选择理由写入文档

## 阶段 7 · 安全审查

- [ ] 人工核对生成的单元文件不含 token/密码
- [ ] 文件权限：用户级 `0644`、目录 `0755`，不对其他用户可写
- [ ] 单测断言文件权限位
- [ ] 日志输出无敏感值

## 阶段 8 · 手动验证（三平台）

> CI 不装真实服务，本阶段全部手动，记录归档到任务目录。

macOS：

- [ ] `install` 成功，`launchctl list` 可见
- [ ] 服务实际运行，读取了正确的配置
- [ ] 崩溃后自动重启
- [ ] `uninstall` 后无残留

Linux：

- [ ] 同上（`systemctl --user status`）
- [ ] lingering 提示正确

Windows：

- [ ] 非管理员运行时提示正确
- [ ] 管理员运行时安装成功，`services.msc` 可见
- [ ] 同上其余项

- [ ] 三平台记录（系统版本 + 观察结果）写入 `research/manual-verification.md`

## 阶段 9 · 文档

- [ ] README 或 spec 增补：三平台作用域、权限要求、卸载方式、手动清理命令
- [ ] 回写路线图 §7.2 状态为 done

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
| G-1 | 阶段 2 结束 | **配置路径一致性**是否已用测试锁死 |
| G-2 | 阶段 4 结束 | 回滚逻辑是否覆盖每个失败点，无半装状态 |
| G-3 | 阶段 8 结束 | 三平台手动验证记录是否完整可信 |

## 回滚点

| 阶段 | 回滚方式 |
|---|---|
| 1–4 | 纯新增包，直接删除 |
| 5 | 删除 CLI 子命令文件与注册行 |
| 用户侧 | `taskdaemon service uninstall`；或按文档手动清理 |

## 与其他任务的协调

- **完全独立**：不依赖任何契约冻结，W1 波次可立即开工
- **T1a（C-1）**：若需要把服务状态放进设置页，那是后续任务；本任务只提供 CLI
- **D3（TD2）**：D3 的「Desktop 应用自启动」与本任务的「系统服务」**是两回事**。D3 的 PRD 已要求在文档与 UI 中区分，本任务的文档也应提及这个区别，避免用户困惑
- 共享文件：`internal/config/types.go`、`services/taskdaemon-go/package.json`（均 append-only）
