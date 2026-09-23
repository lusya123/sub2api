package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestModelMarketplaceListLatestForMonitorIDsUsesBoundedIndexProbes(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	checkedAt := time.Date(2026, time.September, 23, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)targets AS .*JOIN LATERAL .*ORDER BY h\.checked_at DESC.*LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"monitor_id", "model", "status", "latency_ms", "ping_latency_ms", "checked_at",
		}).AddRow(int64(9), "claude-sonnet", "operational", 123, 45, checkedAt))

	repo := &modelMarketplaceMonitorRepository{db: db}
	latest, err := repo.ListLatestForMonitorIDs(context.Background(), []int64{9, 11})

	require.NoError(t, err)
	require.Len(t, latest[9], 1)
	require.Equal(t, "claude-sonnet", latest[9][0].Model)
	require.Equal(t, "operational", latest[9][0].Status)
	require.Equal(t, 123, *latest[9][0].LatencyMs)
	require.Equal(t, checkedAt, latest[9][0].CheckedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelMarketplaceListRecentHistoryForMonitorsLimitsInsideLateralProbe(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	checkedAt := time.Date(2026, time.September, 23, 8, 1, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)WITH targets AS .*JOIN LATERAL .*ORDER BY h\.checked_at DESC.*LIMIT \$3`).
		WillReturnRows(sqlmock.NewRows([]string{
			"monitor_id", "status", "latency_ms", "ping_latency_ms", "checked_at",
		}).AddRow(int64(9), "degraded", 456, nil, checkedAt))

	repo := &modelMarketplaceMonitorRepository{db: db}
	history, err := repo.ListRecentHistoryForMonitors(
		context.Background(),
		[]int64{9},
		map[int64]string{9: "claude-sonnet"},
		200,
	)

	require.NoError(t, err)
	require.Len(t, history[9], 1)
	require.Equal(t, "degraded", history[9][0].Status)
	require.Equal(t, 456, *history[9][0].LatencyMs)
	require.Nil(t, history[9][0].PingLatencyMs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelMarketplaceListRecentHistoryForMonitorModelsLimitsInsideLateralProbe(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	checkedAt := time.Date(2026, time.September, 23, 8, 2, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)WITH targets AS .*JOIN LATERAL .*ORDER BY h\.checked_at DESC.*LIMIT \$3`).
		WillReturnRows(sqlmock.NewRows([]string{
			"monitor_id", "model", "status", "latency_ms", "ping_latency_ms", "checked_at",
		}).AddRow(int64(11), "gpt-5", "operational", nil, 31, checkedAt))

	repo := &modelMarketplaceMonitorRepository{db: db}
	history, err := repo.ListRecentHistoryForMonitorModels(
		context.Background(),
		map[int64][]string{11: {"gpt-5"}},
		200,
	)

	require.NoError(t, err)
	require.Len(t, history[11]["gpt-5"], 1)
	require.Equal(t, "operational", history[11]["gpt-5"][0].Status)
	require.Nil(t, history[11]["gpt-5"][0].LatencyMs)
	require.Equal(t, 31, *history[11]["gpt-5"][0].PingLatencyMs)
	require.NoError(t, mock.ExpectationsWereMet())
}
