package template

import (
	"fmt"
	"sync"

	"taskdaemon/internal/runner"
)

// Registry 保存已注册模板；并发安全只读查询，注册通常在启动期。
type Registry struct {
	mu   sync.RWMutex
	byID map[string]Definition
	// order 保持注册顺序，便于列表稳定。
	order []string
}

// NewRegistry 创建空注册表。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *Registry: 空注册表。
func NewRegistry() *Registry {
	return &Registry{byID: make(map[string]Definition)}
}

// DefaultRegistry 返回含内置备份模板的注册表。
//
// 参数:
//   - 无。
//
// 返回值:
//   - *Registry: 内置模板注册表。
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	for _, def := range builtinDefinitions() {
		if err := reg.Register(def); err != nil {
			// 内置模板必须合法；启动期 panic 暴露编程错误。
			panic(fmt.Sprintf("register builtin template %s: %v", def.ID, err))
		}
	}
	return reg
}

// Register 注册模板；RunnerType 必须在 runner 白名单内。
//
// 参数:
//   - def: 模板定义。
//
// 返回值:
//   - error: 非法定义或 runner 越界。
func (r *Registry) Register(def Definition) error {
	if r == nil {
		return &DomainError{Code: CodeUnavailable, Message: "template registry is nil"}
	}
	if def.ID == "" {
		return &DomainError{Code: CodeRegisterRejected, Message: "template id is required"}
	}
	if def.Name == "" {
		return &DomainError{Code: CodeRegisterRejected, Message: "template name is required"}
	}
	if def.Render == nil {
		return &DomainError{Code: CodeRegisterRejected, Message: "template render function is required", Details: map[string]any{"id": def.ID}}
	}
	// 注册期强制 runner 白名单：用合法最小 config 探测类型。
	if err := runner.Validate(runner.Config{Type: def.RunnerType, Inline: "true"}); err != nil {
		return &DomainError{
			Code:    CodeRunnerUnsupported,
			Message: "template runner type is not supported",
			Details: map[string]any{"runnerType": string(def.RunnerType), "id": def.ID},
		}
	}
	for _, p := range def.Params {
		if p.Name == "" {
			return &DomainError{Code: CodeRegisterRejected, Message: "param name is required", Details: map[string]any{"id": def.ID}}
		}
		if !validParamType(p.Type) {
			return &DomainError{
				Code:    CodeRegisterRejected,
				Message: "param type is not in the allowed enum",
				Details: map[string]any{"id": def.ID, "param": p.Name, "type": string(p.Type)},
			}
		}
		if p.Type == ParamEnum && len(p.EnumOptions) == 0 {
			return &DomainError{Code: CodeRegisterRejected, Message: "enum param requires options", Details: map[string]any{"id": def.ID, "param": p.Name}}
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[def.ID]; exists {
		return &DomainError{Code: CodeRegisterRejected, Message: "template id already registered", Details: map[string]any{"id": def.ID}}
	}
	r.byID[def.ID] = def
	r.order = append(r.order, def.ID)
	return nil
}

// Get 按 id 查询模板。
//
// 参数:
//   - id: 模板标识。
//
// 返回值:
//   - Definition: 定义副本。
//   - error: 未找到时 TEMPLATE_NOT_FOUND。
func (r *Registry) Get(id string) (Definition, error) {
	if r == nil {
		return Definition{}, &DomainError{Code: CodeUnavailable, Message: "template registry is unavailable"}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.byID[id]
	if !ok {
		return Definition{}, &DomainError{Code: CodeNotFound, Message: "template not found", Details: map[string]any{"id": id}}
	}
	return def, nil
}

// List 按注册顺序返回全部模板。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []Definition: 模板列表。
func (r *Registry) List() []Definition {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Definition, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out
}

// validParamType 判断参数类型是否在有限枚举内。
//
// 参数:
//   - t: 参数类型。
//
// 返回值:
//   - bool: 是否合法。
func validParamType(t ParamType) bool {
	switch t {
	case ParamString, ParamNumber, ParamBoolean, ParamEnum, ParamPath, ParamSecretRef:
		return true
	default:
		return false
	}
}
