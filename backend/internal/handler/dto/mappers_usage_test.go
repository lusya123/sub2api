package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogFromService_IncludesOpenAIWSMode(t *testing.T) {
	t.Parallel()

	wsLog := &service.UsageLog{
		RequestID:    "req_1",
		Model:        "gpt-5.3-codex",
		OpenAIWSMode: true,
	}
	httpLog := &service.UsageLog{
		RequestID:    "resp_1",
		Model:        "gpt-5.3-codex",
		OpenAIWSMode: false,
	}

	require.True(t, UsageLogFromService(wsLog).OpenAIWSMode)
	require.False(t, UsageLogFromService(httpLog).OpenAIWSMode)
	require.True(t, UsageLogFromServiceAdmin(wsLog).OpenAIWSMode)
	require.False(t, UsageLogFromServiceAdmin(httpLog).OpenAIWSMode)
}

func TestUsageLogFromService_PrefersRequestTypeForLegacyFields(t *testing.T) {
	t.Parallel()

	log := &service.UsageLog{
		RequestID:    "req_2",
		Model:        "gpt-5.3-codex",
		RequestType:  service.RequestTypeWSV2,
		Stream:       false,
		OpenAIWSMode: false,
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	require.Equal(t, "ws_v2", userDTO.RequestType)
	require.True(t, userDTO.Stream)
	require.True(t, userDTO.OpenAIWSMode)
	require.Equal(t, "ws_v2", adminDTO.RequestType)
	require.True(t, adminDTO.Stream)
	require.True(t, adminDTO.OpenAIWSMode)
}

func TestUsageCleanupTaskFromService_RequestTypeMapping(t *testing.T) {
	t.Parallel()

	requestType := int16(service.RequestTypeStream)
	task := &service.UsageCleanupTask{
		ID:     1,
		Status: service.UsageCleanupStatusPending,
		Filters: service.UsageCleanupFilters{
			RequestType: &requestType,
		},
	}

	dtoTask := UsageCleanupTaskFromService(task)
	require.NotNil(t, dtoTask)
	require.NotNil(t, dtoTask.Filters.RequestType)
	require.Equal(t, "stream", *dtoTask.Filters.RequestType)
}

func TestRequestTypeStringPtrNil(t *testing.T) {
	t.Parallel()
	require.Nil(t, requestTypeStringPtr(nil))
}

func TestUsageLogFromService_IncludesServiceTierForUserAndAdmin(t *testing.T) {
	t.Parallel()

	serviceTier := "priority"
	inboundEndpoint := "/v1/chat/completions"
	upstreamEndpoint := "/v1/responses"
	log := &service.UsageLog{
		RequestID:             "req_3",
		Model:                 "gpt-5.4",
		ServiceTier:           &serviceTier,
		InboundEndpoint:       &inboundEndpoint,
		UpstreamEndpoint:      &upstreamEndpoint,
		AccountRateMultiplier: f64Ptr(1.5),
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	require.NotNil(t, userDTO.ServiceTier)
	require.Equal(t, serviceTier, *userDTO.ServiceTier)
	require.NotNil(t, userDTO.InboundEndpoint)
	require.Equal(t, inboundEndpoint, *userDTO.InboundEndpoint)
	require.NotNil(t, userDTO.UpstreamEndpoint)
	require.Equal(t, upstreamEndpoint, *userDTO.UpstreamEndpoint)
	require.NotNil(t, adminDTO.ServiceTier)
	require.Equal(t, serviceTier, *adminDTO.ServiceTier)
	require.NotNil(t, adminDTO.InboundEndpoint)
	require.Equal(t, inboundEndpoint, *adminDTO.InboundEndpoint)
	require.NotNil(t, adminDTO.UpstreamEndpoint)
	require.Equal(t, upstreamEndpoint, *adminDTO.UpstreamEndpoint)
	require.NotNil(t, adminDTO.AccountRateMultiplier)
	require.InDelta(t, 1.5, *adminDTO.AccountRateMultiplier, 1e-12)
}

func TestUsageLogFromService_UsesRequestedModelAndKeepsUpstreamAdminOnly(t *testing.T) {
	t.Parallel()

	upstreamModel := "claude-sonnet-4-20250514"
	log := &service.UsageLog{
		RequestID:      "req_4",
		Model:          upstreamModel,
		RequestedModel: "claude-sonnet-4",
		UpstreamModel:  &upstreamModel,
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	require.Equal(t, "claude-sonnet-4", userDTO.Model)
	require.Equal(t, "claude-sonnet-4", adminDTO.Model)

	userJSON, err := json.Marshal(userDTO)
	require.NoError(t, err)
	require.NotContains(t, string(userJSON), "upstream_model")

	adminJSON, err := json.Marshal(adminDTO)
	require.NoError(t, err)
	require.Contains(t, string(adminJSON), `"upstream_model":"claude-sonnet-4-20250514"`)
}

func TestUsageLogFromService_HidesBillingInternalsFromUserJSON(t *testing.T) {
	t.Parallel()

	log := &service.UsageLog{
		RequestID:         "req_hidden",
		Model:             "gpt-5.4",
		InputCost:         0.1,
		OutputCost:        0.2,
		CacheCreationCost: 0.3,
		CacheReadCost:     0.4,
		TotalCost:         0.5,
		ActualCost:        0.8,
		RateMultiplier:    1.6,
		ShowCostBreakdown: boolPtr(false),
	}

	userJSON, err := json.Marshal(UsageLogFromService(log))
	require.NoError(t, err)
	require.Contains(t, string(userJSON), `"actual_cost":0.8`)
	require.NotContains(t, string(userJSON), "input_cost")
	require.NotContains(t, string(userJSON), "output_cost")
	require.NotContains(t, string(userJSON), "cache_creation_cost")
	require.NotContains(t, string(userJSON), "cache_read_cost")
	require.NotContains(t, string(userJSON), "total_cost")
	require.NotContains(t, string(userJSON), "rate_multiplier")

	adminJSON, err := json.Marshal(UsageLogFromServiceAdmin(log))
	require.NoError(t, err)
	require.Contains(t, string(adminJSON), `"input_cost":0.1`)
	require.Contains(t, string(adminJSON), `"output_cost":0.2`)
	require.Contains(t, string(adminJSON), `"cache_creation_cost":0.3`)
	require.Contains(t, string(adminJSON), `"cache_read_cost":0.4`)
	require.Contains(t, string(adminJSON), `"total_cost":0.5`)
	require.Contains(t, string(adminJSON), `"rate_multiplier":1.6`)
}

func TestUsageLogFromService_ExposesCostBreakdownWhenGroupAllows(t *testing.T) {
	t.Parallel()

	log := &service.UsageLog{
		RequestID:         "req_cost_breakdown",
		Model:             "gpt-5.4",
		InputTokens:       1000,
		OutputTokens:      500,
		InputCost:         0.01,
		OutputCost:        0.02,
		CacheCreationCost: 0.03,
		CacheReadCost:     0.04,
		ActualCost:        0.2,
		ShowCostBreakdown: boolPtr(true),
	}

	userJSON, err := json.Marshal(UsageLogFromService(log))
	require.NoError(t, err)
	require.Contains(t, string(userJSON), `"input_cost":0.01`)
	require.Contains(t, string(userJSON), `"output_cost":0.02`)
	require.Contains(t, string(userJSON), `"cache_creation_cost":0.03`)
	require.Contains(t, string(userJSON), `"cache_read_cost":0.04`)
	require.NotContains(t, string(userJSON), "total_cost")
	require.NotContains(t, string(userJSON), `"actual_rate_multiplier"`)
	require.Contains(t, string(userJSON), `"show_cost_breakdown":true`)
}

func TestUsageLogFromService_UsesSnapshotOverCurrentGroupSetting(t *testing.T) {
	t.Parallel()

	log := &service.UsageLog{
		RequestID:         "req_snapshot_breakdown",
		Model:             "gpt-5.4",
		InputCost:         0.01,
		OutputCost:        0.02,
		ShowCostBreakdown: boolPtr(false),
		Group: &service.Group{
			ShowCostBreakdown: true,
		},
	}

	userJSON, err := json.Marshal(UsageLogFromService(log))
	require.NoError(t, err)
	require.Contains(t, string(userJSON), `"show_cost_breakdown":false`)
	require.NotContains(t, string(userJSON), "input_cost")
	require.NotContains(t, string(userJSON), "output_cost")
}

func TestUsageLogFromService_FallsBackToLegacyModelWhenRequestedModelMissing(t *testing.T) {
	t.Parallel()

	log := &service.UsageLog{
		RequestID: "req_legacy",
		Model:     "claude-3",
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	require.Equal(t, "claude-3", userDTO.Model)
	require.Equal(t, "claude-3", adminDTO.Model)
}

func f64Ptr(value float64) *float64 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
