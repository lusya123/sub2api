package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type modelMarketplaceRunnerSvc interface {
	ListEnabledMonitors(ctx context.Context) ([]*ModelMarketplaceMonitor, error)
	RunCheck(ctx context.Context, id int64) ([]*ModelMarketplaceCheckResult, error)
}

type ModelMarketplaceMonitorRunner struct {
	svc       modelMarketplaceRunnerSvc
	mu        sync.Mutex
	tasks     map[int64]context.CancelFunc
	inFlight  map[int64]struct{}
	started   bool
	startOnce sync.Once
	stopOnce  sync.Once
	stopAll   context.CancelFunc
}

func NewModelMarketplaceMonitorRunner(svc *ModelMarketplaceMonitorService) *ModelMarketplaceMonitorRunner {
	ctx, cancel := context.WithCancel(context.Background())
	_ = ctx
	return &ModelMarketplaceMonitorRunner{
		svc:      svc,
		tasks:    map[int64]context.CancelFunc{},
		inFlight: map[int64]struct{}{},
		stopAll:  cancel,
	}
}

func (r *ModelMarketplaceMonitorRunner) Start() {
	r.startOnce.Do(func() {
		r.mu.Lock()
		r.started = true
		r.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), modelMarketplaceStartupLoadTimeout)
		defer cancel()
		monitors, err := r.svc.ListEnabledMonitors(ctx)
		if err != nil {
			slog.Warn("model_marketplace_monitor: startup load failed", "error", err)
			return
		}
		for _, m := range monitors {
			r.Schedule(m)
		}
	})
}

func (r *ModelMarketplaceMonitorRunner) Schedule(m *ModelMarketplaceMonitor) {
	if r == nil || m == nil {
		return
	}
	r.mu.Lock()
	started := r.started
	r.mu.Unlock()
	if !started {
		return
	}
	r.Unschedule(m.ID)
	if !m.Enabled || m.IntervalSeconds <= 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.tasks[m.ID] = cancel
	r.mu.Unlock()
	go r.runScheduled(ctx, m.ID, m.Name, time.Duration(m.IntervalSeconds)*time.Second)
}

func (r *ModelMarketplaceMonitorRunner) Unschedule(id int64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel := r.tasks[id]
	delete(r.tasks, id)
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *ModelMarketplaceMonitorRunner) Stop() {
	r.stopOnce.Do(func() {
		if r.stopAll != nil {
			r.stopAll()
		}
		r.mu.Lock()
		cancels := make([]context.CancelFunc, 0, len(r.tasks))
		for id, cancel := range r.tasks {
			cancels = append(cancels, cancel)
			delete(r.tasks, id)
		}
		r.mu.Unlock()
		for _, cancel := range cancels {
			cancel()
		}
	})
}

func (r *ModelMarketplaceMonitorRunner) runScheduled(ctx context.Context, id int64, name string, interval time.Duration) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			r.fire(ctx, id, name)
			timer.Reset(interval)
		}
	}
}

func (r *ModelMarketplaceMonitorRunner) fire(ctx context.Context, id int64, name string) {
	if !r.tryAcquireInFlight(id) {
		return
	}
	defer r.releaseInFlight(id)
	runCtx, cancel := context.WithTimeout(ctx, modelMarketplaceRequestTimeout+modelMarketplacePingTimeout+modelMarketplaceRunOneBuffer)
	defer cancel()
	if _, err := r.svc.RunCheck(runCtx, id); err != nil {
		slog.Warn("model_marketplace_monitor: scheduled check failed", "monitor_id", id, "name", name, "error", err)
	}
}

func (r *ModelMarketplaceMonitorRunner) tryAcquireInFlight(id int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.inFlight[id]; ok {
		return false
	}
	r.inFlight[id] = struct{}{}
	return true
}

func (r *ModelMarketplaceMonitorRunner) releaseInFlight(id int64) {
	r.mu.Lock()
	delete(r.inFlight, id)
	r.mu.Unlock()
}
