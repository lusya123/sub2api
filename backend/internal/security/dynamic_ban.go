package security

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type DynamicBan struct {
	rdb *redis.Client
}

func NewDynamicBan(rdb *redis.Client) *DynamicBan {
	return &DynamicBan{rdb: rdb}
}

func (d *DynamicBan) Trigger(ctx context.Context, fingerprint, reason string) {
	if os.Getenv("DEFENSE_DYNAMIC_BAN_ENABLED") == "false" || d == nil || d.rdb == nil || fingerprint == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	vkey := "violations:" + fingerprint
	n, err := d.rdb.Incr(ctx, vkey).Result()
	if err != nil {
		return
	}
	_ = d.rdb.Expire(ctx, vkey, 24*time.Hour).Err()

	var duration time.Duration
	switch {
	case n >= 4:
		duration = 365 * 24 * time.Hour
	case n >= 3:
		duration = 24 * time.Hour
	case n >= 2:
		duration = time.Hour
	default:
		duration = 5 * time.Minute
	}
	_ = d.rdb.Set(ctx, "ban:"+fingerprint, reason, duration).Err()
}

func (d *DynamicBan) IsBanned(ctx context.Context, fingerprint string) bool {
	if os.Getenv("DEFENSE_DYNAMIC_BAN_ENABLED") == "false" || d == nil || d.rdb == nil || fingerprint == "" {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	n, _ := d.rdb.Exists(ctx, "ban:"+fingerprint).Result()
	return n > 0
}
