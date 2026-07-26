# T5 系统服务安装器 — 技术设计

## 1. 分层

```text
internal/cli/service_commands.go     ← CLI 层：参数解析、输出格式化、确认交互
        │ 只调用下面的接口，不含任何平台分支
        ▼
internal/service/manager.go          ← 平台无关的编排：install/uninstall/status 流程
        │
        ├── internal/service/launchd.go    (darwin)
        ├── internal/service/systemd.go    (linux)
        └── internal/service/windows.go    (windows)
```

**硬约束**：`internal/cli/service_commands.go` 中**不出现任何 `runtime.GOOS` 判断**。平台差异全部在 `internal/service/` 内，通过构建标签或运行时 switch 分派。CLI 层只知道「安装成功/失败/需要提权」。

## 2. 接口

```go
type Manager interface {
    Install(ctx context.Context, spec Spec) (Result, error)
    Uninstall(ctx context.Context) (Result, error)
    Status(ctx context.Context) (Status, error)
    Render(spec Spec) (Unit, error)   // dry-run 用，纯函数
}

type Spec struct {
    ConfigPath  string   // 显式 --config，空则用解析结果
    Scope       Scope    // user | system
    Executable  string   // 绝对路径
    WorkingDir  string
}

type Unit struct {
    Path    string   // 将写入的路径
    Content string   // 完整内容
}
```

**`Render` 是纯函数**，这是可测试性的关键：单元测试只测「生成什么内容」，不做真实系统注册。

## 3. 服务作用域决策（R2 要求显式决策）

| 平台 | 默认 | 备选 | 决策依据 |
|---|---|---|---|
| macOS | **用户级 LaunchAgent**（`~/Library/LaunchAgents/`） | 系统级 LaunchDaemon | taskdaemon 是个人任务调度器，跑在用户会话下更符合预期；系统级需 root 且无法访问用户配置目录 |
| Linux | **用户级 systemd**（`~/.config/systemd/user/`） | 系统级 `/etc/systemd/system/` | 同上；用户级无需 sudo，且 `systemctl --user` 可用 |
| Windows | **系统服务**（SCM） | 计划任务（登录时触发） | Windows 无用户级服务概念；SCM 需管理员权限 |

**Windows 是唯一默认需要提权的平台**，CLI 必须在执行前检测并明确提示。

> ⚠️ Linux 用户级 systemd 的坑：默认情况下用户注销后服务会停止，除非启用 lingering（`loginctl enable-linger`）。**这个必须在文档和 CLI 输出中说明**，否则用户会以为服务坏了。

## 4. 配置路径一致性（R3 核心）

**最容易出错的地方**：装完服务后服务读到了另一份配置。

规则：

1. 若用户传了 `--config <path>`，把**绝对路径**写进服务单元
2. 否则调用现有的配置解析逻辑（`internal/config/paths.go` + `load.go`）拿到**实际会被加载的路径**，写绝对路径进单元
3. **绝不**在单元里写相对路径或依赖启动时的 cwd
4. `Render` 的输出中包含解析出的配置路径，dry-run 时用户可以核对

同理，可执行文件路径也必须是绝对路径（`os.Executable()` + `filepath.EvalSymlinks`）。

## 5. 幂等与原子性（R5）

### 重复 install

**决策：默认拒绝并提示**，而非静默覆盖。提供 `--force` 显式覆盖。

理由：静默覆盖会让用户失去「我改过的单元文件」而不自知。拒绝 + 明确提示 + 显式覆盖标志是更安全的默认。

### 中途失败的清理

`Install` 内部按步骤记录已产生的副作用，失败时逆序回滚：

```
1. 写单元文件      → 失败：无副作用
2. 注册到系统      → 失败：删除步骤 1 的文件
3. 启动服务        → 失败：注销步骤 2 + 删除步骤 1
```

**不留半装状态**是硬要求。回滚本身失败时，输出必须明确告知用户残留了什么、如何手动清理。

## 6. 权限检测

在执行任何有副作用的操作**之前**检测：

- Windows：检查当前进程是否 elevated
- Linux 系统级：检查是否 root 或有对应 polkit 权限
- macOS 系统级：同上

检测失败返回 `SERVICE_INSTALL_PERMISSION_DENIED`，错误信息中给出**可直接复制执行的补救命令**（如 `sudo taskdaemon service install --scope system`）。

**不擅自提权**：不调用 sudo、不弹 UAC。用户应该知道自己在做什么。

## 7. 安全

| 项 | 处置 |
|---|---|
| 单元文件内容 | 只含路径与启动参数，**不含任何 token/密码**。敏感配置留在配置文件中 |
| 文件权限 | 用户级 `0644`（目录 `0755`）；系统级按平台惯例。**不对其他用户可写** |
| 日志 | 沿用 `internal/logging` 的脱敏；单元内容输出前不需脱敏（本就不含敏感值），但配置路径要正常打印 |

## 8. dry-run

`--dry-run` 调用 `Render` 后打印：

```
将写入: /Users/x/Library/LaunchAgents/com.taskdaemon.plist
配置路径: /Users/x/.config/taskdaemon/config.yaml
可执行文件: /usr/local/bin/taskdaemon
作用域: user
--- 单元内容 ---
<完整内容>
```

**零副作用**，不检测权限、不碰文件系统（除了读取配置解析所需）。

## 9. 测试策略

| 层 | 测什么 | 怎么测 |
|---|---|---|
| `Render` | 生成的单元内容、路径、权限位 | 纯函数单测，三平台各一组，golden file 对比 |
| 配置路径解析 | 各种 `--config` 与默认场景下写入的路径 | 单测 + 表驱动 |
| 回滚逻辑 | 每个步骤失败时的清理行为 | 注入失败的假实现 |
| 真实注册 | 装得上、起得来、卸得掉 | **手动验证**，不进 CI |

**CI 里不安装真实服务** —— 会污染 runner 环境且不可靠。手动验证记录归档到任务目录。

## 10. 兼容性与回滚

- 纯新增 CLI 子命令，不改现有行为 → 无破坏性变更
- 用户回滚：`taskdaemon service uninstall`
- 代码回滚：revert 即可；已安装的服务需用户手动清理，文档中给出各平台的手动清理命令

## 11. 风险

| 风险 | 处置 |
|---|---|
| **配置路径不一致**（装完读到另一份配置） | §4 的绝对路径规则 + dry-run 可核对 |
| **Linux 用户级服务注销即停** | 文档 + CLI 输出中说明 lingering |
| 半装状态 | §5 的逆序回滚 |
| 静默覆盖用户改过的单元 | 默认拒绝，需 `--force` |
| 平台分支泄漏到 CLI 层 | §1 的硬约束，review 时检查 |
| CI 装真实服务污染环境 | §9：真实注册只手动验证 |
