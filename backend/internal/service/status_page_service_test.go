package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

// fixedNow pins the clock at a convenient minute boundary so tests can reason
// about buckets without off-by-one pain from live wall time.
var fixedNow = time.Date(2026, 4, 24, 12, 30, 0, 0, time.UTC)

// seedSample writes one channel_health_samples row with explicit counts. Mirrors
// the ent column order used elsewhere in this package's tests.
func seedSample(
	t *testing.T,
	client *dbent.Client,
	bucketTs time.Time,
	accountID, groupID int64,
	model string,
	success, errors, ratelimited, overloaded int,
) {
	t.Helper()
	_, err := client.ChannelHealthSample.Create().
		SetBucketTs(bucketTs).
		SetAccountID(accountID).
		SetGroupID(groupID).
		SetModel(model).
		SetSuccessCount(success).
		SetErrorCount(errors).
		SetRateLimitedCount(ratelimited).
		SetOverloadedCount(overloaded).
		SetLatencyP50Ms(100).
		SetSource("passive").
		Save(context.Background())
	require.NoError(t, err)
}

// seedStatusFixture wires one account <-> one group routed to a model and
// returns their ids. Test cases layer samples on top.
func seedStatusFixture(t *testing.T, client *dbent.Client, model string) (accountID, groupID int64) {
	t.Helper()
	accountID = seedAccount(t, client, "status-test-account")
	groupID = seedGroup(t, client, "status-test-group", map[string][]int64{model: {accountID}})
	seedAccountGroup(t, client, accountID, groupID)
	return accountID, groupID
}

func testPublicStatusConfig(models []string, groupIDs ...int64) PublicStatusConfig {
	cfg := PublicStatusConfig{Models: make([]PublicStatusModelConfig, 0, len(models))}
	for _, model := range models {
		cfg.Models = append(cfg.Models, modelConfigFromCatalog(model))
	}
	for _, gid := range groupIDs {
		cfg.Groups = append(cfg.Groups, PublicStatusGroupConfig{
			GroupID: gid,
			Enabled: true,
		})
	}
	return cfg
}

func hasStatusModel(models []StatusModel, name string) bool {
	for _, model := range models {
		if model.Name == name {
			return true
		}
	}
	return false
}

func statusGroupNames(groups []StatusGroup) []string {
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		out = append(out, group.Name)
	}
	return out
}

func statusChannelNames(channels []StatusChannel) []string {
	out := make([]string, 0, len(channels))
	for _, channel := range channels {
		out = append(out, channel.Name)
	}
	return out
}

func TestStatusPage_AllGreen(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-sonnet-4-6"
	aid, gid := seedStatusFixture(t, client, model)

	// Fill every minute of the window with success samples.
	base := floorToMinute(fixedNow.Add(-time.Duration(statusWindowMinutes-1) * time.Minute))
	for i := 0; i < statusWindowMinutes; i++ {
		seedSample(t, client, base.Add(time.Duration(i)*time.Minute), aid, gid, model, 1, 0, 0, 0)
	}

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{model}, gid)).
		WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.NotNil(t, detail)

	require.Len(t, detail.Heartbeats, statusWindowMinutes)
	for i, b := range detail.Heartbeats {
		require.Equal(t, "ok", b.Status, "bucket %d must be ok", i)
	}
	require.InDelta(t, 100.0, detail.AvailabilityPct, 0.001)

	require.Len(t, detail.Groups, 1)
	require.Len(t, detail.Groups[0].Channels, 1)
	require.InDelta(t, 100.0, detail.Groups[0].Channels[0].AvailabilityPct, 0.001)
}

func TestStatusPage_PartiallyDegraded(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-opus-4-6"
	aid, gid := seedStatusFixture(t, client, model)

	// 60 buckets ok, 30 buckets 429 (degraded). Availability = 60/90 ≈ 66.67%.
	base := floorToMinute(fixedNow.Add(-time.Duration(statusWindowMinutes-1) * time.Minute))
	for i := 0; i < statusWindowMinutes; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		if i < 60 {
			seedSample(t, client, ts, aid, gid, model, 1, 0, 0, 0)
		} else {
			seedSample(t, client, ts, aid, gid, model, 0, 0, 1, 0)
		}
	}

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{model}, gid)).
		WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)

	okCount := 0
	degradedCount := 0
	for _, b := range detail.Heartbeats {
		switch b.Status {
		case "ok":
			okCount++
		case "degraded":
			degradedCount++
		}
	}
	require.Equal(t, 60, okCount)
	require.Equal(t, 30, degradedCount)
	// 60 ok / 90 with-sample = 66.666...
	require.InDelta(t, 66.6666, detail.AvailabilityPct, 0.01)
}

func TestStatusPage_NoSamples(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-haiku-4-5-20251001"
	_, gid := seedStatusFixture(t, client, model)

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{model}, gid)).
		WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Len(t, detail.Heartbeats, statusWindowMinutes)
	for _, b := range detail.Heartbeats {
		require.Equal(t, "unknown", b.Status)
	}
	// Convention: empty window reports 100% so frontend shows the grey bar
	// without a misleading "0% availability".
	require.InDelta(t, 100.0, detail.AvailabilityPct, 0.001)
}

func TestStatusPage_ConfiguredGroupDisplayName(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-opus-4-7"
	_, gid := seedStatusFixture(t, client, model)
	cfg := testPublicStatusConfig([]string{model}, gid)
	cfg.Groups[0].DisplayName = "Pro"

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(cfg).
		WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Len(t, detail.Groups, 1)
	require.Len(t, detail.Groups[0].Channels, 1)
	require.Equal(t, "Pro", detail.Groups[0].Name)
	require.Equal(t, "Pro", detail.Groups[0].Channels[0].Name)
}

func TestStatusPage_ConfiguredProbeLinesBecomeChannels(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-sonnet-4-6"
	aid, gid := seedStatusFixture(t, client, model)
	seedSample(t, client, floorToMinute(fixedNow.Add(-1*time.Minute)), aid, gid, model, 1, 0, 0, 0)

	cfg := testPublicStatusConfig([]string{model}, gid)
	cfg.Groups[0].DisplayName = "AWS"
	cfg.Groups[0].ProbeLines = []PublicStatusProbeLineConfig{
		{ID: "us", Name: "US", Region: "Virginia", Enabled: true, SortOrder: 2},
		{ID: "asia", Name: "Asia", Region: "Singapore", Enabled: true, SortOrder: 1},
		{ID: "disabled", Name: "Disabled", Enabled: false, SortOrder: 3},
	}

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(cfg).
		WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Len(t, detail.Groups, 1)
	require.Len(t, detail.Groups[0].Channels, 2)
	require.Equal(t, "Asia · Singapore", detail.Groups[0].Channels[0].Name)
	require.Equal(t, "US · Virginia", detail.Groups[0].Channels[1].Name)
	require.InDelta(t, 100.0, detail.Groups[0].Channels[0].AvailabilityPct, 0.01)
	require.InDelta(t, 100.0, detail.Groups[0].Channels[1].AvailabilityPct, 0.01)
}

func TestStatusPage_DerivesConfigFromLiveGroupsWhenNoSavedConfig(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-sonnet-4-6"

	accID := seedAccount(t, client, "live-claude-account")
	liteID := seedGroup(t, client, "lite-claude", map[string][]int64{})
	awsID := seedGroup(t, client, "aws", map[string][]int64{})
	monthlyID := seedGroup(t, client, "标准版 | 50美金/天 | 月卡", map[string][]int64{})
	noiseID := seedGroup(t, client, "内部压测", map[string][]int64{})
	_, err := client.Group.UpdateOneID(monthlyID).
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(context.Background())
	require.NoError(t, err)

	for _, gid := range []int64{liteID, awsID, monthlyID, noiseID} {
		seedAccountGroup(t, client, accID, gid)
	}
	seedSample(t, client, floorToMinute(fixedNow.Add(-1*time.Minute)), accID, liteID, model, 1, 0, 0, 0)
	seedSample(t, client, floorToMinute(fixedNow.Add(-1*time.Minute)), accID, awsID, model, 1, 0, 0, 0)
	seedSample(t, client, floorToMinute(fixedNow.Add(-1*time.Minute)), accID, monthlyID, model, 1, 0, 0, 0)

	svc := NewStatusPageService(client).WithNowFn(func() time.Time { return fixedNow })
	list, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, list)
	require.True(t, hasStatusModel(list, model))

	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Equal(t, []string{"AWS", "lite", "月卡"}, statusGroupNames(detail.Groups))
	require.Equal(t, []string{"US · Virginia", "EU · Frankfurt", "Asia · Singapore"}, statusChannelNames(detail.Groups[0].Channels))
}

func TestStatusPage_MonthlyGroupsAggregate(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-opus-4-7"
	aid1 := seedAccount(t, client, "monthly-a")
	aid2 := seedAccount(t, client, "monthly-b")
	g1 := seedGroup(t, client, "入门版 | 15美金/天 | 月卡", map[string][]int64{model: {aid1}})
	g2 := seedGroup(t, client, "团队版 | 200美金/天 | 月卡", map[string][]int64{model: {aid2}})
	seedAccountGroup(t, client, aid1, g1)
	seedAccountGroup(t, client, aid2, g2)
	seedSample(t, client, floorToMinute(fixedNow.Add(-1*time.Minute)), aid1, g1, model, 1, 0, 0, 0)
	seedSample(t, client, floorToMinute(fixedNow.Add(-2*time.Minute)), aid2, g2, model, 0, 0, 1, 0)

	cfg := testPublicStatusConfig([]string{model}, g1, g2)
	cfg.Groups[0].DisplayName = "月卡"
	cfg.Groups[0].AggregateKey = publicStatusAggregateMonthly
	cfg.Groups[1].DisplayName = "月卡"
	cfg.Groups[1].AggregateKey = publicStatusAggregateMonthly

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(cfg).
		WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Len(t, detail.Groups, 1)
	require.Len(t, detail.Groups[0].Channels, 1)
	require.Equal(t, "月卡", detail.Groups[0].Name)
	require.Equal(t, "月卡", detail.Groups[0].Channels[0].Name)
	require.InDelta(t, 50.0, detail.Groups[0].Channels[0].AvailabilityPct, 0.01)
	require.InDelta(t, 100.0, detail.Groups[0].LoadPct, 0.01)
}

func TestStatusPage_GroupDisplayOrder(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-sonnet-4-6"
	aid := seedAccount(t, client, "status-order-account")

	pro := seedGroup(t, client, "pro-claude", map[string][]int64{model: {aid}})
	monthly := seedGroup(t, client, "入门版 | 月卡", map[string][]int64{model: {aid}})
	aws := seedGroup(t, client, "aws", map[string][]int64{model: {aid}})
	light := seedGroup(t, client, "lite-claude", map[string][]int64{model: {aid}})
	special := seedGroup(t, client, "特惠-claude", map[string][]int64{model: {aid}})
	maxPure := seedGroup(t, client, "纯血MAX官转", map[string][]int64{model: {aid}})
	for _, gid := range []int64{pro, monthly, aws, light, special, maxPure} {
		seedAccountGroup(t, client, aid, gid)
	}

	cfg := testPublicStatusConfig([]string{model}, pro, monthly, aws, light, special, maxPure)
	for i := range cfg.Groups {
		switch cfg.Groups[i].GroupID {
		case maxPure:
			cfg.Groups[i].DisplayName = "MAX 纯血"
			cfg.Groups[i].AggregateKey = publicStatusAggregateMaxPure
		case aws:
			cfg.Groups[i].DisplayName = "AWS"
			cfg.Groups[i].AggregateKey = publicStatusAggregateAWS
		case light:
			cfg.Groups[i].DisplayName = "lite"
			cfg.Groups[i].AggregateKey = publicStatusAggregateLite
		case special:
			cfg.Groups[i].DisplayName = "特惠"
			cfg.Groups[i].AggregateKey = publicStatusAggregateSpecial
		case pro:
			cfg.Groups[i].DisplayName = "PRO"
			cfg.Groups[i].AggregateKey = publicStatusAggregatePro
		case monthly:
			cfg.Groups[i].DisplayName = "月卡"
			cfg.Groups[i].AggregateKey = publicStatusAggregateMonthly
		}
	}

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(cfg).
		WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Len(t, detail.Groups, 6)
	require.Equal(t, []string{"MAX 纯血", "AWS", "lite", "特惠", "PRO", "月卡"}, []string{
		detail.Groups[0].Name,
		detail.Groups[1].Name,
		detail.Groups[2].Name,
		detail.Groups[3].Name,
		detail.Groups[4].Name,
		detail.Groups[5].Name,
	})
}

func TestStatusPage_GroupConfigModelFilter(t *testing.T) {
	client := newChannelHealthTestClient(t)
	allowedModel := "claude-sonnet-4-6"
	blockedModel := "claude-opus-4-7"
	aid := seedAccount(t, client, "status-model-filter-account")
	common := seedGroup(t, client, "aws", map[string][]int64{blockedModel: {aid}})
	special := seedGroup(t, client, "特惠-claude", map[string][]int64{
		allowedModel: {aid},
		blockedModel: {aid},
	})
	seedAccountGroup(t, client, aid, common)
	seedAccountGroup(t, client, aid, special)

	cfg := testPublicStatusConfig([]string{allowedModel, blockedModel}, common, special)
	cfg.Groups[0].DisplayName = "AWS"
	cfg.Groups[0].AggregateKey = publicStatusAggregateAWS
	cfg.Groups[1].DisplayName = "特惠"
	cfg.Groups[1].AggregateKey = publicStatusAggregateSpecial
	cfg.Groups[1].Models = []string{allowedModel}

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(cfg).
		WithNowFn(func() time.Time { return fixedNow })
	allowed, err := svc.GetModelDetail(context.Background(), allowedModel)
	require.NoError(t, err)
	require.Len(t, allowed.Groups, 2)
	require.Equal(t, "AWS", allowed.Groups[0].Name)
	require.Equal(t, "特惠", allowed.Groups[1].Name)

	blocked, err := svc.GetModelDetail(context.Background(), blockedModel)
	require.NoError(t, err)
	require.Len(t, blocked.Groups, 1)
	require.Equal(t, "AWS", blocked.Groups[0].Name)
}

func TestStatusPage_ConfiguredGeminiDoesNotLeakIntoLegacyClaudeGroups(t *testing.T) {
	client := newChannelHealthTestClient(t)
	claudeModel := "claude-sonnet-4-6"
	geminiModel := "gemini-2.5-pro"
	aid, gid := seedStatusFixture(t, client, claudeModel)
	_, err := client.Group.UpdateOneID(gid).SetSupportedModelScopes([]string{}).Save(context.Background())
	require.NoError(t, err)
	seedSample(t, client, floorToMinute(fixedNow.Add(-1*time.Minute)), aid, gid, claudeModel, 1, 0, 0, 0)

	cfg := testPublicStatusConfig([]string{claudeModel, geminiModel}, gid)
	cfg.Models[1].Provider = "GOOGLE"
	cfg.Groups[0].DisplayName = "lite"
	cfg.Groups[0].AggregateKey = publicStatusAggregateLite

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(cfg).
		WithNowFn(func() time.Time { return fixedNow })

	models, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, claudeModel, models[0].Name)

	_, err = svc.GetModelDetail(context.Background(), geminiModel)
	require.ErrorIs(t, err, ErrStatusModelUnknown)
}

func TestPublicStatusGroupSuggestions(t *testing.T) {
	cases := []struct {
		name             string
		subscriptionType string
		wantName         string
		wantKey          string
	}{
		{name: "Claude Lite", subscriptionType: SubscriptionTypeStandard, wantName: "lite", wantKey: publicStatusAggregateLite},
		{name: "Claude Pro", subscriptionType: SubscriptionTypeStandard, wantName: "PRO", wantKey: publicStatusAggregatePro},
		{name: "月卡 Pro A", subscriptionType: SubscriptionTypeSubscription, wantName: "月卡", wantKey: publicStatusAggregateMonthly},
		{name: "特惠-claude", subscriptionType: SubscriptionTypeStandard, wantName: "特惠", wantKey: publicStatusAggregateSpecial},
		{name: "Max 主力", subscriptionType: SubscriptionTypeStandard, wantName: "MAX", wantKey: publicStatusAggregateMax},
		{name: "纯血MAX官转", subscriptionType: SubscriptionTypeStandard, wantName: "MAX 纯血", wantKey: publicStatusAggregateMaxPure},
		{name: "aws", subscriptionType: SubscriptionTypeStandard, wantName: "AWS", wantKey: publicStatusAggregateAWS},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotKey := suggestedPublicStatusGroup(&dbent.Group{
				Name:             tc.name,
				SubscriptionType: tc.subscriptionType,
			})
			require.Equal(t, tc.wantName, gotName)
			require.Equal(t, tc.wantKey, gotKey)
		})
	}
}

// TestStatusPage_UnknownModelReturnsSentinel: GetModelDetail on a model that
// isn't present in any group.model_routing short-circuits with
// ErrStatusModelUnknown instead of running the 4-query aggregation — this is
// the DoS fast-path.
func TestStatusPage_UnknownModelReturnsSentinel(t *testing.T) {
	client := newChannelHealthTestClient(t)
	gid := seedGroup(t, client, "g-known", map[string][]int64{
		"claude-opus-4-7": nil,
	})

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gid)).
		WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), "totally-made-up-model-that-does-not-exist")
	require.ErrorIs(t, err, ErrStatusModelUnknown)
	require.Nil(t, detail)
}

// TestStatusPage_UnknownModelDoesNotCachePoison: hitting GetModelDetail with
// a stream of junk names must not populate the detail cache with entries,
// both to avoid unbounded memory growth and to confirm the fast-path runs
// before the detail cache is consulted.
func TestStatusPage_UnknownModelDoesNotCachePoison(t *testing.T) {
	client := newChannelHealthTestClient(t)
	gid := seedGroup(t, client, "g-known", map[string][]int64{
		"claude-opus-4-7": nil,
	})

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gid)).
		WithNowFn(func() time.Time { return fixedNow })

	for i := 0; i < 50; i++ {
		name := "bogus-" + strconv.Itoa(i)
		_, err := svc.GetModelDetail(context.Background(), name)
		require.ErrorIs(t, err, ErrStatusModelUnknown)
	}
	svc.detailMu.RLock()
	size := len(svc.detailCache)
	svc.detailMu.RUnlock()
	require.Equal(t, 0, size, "unknown-model fast-path must not populate detail cache")
}

// TestStatusPage_CacheHits: two back-to-back ListModels calls share one DB
// Group.Query. We verify by creating a new group between the two calls and
// confirming the cached response is returned unchanged. A real miss would
// surface the new group.
func TestStatusPage_CacheHits(t *testing.T) {
	client := newChannelHealthTestClient(t)
	aid := seedAccount(t, client, "cache-hit-account")
	gid := seedGroup(t, client, "g-initial", map[string][]int64{
		"claude-opus-4-7": nil,
	})
	seedAccountGroup(t, client, aid, gid)

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gid)).
		WithNowFn(func() time.Time { return fixedNow })
	first, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, first, 1)

	// Mutate config in a way a non-cached call would observe immediately.
	svc.WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7", "claude-sonnet-4-6"}, gid))

	// Second call at the same fixed time hits cache; the new group is NOT
	// reflected.
	second, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, second, 1, "cache hit should not reflect post-cache DB mutation")
	require.Equal(t, first[0].Name, second[0].Name)
}

// TestStatusPage_CacheExpires: advancing the clock past the TTL forces a
// fresh DB read. The newly-seeded model surfaces.
func TestStatusPage_CacheExpires(t *testing.T) {
	client := newChannelHealthTestClient(t)
	aid := seedAccount(t, client, "cache-expire-account")
	gid := seedGroup(t, client, "g-initial", map[string][]int64{
		"claude-opus-4-7": nil,
	})
	seedAccountGroup(t, client, aid, gid)

	now := fixedNow
	svc := NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, gid)).
		WithNowFn(func() time.Time { return now })
	first, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, first, 1)

	svc.WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7", "claude-sonnet-4-6"}, gid))

	// Jump well past the 30s TTL.
	now = fixedNow.Add(statusCacheTTL + 5*time.Second)
	second, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, second, 2, "expired cache must re-read DB")
}

// TestStatusPage_DetailCacheHits: repeated GetModelDetail within the TTL
// reuses the cached aggregation. Seed a sample between the two calls and
// verify the Heartbeats snapshot does NOT update.
func TestStatusPage_DetailCacheHits(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-opus-4-7"
	aid, gid := seedStatusFixture(t, client, model)

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{model}, gid)).
		WithNowFn(func() time.Time { return fixedNow })

	// First call: empty window, all unknown.
	first, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.NotNil(t, first)
	for _, b := range first.Heartbeats {
		require.Equal(t, "unknown", b.Status)
	}

	// Add a sample — without cache this would flip at least one bucket to "ok".
	seedSample(t, client, floorToMinute(fixedNow.Add(-1*time.Minute)), aid, gid, model, 5, 0, 0, 0)

	// Second call (within TTL) must be served from cache → still all-unknown.
	second, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	for _, b := range second.Heartbeats {
		require.Equal(t, "unknown", b.Status, "detail cache hit must not reflect post-cache sample")
	}
}

func TestStatusPage_ListModels(t *testing.T) {
	client := newChannelHealthTestClient(t)
	// Two groups routing three total models — one duplicate across groups.
	aid1 := seedAccount(t, client, "list-models-a")
	aid2 := seedAccount(t, client, "list-models-b")
	g1 := seedGroup(t, client, "g-a", map[string][]int64{
		"claude-opus-4-7":   nil,
		"claude-opus-4-6":   nil,
		"claude-*-wildcard": nil, // must be filtered out
	})
	g2 := seedGroup(t, client, "g-b", map[string][]int64{
		"claude-opus-4-7":   nil,
		"claude-sonnet-4-6": nil,
	})
	seedAccountGroup(t, client, aid1, g1)
	seedAccountGroup(t, client, aid2, g2)

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7", "claude-opus-4-6", "claude-sonnet-4-6"}, g1, g2)).
		WithNowFn(func() time.Time { return fixedNow })
	models, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	names := make([]string, 0, len(models))
	for _, m := range models {
		names = append(names, m.Name)
	}
	require.ElementsMatch(t,
		[]string{"claude-opus-4-7", "claude-opus-4-6", "claude-sonnet-4-6"},
		names,
	)
	// Catalogue metadata is attached.
	for _, m := range models {
		if m.Name == "claude-opus-4-7" {
			require.Equal(t, "ANTHROPIC", m.Provider)
			require.True(t, m.PromptCaching)
			require.Greater(t, m.Pricing.InputPerMTok, 0.0)
		}
	}
}

func TestDefaultPublicStatusModelsIncludeGLMAndMiniMax(t *testing.T) {
	cfg := defaultPublicStatusConfig()
	names := make([]string, 0, len(cfg.Models))
	byName := make(map[string]PublicStatusModelConfig, len(cfg.Models))
	for _, model := range cfg.Models {
		names = append(names, model.Name)
		byName[model.Name] = model
	}

	require.Contains(t, names, "glm-5")
	require.Contains(t, names, "minimax-m2.5")
	require.Equal(t, "Z.AI", byName["glm-5"].Provider)
	require.Equal(t, "MINIMAX", byName["minimax-m2.5"].Provider)
	require.InDelta(t, 1.0, byName["glm-5"].Pricing.InputPerMTok, 1e-12)
	require.InDelta(t, 0.3, byName["minimax-m2.5"].Pricing.InputPerMTok, 1e-12)
}

func TestStatusPage_ExcludesConfiguredGroupWithoutSchedulableAccount(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-sonnet-4-6"
	aid, monitorableGroupID := seedStatusFixture(t, client, model)
	unmonitorableGroupID := seedGroup(t, client, "configured-without-account", map[string][]int64{model: {aid}})

	cfg := testPublicStatusConfig([]string{model}, monitorableGroupID, unmonitorableGroupID)
	cfg.Groups[0].DisplayName = "Monitorable"
	cfg.Groups[1].DisplayName = "NoAccount"

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(cfg).
		WithNowFn(func() time.Time { return fixedNow })

	models, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)

	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Len(t, detail.Groups, 1)
	require.Equal(t, "Monitorable", detail.Groups[0].Name)
}

func TestStatusPage_ConfiguredInactiveGroupIsHidden(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-sonnet-4-6"

	activeAccountID := seedAccount(t, client, "active-group-account")
	activeGroupID := seedGroup(t, client, "active-public-group", map[string][]int64{model: {activeAccountID}})
	seedAccountGroup(t, client, activeAccountID, activeGroupID)

	inactiveAccountID := seedAccount(t, client, "inactive-group-account")
	inactiveGroupID := seedGroup(t, client, "inactive-public-group", map[string][]int64{model: {inactiveAccountID}})
	_, err := client.Group.UpdateOneID(inactiveGroupID).SetStatus("inactive").Save(context.Background())
	require.NoError(t, err)
	seedAccountGroup(t, client, inactiveAccountID, inactiveGroupID)

	cfg := testPublicStatusConfig([]string{model}, activeGroupID, inactiveGroupID)
	cfg.Groups[0].DisplayName = "ACTIVE"
	cfg.Groups[1].DisplayName = "INACTIVE"

	svc := NewStatusPageService(client).
		WithPublicStatusConfig(cfg).
		WithNowFn(func() time.Time { return fixedNow })

	models, err := svc.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)

	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Len(t, detail.Groups, 1)
	require.Equal(t, "ACTIVE", detail.Groups[0].Name)
}
