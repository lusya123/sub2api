package security

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type DynamicBan struct {
	rdb *redis.Client
}

func NewDynamicBan(rdb *redis.Client) *DynamicBan {
	return &DynamicBan{rdb: rdb}
}

func (d *DynamicBan) Trigger(ctx context.Context, fingerprint, reason string) {
	// Dynamic IP/client-fingerprint bans are intentionally disabled for CDN
	// deployments. Active defense now uses signed visitor cookies and credit
	// scores so a CDN node address can never be blocked.
}

func (d *DynamicBan) IsBanned(ctx context.Context, fingerprint string) bool {
	return false
}
