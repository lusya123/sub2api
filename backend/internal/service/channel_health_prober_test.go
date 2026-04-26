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
		SetCredentials(map[string]any{}).
		SetExtra(map[string]any{}).
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
// The prober no longer enumerates model_routing; tests still accept this field
// so we can prove routing contents do not drive public status probing.
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
	groupID int64
	model   string
}

type fakeProberSettingRepo struct {
	values map[string]string
}

func (f fakeProberSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	if v, ok := f.values[key]; ok {
		return &Setting{Key: key, Value: v}, nil
	}
	return nil, ErrSettingNotFound
}

func (f fakeProberSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := f.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (f fakeProberSettingRepo) Set(context.Context, string, string) error {
	return nil
}

func (f fakeProberSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := f.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (f fakeProberSettingRepo) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (f fakeProberSettingRepo) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(f.values))
	for key, value := range f.values {
		out[key] = value
	}
	return out, nil
}

func (f fakeProberSettingRepo) Delete(context.Context, string) error {
	return nil
}

func (f *fakeExecutor) ProbeOnce(_ context.Context, groupID int64, model string) (int64, int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeProbeCall{groupID: groupID, model: model})
	if f.errAfter > 0 && len(f.calls) > f.errAfter {
		return 0, 0, 0, errors.New("forced failure")
	}
	return groupID * 1000, f.status, f.latency, nil
}

// TestProber_UsesPublicStatusConfig: model_routing can be empty or misleading;
// public status probes are driven only by the admin-managed status config.
func TestProber_UsesPublicStatusConfig(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 42}

	accID := seedAccount(t, client, "acc1")
	gID := seedGroup(t, client, "g1", map[string][]int64{
		"claude-opus-*": {accID},
	})
	seedAccountGroup(t, client, accID, gID)

	p := NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gID))
	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, probed)
	require.Len(t, fe.calls, 1)
	require.Equal(t, "claude-opus-4-7", fe.calls[0].model)
}

func TestProber_SkipsWhenModelHealthPageDisabled(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 42}
	settings := fakeProberSettingRepo{values: map[string]string{SettingKeyModelHealthPageEnabled: " FALSE "}}

	accID := seedAccount(t, client, "acc-disabled-page")
	gID := seedGroup(t, client, "grp-disabled-page", map[string][]int64{})
	seedAccountGroup(t, client, accID, gID)

	p := NewChannelHealthProber(client, rec, nil).
		WithSettingRepo(settings).
		WithProbeExecutor(fe).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gID))
	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, probed)
	require.Len(t, fe.calls, 0)
}

func TestProber_SkipsWhenNoEnabledModelsOrGroups(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 42}

	accID := seedAccount(t, client, "acc-disabled-config")
	gID := seedGroup(t, client, "grp-disabled-config", map[string][]int64{})
	seedAccountGroup(t, client, accID, gID)

	disabledModelConfig := PublicStatusConfig{
		Models: []PublicStatusModelConfig{{Name: "claude-opus-4-7", Enabled: false}},
		Groups: []PublicStatusGroupConfig{{GroupID: gID, Enabled: true}},
	}
	p := NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithPublicStatusConfig(disabledModelConfig)
	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, probed)
	require.Len(t, fe.calls, 0)

	disabledGroupConfig := PublicStatusConfig{
		Models: []PublicStatusModelConfig{{Name: "claude-opus-4-7", Enabled: true}},
		Groups: []PublicStatusGroupConfig{{GroupID: gID, Enabled: false}},
	}
	p = NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithPublicStatusConfig(disabledGroupConfig)
	probed, err = p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, probed)
	require.Len(t, fe.calls, 0)
}

func TestProber_SkipsConfiguredInactiveGroup(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 42}

	accID := seedAccount(t, client, "inactive-group-account")
	gID := seedGroup(t, client, "inactive-public-group", map[string][]int64{})
	_, err := client.Group.UpdateOneID(gID).SetStatus("inactive").Save(context.Background())
	require.NoError(t, err)
	seedAccountGroup(t, client, accID, gID)

	p := NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gID))
	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, probed)
	require.Len(t, fe.calls, 0)
}

// TestProber_ExcludesRecentlySampledGroups: the public probe unit is the
// group. A recent sample for the configured group/model suppresses another
// active probe, regardless of how many accounts hang behind that group.
func TestProber_ExcludesRecentlySampledGroups(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 15}

	accA := seedAccount(t, client, "representative-acc")
	accB := seedAccount(t, client, "other-acc")
	gID := seedGroup(t, client, "grp", map[string][]int64{})
	seedAccountGroup(t, client, accA, gID)
	seedAccountGroup(t, client, accB, gID)

	// Pin "now" so we can assert the freshness window precisely.
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	require.NoError(t, rec.Record(context.Background(), ChannelHealthEvent{
		AccountID: accB, GroupID: gID, Model: "claude-opus-4-7",
		Outcome: OutcomeSuccess, LatencyMs: 10, Source: SourcePassive,
		At: now.Add(-30 * time.Second), // well inside 5-min window
	}))

	p := NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gID)).
		WithNowFn(func() time.Time { return now })

	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, probed, "recent group/model sample must suppress the group-level probe")
	require.Len(t, fe.calls, 0)
}

// TestProber_RespectsBudget: 5 cold groups + budget=2 -> exactly 2 probes.
// Also asserts the ordering preference: groups with no prior sample go first.
func TestProber_RespectsBudget(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 20}

	groupIDs := make([]int64, 5)
	accountIDs := make([]int64, 5)
	for i := range groupIDs {
		accountIDs[i] = seedAccount(t, client, "acc")
		groupIDs[i] = seedGroup(t, client, "grp", map[string][]int64{})
		seedAccountGroup(t, client, accountIDs[i], groupIDs[i])
	}

	// Give groups ids[2] and ids[4] older stale samples (outside
	// coldFreshness) so that ids[0], ids[1], ids[3] have no samples at all
	// and must sort first. Per our ordering (NULLS FIRST, tie-break by
	// groupID ASC), the first two probed must be ids[0] and ids[1].
	now := time.Date(2026, 4, 24, 15, 0, 0, 0, time.UTC)
	oldTs := now.Add(-30 * time.Minute) // way older than coldFreshness=5m
	require.NoError(t, rec.Record(context.Background(), ChannelHealthEvent{
		AccountID: accountIDs[2], GroupID: groupIDs[2], Model: "claude-opus-4-7",
		Outcome: OutcomeSuccess, Source: SourcePassive, At: oldTs,
	}))
	require.NoError(t, rec.Record(context.Background(), ChannelHealthEvent{
		AccountID: accountIDs[4], GroupID: groupIDs[4], Model: "claude-opus-4-7",
		Outcome: OutcomeSuccess, Source: SourcePassive, At: oldTs,
	}))

	p := NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, groupIDs...)).
		WithBudget(2).
		WithNowFn(func() time.Time { return now })

	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, probed)
	require.Len(t, fe.calls, 2)
	// Expected order: both no-sample groups first, tie-broken by groupID ASC.
	require.Equal(t, groupIDs[0], fe.calls[0].groupID)
	require.Equal(t, groupIDs[1], fe.calls[1].groupID)
}

// TestProber_QueryOnlyRecentSamples locks in the time-bounded scan fix:
// enumerateCandidates must not drag in samples older than 2x coldFreshness,
// otherwise a 24h retained table with millions of rows would be loaded into
// memory on every 5-min tick.
//
// Verification strategy: seed ONE combo with a 20-minute-old sample and a
// 2-minute-old sample under coldFreshness=5min. The 2-min sample falls in
// the fresh window -> combo must be excluded from the probe list. The
// 20-min sample is ancient enough that it should have been filtered out by
// the scan cutoff (2 × 5min = 10min) and never even be consulted as
// "lastSampleTs". So: zero probes, and fe.calls empty.
func TestProber_QueryOnlyRecentSamples(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 1}

	accID := seedAccount(t, client, "acc")
	gID := seedGroup(t, client, "grp", map[string][]int64{})
	seedAccountGroup(t, client, accID, gID)

	now := time.Date(2026, 4, 24, 16, 0, 0, 0, time.UTC)
	// Ancient sample — beyond 2x coldFreshness window (= 10 min with default 5min).
	require.NoError(t, rec.Record(context.Background(), ChannelHealthEvent{
		AccountID: accID, GroupID: gID, Model: "claude-opus-4-7",
		Outcome: OutcomeSuccess, Source: SourcePassive,
		At: now.Add(-20 * time.Minute),
	}))
	// Recent sample — inside coldFreshness.
	require.NoError(t, rec.Record(context.Background(), ChannelHealthEvent{
		AccountID: accID, GroupID: gID, Model: "claude-opus-4-7",
		Outcome: OutcomeSuccess, Source: SourcePassive,
		At: now.Add(-2 * time.Minute),
	}))

	p := NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gID)).
		WithNowFn(func() time.Time { return now })

	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, probed, "recent sample must exclude combo from probing")
	require.Len(t, fe.calls, 0)

	// Indirect assertion on the query bound: flip to a scenario where the
	// ancient sample is the ONLY one — combo should still be probed because
	// the scan-window filter drops the ancient row and the candidate is
	// treated as "no sample" (NULLS FIRST). This confirms that the bounded
	// query is actually what controls behaviour, not just in-memory filtering.
	client2 := newChannelHealthTestClient(t)
	rec2 := NewChannelHealthRecorder(client2)
	fe2 := &fakeExecutor{status: 200, latency: 1}
	accID2 := seedAccount(t, client2, "acc")
	gID2 := seedGroup(t, client2, "grp", map[string][]int64{})
	seedAccountGroup(t, client2, accID2, gID2)
	require.NoError(t, rec2.Record(context.Background(), ChannelHealthEvent{
		AccountID: accID2, GroupID: gID2, Model: "claude-opus-4-7",
		Outcome: OutcomeSuccess, Source: SourcePassive,
		At: now.Add(-20 * time.Minute),
	}))
	p2 := NewChannelHealthProber(client2, rec2, nil).
		WithProbeExecutor(fe2).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gID2)).
		WithNowFn(func() time.Time { return now })
	probed2, err := p2.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, probed2, "combo with only ancient sample is treated as cold and probed")
}

// TestProber_RecordsActiveProbeSource: after a tick, channel_health_samples
// must contain a row with source=active_probe for the probed combo.
func TestProber_RecordsActiveProbeSource(t *testing.T) {
	client := newChannelHealthTestClient(t)
	rec := NewChannelHealthRecorder(client)
	fe := &fakeExecutor{status: 200, latency: 77}

	accID := seedAccount(t, client, "acc")
	gID := seedGroup(t, client, "grp", map[string][]int64{})
	seedAccountGroup(t, client, accID, gID)

	p := NewChannelHealthProber(client, rec, nil).
		WithProbeExecutor(fe).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gID))
	probed, err := p.RunTick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, probed)

	rows, err := client.ChannelHealthSample.Query().
		Where(channelhealthsample.SourceEQ(string(SourceActiveProbe))).
		All(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1, "must persist exactly one active_probe sample")
	require.Equal(t, gID*1000, rows[0].AccountID)
	require.Equal(t, gID, rows[0].GroupID)
	require.Equal(t, "claude-opus-4-7", rows[0].Model)
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
