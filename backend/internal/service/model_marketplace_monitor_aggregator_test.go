package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModelMarketplaceListUserViewSingleflightAndCache(t *testing.T) {
	repo := &stubModelMarketplaceMonitorRepo{
		monitors: []*ModelMarketplaceMonitor{{
			ID:            1,
			Name:          "test monitor",
			Provider:      "openai",
			Endpoint:      "https://example.com",
			PrimaryModel:  "gpt-test",
			Enabled:       true,
			EffectiveRate: 1,
		}},
		listDelay: 50 * time.Millisecond,
	}
	svc := NewModelMarketplaceMonitorService(repo, nil, nil)

	const workers = 50
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			views, err := svc.ListUserView(context.Background())
			require.NoError(t, err)
			require.Len(t, views, 1)
		}()
	}
	wg.Wait()
	require.Equal(t, int64(1), repo.listEnabledCalls.Load())

	views, err := svc.ListUserView(context.Background())
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Equal(t, int64(1), repo.listEnabledCalls.Load())

	require.NoError(t, svc.Delete(context.Background(), 1))
	views, err = svc.ListUserView(context.Background())
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Equal(t, int64(2), repo.listEnabledCalls.Load())
}

type stubModelMarketplaceMonitorRepo struct {
	monitors         []*ModelMarketplaceMonitor
	listDelay        time.Duration
	listEnabledCalls atomic.Int64
}

func (r *stubModelMarketplaceMonitorRepo) Create(context.Context, *ModelMarketplaceMonitor) error {
	return nil
}

func (r *stubModelMarketplaceMonitorRepo) GetByID(context.Context, int64) (*ModelMarketplaceMonitor, error) {
	return &ModelMarketplaceMonitor{}, nil
}

func (r *stubModelMarketplaceMonitorRepo) Update(context.Context, *ModelMarketplaceMonitor) error {
	return nil
}

func (r *stubModelMarketplaceMonitorRepo) Delete(context.Context, int64) error { return nil }

func (r *stubModelMarketplaceMonitorRepo) List(context.Context, ModelMarketplaceMonitorListParams) ([]*ModelMarketplaceMonitor, int64, error) {
	return r.monitors, int64(len(r.monitors)), nil
}

func (r *stubModelMarketplaceMonitorRepo) ListEnabled(context.Context) ([]*ModelMarketplaceMonitor, error) {
	r.listEnabledCalls.Add(1)
	time.Sleep(r.listDelay)
	return r.monitors, nil
}

func (r *stubModelMarketplaceMonitorRepo) MarkChecked(context.Context, int64, time.Time) error {
	return nil
}

func (r *stubModelMarketplaceMonitorRepo) InsertHistoryBatch(context.Context, []*ModelMarketplaceMonitorHistoryRow) error {
	return nil
}

func (r *stubModelMarketplaceMonitorRepo) DeleteHistoryBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (r *stubModelMarketplaceMonitorRepo) ListHistory(context.Context, int64, string, int) ([]*ModelMarketplaceMonitorHistoryEntry, error) {
	return nil, nil
}

func (r *stubModelMarketplaceMonitorRepo) ListLatestPerModel(context.Context, int64) ([]*ModelMarketplaceMonitorLatest, error) {
	return nil, nil
}

func (r *stubModelMarketplaceMonitorRepo) ComputeAvailability(context.Context, int64, int) ([]*ModelMarketplaceMonitorAvailability, error) {
	return nil, nil
}

func (r *stubModelMarketplaceMonitorRepo) ListLatestForMonitorIDs(context.Context, []int64) (map[int64][]*ModelMarketplaceMonitorLatest, error) {
	return map[int64][]*ModelMarketplaceMonitorLatest{}, nil
}

func (r *stubModelMarketplaceMonitorRepo) ComputeAvailabilityForMonitors(context.Context, []int64, int) (map[int64][]*ModelMarketplaceMonitorAvailability, error) {
	return map[int64][]*ModelMarketplaceMonitorAvailability{}, nil
}

func (r *stubModelMarketplaceMonitorRepo) ListRecentHistoryForMonitors(context.Context, []int64, map[int64]string, int) (map[int64][]*ModelMarketplaceMonitorHistoryEntry, error) {
	return map[int64][]*ModelMarketplaceMonitorHistoryEntry{}, nil
}

func (r *stubModelMarketplaceMonitorRepo) ListRecentHistoryForMonitorModels(context.Context, map[int64][]string, int) (map[int64]map[string][]*ModelMarketplaceMonitorHistoryEntry, error) {
	return map[int64]map[string][]*ModelMarketplaceMonitorHistoryEntry{}, nil
}
