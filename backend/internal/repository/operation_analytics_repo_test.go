package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNewOperationAnalyticsSnapshot_EmptyCollectionsEncodeAsArrays(t *testing.T) {
	snapshot := newOperationAnalyticsSnapshot(service.OperationAnalyticsFilter{
		StartTime:   time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
		Timezone:    "Asia/Shanghai",
	}, 12)

	payload, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(payload), ":null", "empty analytics collections must not leak JSON null values")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assertJSONArray(t, decoded, "trend")
	assertJSONArray(t, decoded, "funnel")
	assertJSONArray(t, decoded, "funnel_previous")
	assertJSONArray(t, decoded, "cohorts")
	assertJSONArray(t, decoded, "advice")
	assertNestedJSONArray(t, decoded, "lists", "high_spending", "silent_high_value", "benefit_idle", "expiring_soon", "new_inactive")
	assertNestedJSONArray(t, decoded, "distribution", "groups", "models", "api_keys", "promos", "redeem_types")
	assertNestedJSONArray(t, decoded, "churn", "history")
	assertNestedJSONArray(t, decoded, "pyramid", "levels")
	assertNestedJSONArray(t, decoded, "financial", "arpu_history")
	assertNestedJSONArray(t, decoded, "product_matrix", "plans", "models")
}

func assertJSONArray(t *testing.T, payload map[string]any, key string) {
	t.Helper()
	value, ok := payload[key]
	require.True(t, ok, "missing JSON field %q", key)
	require.IsType(t, []any{}, value, "JSON field %q must be an array", key)
}

func assertNestedJSONArray(t *testing.T, payload map[string]any, parent string, keys ...string) {
	t.Helper()
	nested, ok := payload[parent].(map[string]any)
	require.True(t, ok, "JSON field %q must be an object", parent)
	for _, key := range keys {
		assertJSONArray(t, nested, key)
	}
}
