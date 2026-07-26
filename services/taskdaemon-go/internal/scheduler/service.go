package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"

	"taskdaemon/internal/data"
	"taskdaemon/internal/notify"
	"taskdaemon/internal/runner"
)

// Publisher 是调度器对通知总线的最小依赖面。
// 决策 B：直接使用 notify.Event，避免在 scheduler 复制事件结构导致 C-2 契约漂移。
// 未注入时 Publish 为 noop，调度器测试无需构造完整 Bus。
type Publisher interface {
	Publish(ctx context.Context, event notify.Event)
}

// Options 控制调度服务的执行器、时间来源与通知发布器。
type Options struct {
	Runner    runner.Executor
	Now       func() time.Time
	Publisher Publisher // T2a：可选，默认 noop
}

// Service 提供任务定义、手动触发、取消和执行历史写入能力。
type Service struct {
	store     *data.Store
	runner    runner.Executor
	now       func() time.Time
	publisher Publisher
	mu        sync.Mutex
	running   map[int]context.CancelFunc
	scheduler gocron.Scheduler
	jobs      map[int]gocron.Job
}

// NewService 创建调度服务。
//
// 参数:
//   - store: 已完成 migration 的数据层实例。
//   - opts: runner 执行器、时间来源与可选 Publisher。
//
// 返回值:
//   - *Service: 可创建、触发和取消任务的调度服务。
func NewService(store *data.Store, opts Options) *Service {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Service{
		store:     store,
		runner:    opts.Runner,
		now:       opts.Now,
		publisher: opts.Publisher,
		running:   make(map[int]context.CancelFunc),
		jobs:      make(map[int]gocron.Job),
	}
}
