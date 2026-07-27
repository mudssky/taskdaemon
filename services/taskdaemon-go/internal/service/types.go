package service

import (
	"context"
	"os"
)

// Scope 表示服务注册作用域。
type Scope string

const (
	// ScopeUser 用户级服务（macOS LaunchAgent / Linux systemd --user）。
	ScopeUser Scope = "user"
	// ScopeSystem 系统级服务（LaunchDaemon / system unit / Windows SCM）。
	ScopeSystem Scope = "system"
)

// Spec 描述一次安装请求的输入。
type Spec struct {
	// ConfigPath 显式 --config；空则由解析逻辑决定最终绝对路径。
	ConfigPath string
	// Scope 服务作用域；空则使用平台默认。
	Scope Scope
	// Executable 可执行文件绝对路径；空则解析当前二进制。
	Executable string
	// WorkingDir 工作目录绝对路径；空则使用用户主目录。
	WorkingDir string
	// Force 为 true 时允许覆盖已有单元。
	Force bool
}

// Unit 是将写入的服务单元描述。
type Unit struct {
	// Path 单元文件目标路径；Windows 为逻辑描述路径。
	Path string
	// Content 完整单元内容（plist / unit / 说明文本）。
	Content string
	// Mode 文件权限；0 表示使用默认 0644。
	Mode os.FileMode
	// Label 服务标识（launchd label / systemd unit / Windows service name）。
	Label string
	// ProgramArguments 启动参数列表（含可执行文件）。
	ProgramArguments []string
}

// Result 是 install/uninstall 的结果摘要。
type Result struct {
	// Action 执行的动作：install / uninstall。
	Action string `json:"action" yaml:"action"`
	// Scope 实际使用的作用域。
	Scope Scope `json:"scope" yaml:"scope"`
	// UnitPath 单元文件路径。
	UnitPath string `json:"unitPath" yaml:"unitPath"`
	// ConfigPath 写入单元的配置绝对路径。
	ConfigPath string `json:"configPath" yaml:"configPath"`
	// Executable 写入单元的可执行文件绝对路径。
	Executable string `json:"executable" yaml:"executable"`
	// Messages 平台提示（lingering、提权、生效时机等）。
	Messages []string `json:"messages,omitempty" yaml:"messages,omitempty"`
}

// Status 是服务注册与运行状态。
type Status struct {
	// Platform 当前平台标识（darwin/linux/windows/unsupported）。
	Platform string `json:"platform" yaml:"platform"`
	// Scope 查询的作用域。
	Scope Scope `json:"scope" yaml:"scope"`
	// Installed 是否已注册单元。
	Installed bool `json:"installed" yaml:"installed"`
	// Running 服务进程是否在运行。
	Running bool `json:"running" yaml:"running"`
	// UnitPath 单元路径（已安装时）。
	UnitPath string `json:"unitPath,omitempty" yaml:"unitPath,omitempty"`
	// Label 服务标识。
	Label string `json:"label,omitempty" yaml:"label,omitempty"`
	// Detail 平台原始状态摘要。
	Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

// Manager 是平台无关的服务管理接口。
type Manager interface {
	// Install 生成并注册服务单元，失败时逆序回滚。
	Install(ctx context.Context, spec Spec) (Result, error)
	// Uninstall 注销服务并清理单元文件；未安装时安全。
	Uninstall(ctx context.Context, scope Scope) (Result, error)
	// Status 查询注册状态与运行状态。
	Status(ctx context.Context, scope Scope) (Status, error)
	// Render 纯函数生成单元内容，零副作用。
	Render(spec Spec) (Unit, error)
	// DefaultScope 返回平台默认作用域。
	DefaultScope() Scope
	// Platform 返回平台名称。
	Platform() string
}

// ResolvedSpec 是解析后的绝对路径规格，供 Render/Install 共用。
type ResolvedSpec struct {
	Scope      Scope
	Executable string
	ConfigPath string
	WorkingDir string
	Force      bool
}
