package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloneCredentialsDetachesNestedReferenceValues(t *testing.T) {
	t.Parallel()

	original := map[string]any{
		"header_overrides": map[string]any{
			"x-relay": []string{"first", "second"},
		},
		"rules": []any{
			map[string]any{"models": []any{"gpt-5", "gpt-5-mini"}},
		},
	}

	cloned := cloneCredentials(original)
	cloned["header_overrides"].(map[string]any)["x-relay"].([]string)[0] = "changed"
	cloned["rules"].([]any)[0].(map[string]any)["models"].([]any)[0] = "changed"

	require.Equal(t, "first", original["header_overrides"].(map[string]any)["x-relay"].([]string)[0])
	require.Equal(t, "gpt-5", original["rules"].([]any)[0].(map[string]any)["models"].([]any)[0])
}

func TestCloneCredentialsPreservesConcreteTypes(t *testing.T) {
	t.Parallel()

	original := map[string]any{
		"model_mapping": map[string]string{"alias": "target"},
		"limits":        []int64{1, 2},
	}

	cloned := cloneCredentials(original)

	require.IsType(t, map[string]string{}, cloned["model_mapping"])
	require.IsType(t, []int64{}, cloned["limits"])
}
