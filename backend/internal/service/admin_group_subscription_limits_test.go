//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_UpdateGroup_PreservesOmittedSubscriptionLimits(t *testing.T) {
	description := "Supports Claude Opus 5.5"
	models := GroupModelsListConfig{Enabled: true, Models: []string{"claude-opus-5-5"}}
	for _, test := range []struct {
		name  string
		input UpdateGroupInput
	}{
		{name: "description only", input: UpdateGroupInput{Description: &description}},
		{name: "models only", input: UpdateGroupInput{ModelsListConfig: &models}},
		{name: "empty update", input: UpdateGroupInput{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			daily, weekly, monthly := 15.0, 105.0, 450.0
			existing := &Group{ID: 4, Name: "15 USD daily subscription", Platform: PlatformAnthropic,
				Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription,
				DailyLimitUSD: &daily, WeeklyLimitUSD: &weekly, MonthlyLimitUSD: &monthly}
			repo := &groupRepoStubForAdmin{getByID: existing}
			svc := &adminServiceImpl{groupRepo: repo}

			updated, err := svc.UpdateGroup(context.Background(), existing.ID, &test.input)
			require.NoError(t, err)
			require.Equal(t, &daily, updated.DailyLimitUSD)
			require.Equal(t, &weekly, updated.WeeklyLimitUSD)
			require.Equal(t, &monthly, updated.MonthlyLimitUSD)
			if test.input.Description != nil {
				require.Equal(t, description, updated.Description)
			}
			if test.input.ModelsListConfig != nil {
				require.Equal(t, models, updated.ModelsListConfig)
			}
		})
	}
}

func TestAdminService_UpdateGroup_ChangesOnlySubmittedSubscriptionLimit(t *testing.T) {
	for _, field := range []string{"daily", "weekly", "monthly"} {
		for _, value := range []float64{30, 0, -1} {
			t.Run(field+"/"+map[float64]string{30: "replace", 0: "zero", -1: "clear"}[value], func(t *testing.T) {
				daily, weekly, monthly := 15.0, 105.0, 450.0
				existing := &Group{ID: 4, Name: "limited subscription", Platform: PlatformAnthropic,
					Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription,
					DailyLimitUSD: &daily, WeeklyLimitUSD: &weekly, MonthlyLimitUSD: &monthly}
				input := &UpdateGroupInput{}
				wantDaily, wantWeekly, wantMonthly := &daily, &weekly, &monthly
				var replacement *float64
				if value >= 0 {
					replacement = &value
				}
				switch field {
				case "daily":
					input.DailyLimitUSD, wantDaily = &value, replacement
				case "weekly":
					input.WeeklyLimitUSD, wantWeekly = &value, replacement
				case "monthly":
					input.MonthlyLimitUSD, wantMonthly = &value, replacement
				}
				svc := &adminServiceImpl{groupRepo: &groupRepoStubForAdmin{getByID: existing}}

				updated, err := svc.UpdateGroup(context.Background(), existing.ID, input)
				require.NoError(t, err)
				require.Equal(t, wantDaily, updated.DailyLimitUSD)
				require.Equal(t, wantWeekly, updated.WeeklyLimitUSD)
				require.Equal(t, wantMonthly, updated.MonthlyLimitUSD)
			})
		}
	}
}
