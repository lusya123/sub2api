//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisplayNameForModel(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{id: "claude-opus-4-7", want: "Claude Opus 4.7"},
		{id: "claude-opus-4-7-thinking", want: "Claude Opus 4.7 Thinking"},
		{id: "claude-sonnet-4-6", want: "Claude Sonnet 4.6"},
		{id: "claude-haiku-4-5", want: "Claude Haiku 4.5"},
		{id: "claude-sonnet-4-5-20250929", want: "Claude Sonnet 4.5 (2025-09-29)"},
		{id: "claude-opus-4-5-20251101", want: "Claude Opus 4.5 (2025-11-01)"},
		{id: "claude-3-5-sonnet-20241022", want: "Claude 3.5 Sonnet (2024-10-22)"},
		{id: "claude-3-opus-20240229", want: "Claude 3 Opus (2024-02-29)"},
		{id: "deepseek-v4-pro", want: "DeepSeek V4 Pro"},
		{id: "minimax-m3", want: "MiniMax M3"},
		{id: "gpt-5.4-mini", want: "GPT 5.4 Mini"},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			require.Equal(t, tc.want, displayNameForModel(tc.id))
		})
	}
}

func TestCanUserBindGroupInternal(t *testing.T) {
	svc := &APIKeyService{}
	user := &User{AllowedGroups: []int64{2}}
	subscribed := map[int64]bool{3: true}

	cases := []struct {
		group Group
		name  string
		want  bool
	}{
		{
			group: Group{ID: 1, IsExclusive: false, SubscriptionType: SubscriptionTypeStandard},
			name:  "public standard group",
			want:  true,
		},
		{
			group: Group{ID: 2, IsExclusive: true, SubscriptionType: SubscriptionTypeStandard},
			name:  "allowed exclusive group",
			want:  true,
		},
		{
			group: Group{ID: 4, IsExclusive: true, SubscriptionType: SubscriptionTypeStandard},
			name:  "unapproved exclusive group",
			want:  false,
		},
		{
			group: Group{ID: 3, SubscriptionType: SubscriptionTypeSubscription},
			name:  "subscribed group",
			want:  true,
		},
		{
			group: Group{ID: 5, SubscriptionType: SubscriptionTypeSubscription},
			name:  "unsubscribed group",
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, svc.canUserBindGroupInternal(user, &tc.group, subscribed))
		})
	}
}
