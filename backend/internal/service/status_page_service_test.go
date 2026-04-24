package service

import (
	"context"
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

func TestStatusPage_AllGreen(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-sonnet-4-6"
	aid, gid := seedStatusFixture(t, client, model)

	// Fill every minute of the window with success samples.
	base := floorToMinute(fixedNow.Add(-time.Duration(statusWindowMinutes-1) * time.Minute))
	for i := 0; i < statusWindowMinutes; i++ {
		seedSample(t, client, base.Add(time.Duration(i)*time.Minute), aid, gid, model, 1, 0, 0, 0)
	}

	svc := NewStatusPageService(client).WithNowFn(func() time.Time { return fixedNow })
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

	svc := NewStatusPageService(client).WithNowFn(func() time.Time { return fixedNow })
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
	_, _ = seedStatusFixture(t, client, model)

	svc := NewStatusPageService(client).WithNowFn(func() time.Time { return fixedNow })
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

func TestStatusPage_ChannelNameMasked(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-opus-4-7"

	// Account deliberately has a leaky name + email-in-notes. The schema
	// doesn't carry email as a first-class field, but notes frequently hold
	// "ops@example.com" and ops.IP — masking must ignore all of that.
	acc, err := client.Account.Create().
		SetName("ops@example.com").
		SetPlatform("anthropic").
		SetType("oauth").
		SetCredentials(map[string]interface{}{"api_key": "sk-secret"}).
		SetExtra(map[string]interface{}{"region": "HK"}).
		SetConcurrency(1).
		SetPriority(50).
		SetRateMultiplier(1.0).
		SetStatus("active").
		SetAutoPauseOnExpired(false).
		SetSchedulable(true).
		Save(context.Background())
	require.NoError(t, err)
	gid := seedGroup(t, client, "status-masked-group", map[string][]int64{model: {acc.ID}})
	seedAccountGroup(t, client, acc.ID, gid)

	svc := NewStatusPageService(client).WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Len(t, detail.Groups, 1)
	require.Len(t, detail.Groups[0].Channels, 1)
	got := detail.Groups[0].Channels[0].Name
	require.Equal(t, "HK", got)
	require.NotContains(t, got, "@")
	require.NotContains(t, got, "ops@example.com")
}

func TestStatusPage_ChannelNameFallbackToID(t *testing.T) {
	client := newChannelHealthTestClient(t)
	model := "claude-opus-4-7"
	aid, _ := seedStatusFixture(t, client, model)

	svc := NewStatusPageService(client).WithNowFn(func() time.Time { return fixedNow })
	detail, err := svc.GetModelDetail(context.Background(), model)
	require.NoError(t, err)
	require.Len(t, detail.Groups, 1)
	require.Len(t, detail.Groups[0].Channels, 1)
	require.Contains(t, detail.Groups[0].Channels[0].Name, "Channel #")
	require.Contains(t, detail.Groups[0].Channels[0].Name, itoa64(aid))
}

func TestStatusPage_ListModels(t *testing.T) {
	client := newChannelHealthTestClient(t)
	// Two groups routing three total models — one duplicate across groups.
	_ = seedGroup(t, client, "g-a", map[string][]int64{
		"claude-opus-4-7":  nil,
		"claude-opus-4-6":  nil,
		"claude-*-wildcard": nil, // must be filtered out
	})
	_ = seedGroup(t, client, "g-b", map[string][]int64{
		"claude-opus-4-7":    nil,
		"claude-sonnet-4-6":  nil,
	})

	svc := NewStatusPageService(client).WithNowFn(func() time.Time { return fixedNow })
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
