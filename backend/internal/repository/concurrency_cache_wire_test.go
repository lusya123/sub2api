package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProvideConcurrencyCache_WaitTTLAtLeastUserWaitWindow(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			ConcurrencySlotTTLMinutes: 30,
			Scheduling: config.GatewaySchedulingConfig{
				StickySessionWaitTimeout: time.Second,
				FallbackWaitTimeout:      time.Second,
			},
		},
	}

	cache, ok := ProvideConcurrencyCache(nil, cfg).(*concurrencyCache)
	require.True(t, ok)
	require.Equal(t, service.DefaultUserConcurrencyWaitSeconds, cache.waitQueueTTLSeconds)
}
