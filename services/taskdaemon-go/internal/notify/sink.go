package notify

import (
	"context"
	"time"
)

// Sink 是通知投递通道的统一接口。
// 新增 sink 只需实现本接口并 Register 到 Bus，无需修改总线代码（C-2）。
type Sink interface {
	// Name 返回 sink 稳定名称，用于日志与状态查询。
	Name() string
	// Enabled 返回当前是否启用；可在运行时随配置变化。
	Enabled() bool
	// Deliver 投递单条事件；失败由 Bus 记录，不影响其他 sink。
	Deliver(ctx context.Context, event Event) error
}

// SinkStatus 描述单个 sink 的内存态可观测信息（进程重启后清零）。
type SinkStatus struct {
	Name            string     `json:"name"`
	Enabled         bool       `json:"enabled"`
	LastSuccessAt   *time.Time `json:"lastSuccessAt,omitempty"`
	LastFailureAt   *time.Time `json:"lastFailureAt,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
	SuccessCount    int64      `json:"successCount"`
	FailureCount    int64      `json:"failureCount"`
	LastDeliveredAt *time.Time `json:"lastDeliveredAt,omitempty"`
}

// sinkRuntime 保存单个 sink 的运行态。
type sinkRuntime struct {
	sink   Sink
	status SinkStatus
}

// Publisher 是总线对外的最小发布面，供 scheduler 等业务方注入。
type Publisher interface {
	Publish(ctx context.Context, event Event)
}
