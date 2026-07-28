package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloneCredentialsDetachesNestedReferenceValues(t *testing.T) {
	t.Parallel()

	originalRelayHeaders := []string{"first", "second"}
	originalModels := []any{"gpt-5", "gpt-5-mini"}
	original := map[string]any{
		"header_overrides": map[string]any{
			"x-relay": originalRelayHeaders,
		},
		"rules": []any{
			map[string]any{"models": originalModels},
		},
	}

	cloned := cloneCredentials(original)
	clonedHeaderOverrides, ok := cloned["header_overrides"].(map[string]any)
	require.True(t, ok)
	clonedRelayHeaders, ok := clonedHeaderOverrides["x-relay"].([]string)
	require.True(t, ok)
	clonedRelayHeaders[0] = "changed"

	clonedRules, ok := cloned["rules"].([]any)
	require.True(t, ok)
	clonedFirstRule, ok := clonedRules[0].(map[string]any)
	require.True(t, ok)
	clonedModels, ok := clonedFirstRule["models"].([]any)
	require.True(t, ok)
	clonedModels[0] = "changed"

	require.Equal(t, "first", originalRelayHeaders[0])
	require.Equal(t, "gpt-5", originalModels[0])
}

func TestCloneCredentialsPreservesConcreteTypes(t *testing.T) {
	t.Parallel()

	type namedModelMapping map[string]string
	type namedLimits []int64

	original := map[string]any{
		"model_mapping": namedModelMapping{"alias": "target"},
		"limits":        namedLimits{1, 2},
	}

	cloned := cloneCredentials(original)

	require.IsType(t, namedModelMapping{}, cloned["model_mapping"])
	require.IsType(t, namedLimits{}, cloned["limits"])
}
