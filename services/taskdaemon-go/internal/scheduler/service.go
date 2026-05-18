package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"

	"taskdaemon/internal/data"
	"taskdaemon/internal/runner"
)

// Options 控制调度服务的执行器和时间来源。
type Options struct {
	Runner runner.Executor
	Now    func() time.Time
}

// Service 提供任务定义、手动触发、取消和执行历史写入能力。
type Service struct {
	store     *data.Store
	runner    runner.Executor
	now       func() time.Time
	mu        sync.Mutex
	running   map[int]context.CancelFunc
	scheduler gocron.Scheduler
	jobs      map[int]gocron.Job
}

// NewService 创建调度服务。
//
// 参数:
//   - store: 已完成 migration 的数据层实例。
//   - opts: runner 执行器和时间来源。
//
// 返回值:
//   - *Service: 可创建、触发和取消任务的调度服务。
func NewService(store *data.Store, opts Options) *Service {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Service{
		store:   store,
		runner:  opts.Runner,
		now:     opts.Now,
		running: make(map[int]context.CancelFunc),
		jobs:    make(map[int]gocron.Job),
	}
}
