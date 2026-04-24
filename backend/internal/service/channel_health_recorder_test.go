package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/channelhealthsample"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

// newChannelHealthTestClient spins up an in-memory SQLite ent.Client with the
// schema migrated — mirrors the pattern used by other service tests.
func newChannelHealthTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestChannelHealthRecorder_UpsertSameBucketAccumulates(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	ctx := context.Background()

	// Three events inside the same 1-minute bucket for the same
	// (account, group, model) tuple.
	at := time.Date(2026, 4, 24, 10, 30, 15, 0, time.UTC)
	base := ChannelHealthEvent{
		AccountID: 101,
		GroupID:   7,
		Model:     "claude-sonnet-4-5",
		LatencyMs: 120,
		Source:    SourcePassive,
		At:        at,
	}

	ev1 := base
	ev1.Outcome = OutcomeSuccess
	require.NoError(t, rec.Record(ctx, ev1))

	ev2 := base
	ev2.Outcome = OutcomeSuccess
	ev2.At = at.Add(20 * time.Second) // same minute bucket
	require.NoError(t, rec.Record(ctx, ev2))

	ev3 := base
	ev3.Outcome = OutcomeRateLimited
	ev3.At = at.Add(40 * time.Second)
	require.NoError(t, rec.Record(ctx, ev3))

	rows, err := client.ChannelHealthSample.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "same bucket must collapse to one row")
	r := rows[0]
	require.Equal(t, 2, r.SuccessCount)
	require.Equal(t, 1, r.RateLimitedCount)
	require.Equal(t, 0, r.ErrorCount)
	require.Equal(t, 0, r.OverloadedCount)
	// bucket_ts must be floored to the minute
	require.True(t, r.BucketTs.Equal(time.Date(2026, 4, 24, 10, 30, 0, 0, time.UTC)),
		"bucket_ts should floor to the minute, got %v", r.BucketTs)
}

func TestChannelHealthRecorder_DifferentBucketsCreateRows(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	ctx := context.Background()

	at := time.Date(2026, 4, 24, 10, 30, 59, 0, time.UTC)
	ev1 := ChannelHealthEvent{
		AccountID: 55, GroupID: 0, Model: "claude-opus-4-7",
		Outcome: OutcomeSuccess, LatencyMs: 80, Source: SourcePassive, At: at,
	}
	require.NoError(t, rec.Record(ctx, ev1))

	ev2 := ev1
	ev2.At = at.Add(2 * time.Second) // crosses the minute boundary -> 10:31:01
	require.NoError(t, rec.Record(ctx, ev2))

	rows, err := client.ChannelHealthSample.Query().
		Order(dbent.Asc(channelhealthsample.FieldBucketTs)).All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2, "cross-minute events must create two rows")
	require.True(t, rows[0].BucketTs.Equal(time.Date(2026, 4, 24, 10, 30, 0, 0, time.UTC)))
	require.True(t, rows[1].BucketTs.Equal(time.Date(2026, 4, 24, 10, 31, 0, 0, time.UTC)))
}

func TestChannelHealthRecorder_OutcomeMapping(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	ctx := context.Background()

	at := time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC)
	base := ChannelHealthEvent{
		AccountID: 77, GroupID: 2, Model: "claude-haiku-4-5",
		LatencyMs: 50, Source: SourcePassive, At: at,
	}
	for _, o := range []HealthOutcome{
		OutcomeSuccess, OutcomeError, OutcomeRateLimited, OutcomeOverloaded,
	} {
		ev := base
		ev.Outcome = o
		require.NoError(t, rec.Record(ctx, ev))
	}

	rows, err := client.ChannelHealthSample.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	r := rows[0]
	require.Equal(t, 1, r.SuccessCount)
	require.Equal(t, 1, r.ErrorCount)
	require.Equal(t, 1, r.RateLimitedCount)
	require.Equal(t, 1, r.OverloadedCount)
}

// TestChannelHealthRecorder_ConcurrentSameBucket asserts that when N goroutines
// race to record OutcomeSuccess into the *same* (bucket, account, group, model)
// tuple, the final row has success_count == N and nothing is lost. The pre-fix
// implementation did SELECT-then-UPDATE/INSERT inside a transaction: under real
// gateway QPS, the unique index would fire and half the samples would silently
// disappear after the tx rolled back. Post-fix ON CONFLICT makes the upsert
// atomic in a single round-trip so this test exercises the regression.
func TestChannelHealthRecorder_ConcurrentSameBucket(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	ctx := context.Background()

	at := time.Date(2026, 4, 24, 13, 0, 0, 0, time.UTC)
	base := ChannelHealthEvent{
		AccountID: 501,
		GroupID:   11,
		Model:     "claude-opus-4-7",
		LatencyMs: 123,
		Source:    SourcePassive,
		At:        at,
		Outcome:   OutcomeSuccess,
	}

	const n = 10
	var wg sync.WaitGroup
	errs := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := rec.Record(ctx, base); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	rows, err := client.ChannelHealthSample.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "concurrent same-bucket inserts must collapse to one row")
	require.Equal(t, n, rows[0].SuccessCount,
		"every concurrent Record call must be reflected in success_count")
}

func TestChannelHealthRecorder_LatencyUsesMax(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	ctx := context.Background()

	at := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	ev1 := ChannelHealthEvent{
		AccountID: 9, GroupID: 0, Model: "claude-sonnet-4-5",
		Outcome: OutcomeSuccess, LatencyMs: 100, Source: SourcePassive, At: at,
	}
	require.NoError(t, rec.Record(ctx, ev1))

	ev2 := ev1
	ev2.LatencyMs = 50
	ev2.At = at.Add(10 * time.Second)
	require.NoError(t, rec.Record(ctx, ev2))

	rows, err := client.ChannelHealthSample.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 100, rows[0].LatencyP50Ms,
		"latency_p50_ms must keep the MAX of samples in the same bucket")
}
