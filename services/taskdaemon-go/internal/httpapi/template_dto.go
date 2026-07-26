package httpapi

import "taskdaemon/internal/template"

// templateRenderRequest 是渲染 API 请求体。
type templateRenderRequest struct {
	// Params 用户填写的模板参数。
	Params map[string]any `json:"params"`
}

// templateParamResponse 是参数定义 API 形状。
type templateParamResponse struct {
	Name        string             `json:"name"`
	Type        template.ParamType `json:"type"`
	Required    bool               `json:"required"`
	Default     any                `json:"default,omitempty"`
	EnumOptions []string           `json:"enumOptions,omitempty"`
	Help        string             `json:"help,omitempty"`
	Min         *float64           `json:"min,omitempty"`
	Max         *float64           `json:"max,omitempty"`
	Sensitive   bool               `json:"sensitive,omitempty"`
}

// templateDefinitionResponse 是模板详情/列表项。
type templateDefinitionResponse struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Scenario    string                  `json:"scenario"`
	RunnerType  string                  `json:"runnerType"`
	Params      []templateParamResponse `json:"params"`
}

// templateDefinitionFromDomain 将领域定义转为 API DTO（不含 Render 函数）。
//
// 参数:
//   - def: 领域模板定义。
//
// 返回值:
//   - templateDefinitionResponse: API 响应。
func templateDefinitionFromDomain(def template.Definition) templateDefinitionResponse {
	params := make([]templateParamResponse, 0, len(def.Params))
	for _, p := range def.Params {
		params = append(params, templateParamResponse{
			Name:        p.Name,
			Type:        p.Type,
			Required:    p.Required,
			Default:     p.Default,
			EnumOptions: p.EnumOptions,
			Help:        p.Help,
			Min:         p.Min,
			Max:         p.Max,
			Sensitive:   p.Sensitive || p.Type == template.ParamSecretRef,
		})
	}
	return templateDefinitionResponse{
		ID:          def.ID,
		Name:        def.Name,
		Description: def.Description,
		Scenario:    def.Scenario,
		RunnerType:  string(def.RunnerType),
		Params:      params,
	}
}
