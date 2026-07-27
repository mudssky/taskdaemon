package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// 默认窗口几何（与 desktop.go 建窗常量对齐）。
const (
	defaultWindowWidth  = 1200
	defaultWindowHeight = 760
	defaultMinWidth     = 960
	defaultMinHeight    = 600
	// minVisibleEdge 窗口与任一屏 WorkArea 至少相交的像素边长，防止落在不可见区域。
	minVisibleEdge = 80
)

// WindowState 是持久化的窗口几何与最大化状态。
// 与配置文件分离，存放在用户数据目录。
type WindowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised"`
}

// DisplayRect 描述显示器工作区，供 clamp 使用（避免测试依赖 Wails Screen）。
type DisplayRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// DefaultWindowState 返回默认居中前的基准几何（位置 0,0，由调用方决定居中）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - WindowState: 默认宽高、未最大化。
func DefaultWindowState() WindowState {
	return WindowState{
		X:         0,
		Y:         0,
		Width:     defaultWindowWidth,
		Height:    defaultWindowHeight,
		Maximised: false,
	}
}

// WindowStatePath 返回窗口状态 JSON 路径。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: `$UserConfigDir/taskdaemon/window-state.json`；UserConfigDir 失败时用相对路径。
func WindowStatePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return filepath.Join(".", "taskdaemon-window-state.json")
	}
	return filepath.Join(configDir, "taskdaemon", "window-state.json")
}

// LoadWindowState 读取窗口状态；缺失或损坏时返回默认值与 ok=false。
//
// 参数:
//   - path: 状态文件路径。
//
// 返回值:
//   - WindowState: 有效状态或默认值。
//   - bool: true 表示成功从文件加载。
func LoadWindowState(path string) (WindowState, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultWindowState(), false
	}
	var state WindowState
	if err := json.Unmarshal(data, &state); err != nil {
		return DefaultWindowState(), false
	}
	if !isSaneWindowState(state) {
		return DefaultWindowState(), false
	}
	return state, true
}

// SaveWindowState 原子写入窗口状态。
//
// 参数:
//   - path: 目标路径。
//   - state: 待保存状态。
//
// 返回值:
//   - error: 写盘失败时返回。
func SaveWindowState(path string, state WindowState) error {
	if !isSaneWindowState(state) {
		return errors.New("invalid window state")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir window state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write window state tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename window state: %w", err)
	}
	return nil
}

// ClampWindowState 将窗口夹到可见显示器工作区内；无可用屏则回默认。
//
// 参数:
//   - state: 候选状态。
//   - displays: 显示器 WorkArea 列表。
//
// 返回值:
//   - WindowState: 可见且尺寸合法的状态。
//   - bool: true 表示使用了默认布局（无法可见或无显示器）。
func ClampWindowState(state WindowState, displays []DisplayRect) (WindowState, bool) {
	if !isSaneWindowState(state) {
		return DefaultWindowState(), true
	}
	if len(displays) == 0 {
		return DefaultWindowState(), true
	}
	if windowIntersectsAny(state, displays) {
		return state, false
	}
	// 落到主屏（第一块）工作区左上角内缩，避免完全不可见。
	primary := displays[0]
	clamped := state
	clamped.X = primary.X + 40
	clamped.Y = primary.Y + 40
	if clamped.Width > primary.Width {
		clamped.Width = max(defaultMinWidth, primary.Width-80)
	}
	if clamped.Height > primary.Height {
		clamped.Height = max(defaultMinHeight, primary.Height-80)
	}
	if !windowIntersectsAny(clamped, displays) {
		return DefaultWindowState(), true
	}
	return clamped, false
}

// isSaneWindowState 校验宽高下限与非荒谬坐标。
//
// 参数:
//   - state: 待校验状态。
//
// 返回值:
//   - bool: 是否可接受。
func isSaneWindowState(state WindowState) bool {
	if state.Width < defaultMinWidth || state.Height < defaultMinHeight {
		return false
	}
	if state.Width > 10000 || state.Height > 10000 {
		return false
	}
	if state.X < -20000 || state.Y < -20000 || state.X > 20000 || state.Y > 20000 {
		return false
	}
	return true
}

// windowIntersectsAny 判断窗口与任一显示器工作区是否有足够相交。
//
// 参数:
//   - state: 窗口状态。
//   - displays: 显示器列表。
//
// 返回值:
//   - bool: 是否可见。
func windowIntersectsAny(state WindowState, displays []DisplayRect) bool {
	for _, d := range displays {
		if rectsIntersectVisible(state.X, state.Y, state.Width, state.Height, d) {
			return true
		}
	}
	return false
}

// rectsIntersectVisible 判断两矩形相交区域是否达到 minVisibleEdge。
//
// 参数:
//   - x,y,w,h: 窗口矩形。
//   - d: 显示器工作区。
//
// 返回值:
//   - bool: 相交足够。
func rectsIntersectVisible(x, y, w, h int, d DisplayRect) bool {
	left := max(x, d.X)
	top := max(y, d.Y)
	right := min(x+w, d.X+d.Width)
	bottom := min(y+h, d.Y+d.Height)
	if right-left < minVisibleEdge || bottom-top < minVisibleEdge {
		return false
	}
	return true
}
