package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// TestOpsCleanup_ChannelHealthSamples verifies the ops cleanup loop enforces
// the fixed 24h retention on channel_health_samples (rows older than 24h are
// deleted; newer rows survive).
func TestOpsCleanup_ChannelHealthSamples(t *testing.T) {
	// Spin up in-memory SQLite backing both ent.Client (for seeding) and raw
	// *sql.DB (used by deleteOldRowsByID). Must share the underlying DB so
	// seeded rows are visible to the cleanup query.
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	now := time.Now().UTC()

	// Three samples: 25h ago (should be deleted), 23h ago and 1h ago (both kept).
	// bucket_ts values differ so the unique index doesn't collapse them.
	type seed struct {
		label     string
		createdAt time.Time
		bucketTs  time.Time
	}
	seeds := []seed{
		{"25h-ago", now.Add(-25 * time.Hour), now.Add(-25 * time.Hour).Truncate(time.Minute)},
		{"23h-ago", now.Add(-23 * time.Hour), now.Add(-23 * time.Hour).Truncate(time.Minute)},
		{"1h-ago", now.Add(-1 * time.Hour), now.Add(-1 * time.Hour).Truncate(time.Minute)},
	}
	for _, s := range seeds {
		_, err := client.ChannelHealthSample.Create().
			SetBucketTs(s.bucketTs).
			SetAccountID(1).
			SetGroupID(0).
			SetModel("claude-sonnet-4-5").
			SetSuccessCount(1).
			SetErrorCount(0).
			SetRateLimitedCount(0).
			SetOverloadedCount(0).
			SetLatencyP50Ms(100).
			SetSource("passive").
			SetCreatedAt(s.createdAt).
			Save(ctx)
		require.NoError(t, err, "seed %s", s.label)
	}

	// Sanity check: 3 rows seeded.
	count, err := client.ChannelHealthSample.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, count)

	// Run cleanup. Use a minimal simple-mode config so leader-lock / config-
	// driven paths short-circuit cleanly. channel_health_samples block runs
	// unconditionally regardless of retention config.
	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	svc := NewOpsCleanupService(nil, db, nil, cfg)
	counts, err := svc.runCleanupOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), counts.channelHealthSamples, "exactly one 25h-ago row should be deleted")

	// 23h-ago and 1h-ago survive; 25h-ago gone.
	remaining, err := client.ChannelHealthSample.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, remaining, 2)
	var cutoff = now.Add(-24 * time.Hour)
	for _, r := range remaining {
		require.True(t, r.CreatedAt.After(cutoff),
			"kept row created_at=%v must be newer than cutoff=%v", r.CreatedAt, cutoff)
	}
}
