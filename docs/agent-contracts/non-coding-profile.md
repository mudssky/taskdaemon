# 非 coding / general profile 契约（§5.5）

## 1. 协议表达

| 层 | 表达 |
|---|---|
| 创建 thread | `profile: "general"` 和/或 `workspaceBinding: false` |
| Thread 资源 | `profile`、`workspaceBound: false`、`workspaceRoot` 省略或 null |
| SessionOpts | `profile: 'general'`，`cwd?` 可省略，`workspaceBinding: false` |
| capabilities | `workspaceBinding` 反映默认；会话实际绑定以 Thread 字段为准 |

`workspaceBinding: false` 的会话必须能完成纯对话 run（G0：Pi 可用 flag 收敛；OMP 需 adapter 强制裁剪）。

## 2. coding 特化事件语义

| 事件 | general 或 workspaceBound=false |
|---|---|
| `file_change` / CUSTOM file_change | **不发送**（不是发空 path） |
| diff 预览相关 CUSTOM | **不发送** |
| 工具时间线 | 仍可发送（若有 MCP 工具） |
| REASONING_* | 按 capabilities.thinking |
| 代码块 Markdown | 文本内保留（无害），UI 不作为主形态 |

## 3. 前端按 profile 分支（约定）

1. 读取 `Thread.profile` 与 `Thread.workspaceBound`。  
2. 再读 `capabilities`。  
3. 渲染规则：

| UI 区域 | coding + bound | general / unbound |
|---|---|---|
| 文件树 / workspace 路径 | 显示 | **隐藏** |
| diff 面板 | 显示 | **隐藏** |
| 工具时间线 | 按 toolCallDetail | 同 |
| thinking 面板 | 按 thinking | 同 |
| 模型选择 | 按 modelSwitch | 同 |
| 默认欢迎文案 | coding 向 | general 向 |

4. **禁止**用「空数组文件列表」假装 general。  
5. **禁止**仅凭 `runtimeId` 分支 profile UI。

## 4. 默认工具集

- general：默认 `tools: "none"` 或仅 MCP 目录允许项（G6）；不预设 shell/fs 工具。  
- coding：由 adapter/gateway 策略 allowlist 决定，仍受 tool allowlist 约束（§6）。
