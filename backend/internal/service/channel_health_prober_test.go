package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/channelhealthsample"
	"github.com/stretchr/testify/require"
)

// seedAccount creates a minimal schedulable account with the required fields
// that AccountCreate insists on at validation time. Returned id is the PK.
func seedAccount(t *testing.T, client *dbent.Client, name string) int64 {
	t.Helper()
	acc, err := client.Account.Create().
		SetName(name).
		SetPlatform("anthropic").
		SetType("oauth").
		SetCredentials(map[string]interface{}{}).
		SetExtra(map[string]interface{}{}).
		SetConcurrency(1).
		SetPriority(50).
		SetRateMultiplier(1.0).
		SetStatus("active").
		SetAutoPauseOnExpired(false).
		SetSchedulable(true).
		Save(context.Background())
	require.NoError(t, err)
	return acc.ID
}

// seedGroup creates a group with the given model_routing map, no soft-delete.
// model_routing_enabled is irrelevant to the prober (it enumerates regardless
// — the prober's job is to cover cold combos the routing intends to use).
func seedGroup(t *testing.T, client *dbent.Client, name string, routing map[string][]int64) int64 {
	t.Helper()
	g, err := client.Group.Create().
		SetName(name).
		SetRateMultiplier(1.0).
		SetModelRouting(routing).
		Save(context.Background())
	require.NoError(t, err)
	return g.ID
}

// seedAccountGroup wires account -> group in the edge table.
func seedAccountGroup(t *testing.T, client *dbent.Client, accountID, groupID int64) {
	t.Helper()
	_, err := client.AccountGroup.Create().
		SetAccountID(accountID).
		SetGroupID(groupID).
		Save(context.Background())
	require.NoError(t, err)
}

// fakeExecutor records every ProbeOnce invocation and returns a canned
// status/latency. Thread-safety is trivial since the prober is serial.
type fakeExecutor struct {
	mu       sync.Mutex
	calls    []fakeProbeCall
	status   int
	latency  int
	errAfter int // if >0, return err after N calls
}

type fakeProbeCall struct {
	accountID int64
	model     string
}

func (f *fakeExecutor) ProbeOnce(_ context.Context, accountID int64, model string) (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeProbeCall{accountID: accountID, model: model})
	if f.errAfter > 0 && len(f.calls) > f.errAfter {
		return 0, 0, errors.New("forced failure")
	}
	return f.status, f.latency, nil
}

// TestProber_SkipsWildcardPatterns: routing has one concrete key and one
// wildcard key — the prober must enumerate only the concrete one.
func TestProber_SkipsWildcardPatterns(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 42}

	accID := seedAccount(t, client, "acc1")
	// One group maps both a wildcard and a concrete model.
	gID := seedGroup(t, client, "g1", map[string][]int64{
		"claude-opus-*": {accID},
		"gpt-4o":        {accID},
	})
	seedAccountGroup(t, client, accID, gID)

	p := NewChannelHealthProber(client, rec, nil).WithProbeExecutor(fe)
	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, probed, "only the non-wildcard model should be probed")
	require.Len(t, fe.calls, 1)
	require.Equal(t, "gpt-4o", fe.calls[0].model)
}

// TestProber_ExcludesRecentlySampledCombos: combo A has a passive sample in
// the last 2 minutes (< coldFreshness); combo B has no recent sample. Only B
// should be probed.
func TestProber_ExcludesRecentlySampledCombos(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 15}

	accA := seedAccount(t, client, "warm-acc")
	accB := seedAccount(t, client, "cold-acc")
	gID := seedGroup(t, client, "grp", map[string][]int64{
		"gpt-4o": {accA, accB},
	})
	seedAccountGroup(t, client, accA, gID)
	seedAccountGroup(t, client, accB, gID)

	// Pin "now" so we can assert the freshness window precisely.
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	// A has a recent passive sample; B has none.
	require.NoError(t, rec.Record(context.Background(), ChannelHealthEvent{
		AccountID: accA, GroupID: gID, Model: "gpt-4o",
		Outcome: OutcomeSuccess, LatencyMs: 10, Source: SourcePassive,
		At: now.Add(-30 * time.Second), // well inside 5-min window
	}))

	p := NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithNowFn(func() time.Time { return now })

	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, probed, "only the cold combo should be probed")
	require.Len(t, fe.calls, 1)
	require.Equal(t, accB, fe.calls[0].accountID)
}

// TestProber_RespectsBudget: 5 cold combos + budget=2 → exactly 2 probes.
// Also asserts the ordering preference: combos with no prior sample go first.
func TestProber_RespectsBudget(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 20}

	// 5 accounts, each in the same group that routes a single concrete model.
	ids := make([]int64, 5)
	for i := range ids {
		ids[i] = seedAccount(t, client, "acc")
	}
	gID := seedGroup(t, client, "grp", map[string][]int64{
		"gpt-4o": ids,
	})
	for _, id := range ids {
		seedAccountGroup(t, client, id, gID)
	}

	// Give accounts ids[2] and ids[4] older stale samples (outside
	// coldFreshness) so that ids[0], ids[1], ids[3] have no samples at all
	// and must sort first. Per our ordering (NULLS FIRST, tie-break by
	// accountID ASC), the first two probed must be ids[0] and ids[1].
	now := time.Date(2026, 4, 24, 15, 0, 0, 0, time.UTC)
	oldTs := now.Add(-30 * time.Minute) // way older than coldFreshness=5m
	require.NoError(t, rec.Record(context.Background(), ChannelHealthEvent{
		AccountID: ids[2], GroupID: gID, Model: "gpt-4o",
		Outcome: OutcomeSuccess, Source: SourcePassive, At: oldTs,
	}))
	require.NoError(t, rec.Record(context.Background(), ChannelHealthEvent{
		AccountID: ids[4], GroupID: gID, Model: "gpt-4o",
		Outcome: OutcomeSuccess, Source: SourcePassive, At: oldTs,
	}))

	p := NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithBudget(2).
		WithNowFn(func() time.Time { return now })

	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, probed)
	require.Len(t, fe.calls, 2)
	// Expected order: both no-sample combos first, tie-broken by accountID ASC.
	require.Equal(t, ids[0], fe.calls[0].accountID)
	require.Equal(t, ids[1], fe.calls[1].accountID)
}

// TestProber_RecordsActiveProbeSource: after a tick, channel_health_samples
// must contain a row with source=active_probe for the probed combo.
func TestProber_RecordsActiveProbeSource(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 77}

	accID := seedAccount(t, client, "acc")
	gID := seedGroup(t, client, "grp", map[string][]int64{
		"gpt-4o": {accID},
	})
	seedAccountGroup(t, client, accID, gID)

	p := NewChannelHealthProber(client, rec, nil).WithProbeExecutor(fe)
	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, probed)

	rows, err := client.ChannelHealthSample.Query().
		Where(channelhealthsample.SourceEQ(string(SourceActiveProbe))).
		All(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1, "must persist exactly one active_probe sample")
	require.Equal(t, accID, rows[0].AccountID)
	require.Equal(t, gID, rows[0].GroupID)
	require.Equal(t, "gpt-4o", rows[0].Model)
	require.Equal(t, 1, rows[0].SuccessCount, "200 status maps to OutcomeSuccess")
	require.Equal(t, 77, rows[0].LatencyP50Ms)
}

// TestProber_NoOpOnNilDeps guards the wire-safety property: nil recorder or
// nil executor must never blow up the 5-minute cron.
func TestProber_NoOpOnNilDeps(t *testing.T) {
	client := newChannelHealthTestClient(t)

	// nil recorder
	p1 := NewChannelHealthProber(client, nil, nil)
	n, err := p1.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, n)

	// nil executor (tester)
	rec := NewChannelHealthRecorder(client)
	p2 := NewChannelHealthProber(client, rec, nil)
	n, err = p2.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, n)

	// nil prober
	var p3 *ChannelHealthProber
	n, err = p3.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, n)
}
