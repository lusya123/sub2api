//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGetStepUpEnabledDistinguishesMissingFromStorageFailure(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		repoErr     error
		wantEnabled bool
		wantErr     bool
	}{
		{name: "enabled", value: "true", wantEnabled: true},
		{name: "explicitly disabled", value: "false", wantEnabled: false},
		{name: "missing defaults disabled", repoErr: ErrSettingNotFound, wantEnabled: false},
		{name: "storage failure is surfaced", repoErr: errors.New("database unavailable"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &bmRepoStub{getValueFn: func(_ context.Context, key string) (string, error) {
				require.Equal(t, SettingKeyStepUpEnabled, key)
				return tt.value, tt.repoErr
			}}
			svc := NewSettingService(repo, &config.Config{})

			enabled, err := svc.GetStepUpEnabled(context.Background())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantEnabled, enabled)
		})
	}
}
