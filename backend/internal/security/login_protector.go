package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/redis/go-redis/v9"
)

var (
	ErrLoginLocked      = infraerrors.TooManyRequests("LOGIN_LOCKED", "Too many failed login attempts. Please try again later.")
	ErrLoginRateLimited = infraerrors.TooManyRequests("LOGIN_RATE_LIMITED", "Too many login attempts. Please wait a moment and try again.")
	ErrLoginServiceBusy = infraerrors.ServiceUnavailable("LOGIN_SERVICE_BUSY", "The login service is busy. Please try again shortly.")
)

type LoginProtector struct {
	rdb *redis.Client
	cfg config.LoginProtectionConfig
}

func NewLoginProtector(rdb *redis.Client, cfg config.LoginProtectionConfig) *LoginProtector {
	return &LoginProtector{rdb: rdb, cfg: withLoginProtectionDefaults(cfg)}
}

func (p *LoginProtector) Enabled() bool {
	return p != nil && p.cfg.Enabled
}

func (p *LoginProtector) Precheck(ctx context.Context, email string) error {
	if !p.Enabled() || !p.cfg.RedisEnabled || p.rdb == nil {
		return nil
	}

	if err := p.PrecheckGlobal(ctx); err != nil {
		return err
	}
	return p.PrecheckEmail(ctx, email)
}

func (p *LoginProtector) PrecheckGlobal(ctx context.Context) error {
	if !p.Enabled() || !p.cfg.RedisEnabled || p.rdb == nil {
		return nil
	}
	globalLimit := p.cfg.GlobalRatePerSecond
	if globalLimit > 0 {
		globalKey := p.cfg.KeyPrefix + "global:" + time.Now().UTC().Format("20060102150405")
		n, err := p.rdb.Incr(ctx, globalKey).Result()
		if err != nil {
			slog.Warn("login protector global counter failed open", "error", err)
		} else {
			_ = p.rdb.Expire(ctx, globalKey, 5*time.Second).Err()
			if n > int64(globalLimit) {
				return loginServiceBusyError(5 * time.Second)
			}
		}
	}
	return nil
}

func (p *LoginProtector) PrecheckEmail(ctx context.Context, email string) error {
	if !p.Enabled() || !p.cfg.RedisEnabled || p.rdb == nil {
		return nil
	}
	emailKey := loginEmailKey(email)
	if emailKey == "" {
		return nil
	}

	lockKey := p.cfg.KeyPrefix + "lock:" + emailKey
	locked, err := p.rdb.Exists(ctx, lockKey).Result()
	if err != nil {
		slog.Warn("login protector lock check failed open", "error", err)
	} else if locked > 0 {
		return loginLockedError(p.ttlOrDefault(ctx, lockKey, time.Duration(p.cfg.LockShortMinutes)*time.Minute))
	}

	rateLimit := p.cfg.EmailRatePerMinute
	if rateLimit <= 0 {
		return nil
	}
	rateKey := p.cfg.KeyPrefix + "rate:" + emailKey
	n, err := p.rdb.Incr(ctx, rateKey).Result()
	if err != nil {
		slog.Warn("login protector email rate failed open", "error", err)
		return nil
	}
	if n == 1 {
		_ = p.rdb.Expire(ctx, rateKey, time.Minute).Err()
	}
	if n > int64(rateLimit) {
		return loginRateLimitedError(p.ttlOrDefault(ctx, rateKey, time.Minute))
	}
	return nil
}

func (p *LoginProtector) RecordFailure(ctx context.Context, email string) {
	if !p.Enabled() || !p.cfg.RedisEnabled || p.rdb == nil {
		return
	}
	emailKey := loginEmailKey(email)
	if emailKey == "" {
		return
	}

	failKey := p.cfg.KeyPrefix + "fail:" + emailKey
	n, err := p.rdb.Incr(ctx, failKey).Result()
	if err != nil {
		slog.Warn("login protector failure counter skipped", "error", err)
		return
	}
	if n == 1 {
		_ = p.rdb.Expire(ctx, failKey, time.Duration(p.cfg.FailureWindowMinutes)*time.Minute).Err()
	}

	lockKey := p.cfg.KeyPrefix + "lock:" + emailKey
	switch {
	case n > int64(p.cfg.LockLongAfter):
		_ = p.rdb.Set(ctx, lockKey, "1", time.Duration(p.cfg.LockLongMinutes)*time.Minute).Err()
	case n > int64(p.cfg.LockShortAfter):
		_ = p.rdb.Set(ctx, lockKey, "1", time.Duration(p.cfg.LockShortMinutes)*time.Minute).Err()
	}
}

func (p *LoginProtector) RecordSuccess(ctx context.Context, email string) {
	if !p.Enabled() || !p.cfg.RedisEnabled || p.rdb == nil {
		return
	}
	emailKey := loginEmailKey(email)
	if emailKey == "" {
		return
	}
	if err := p.rdb.Del(
		ctx,
		p.cfg.KeyPrefix+"fail:"+emailKey,
		p.cfg.KeyPrefix+"lock:"+emailKey,
		p.cfg.KeyPrefix+"rate:"+emailKey,
	).Err(); err != nil && !errors.Is(err, redis.Nil) {
		slog.Warn("login protector success cleanup skipped", "error", err)
	}
}

func loginEmailKey(email string) string {
	email = normalizeEmail(email)
	if email == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:])
}

func withLoginProtectionDefaults(cfg config.LoginProtectionConfig) config.LoginProtectionConfig {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "login:"
	}
	if cfg.EmailRatePerMinute <= 0 {
		cfg.EmailRatePerMinute = 10
	}
	if cfg.GlobalRatePerSecond <= 0 {
		cfg.GlobalRatePerSecond = 200
	}
	if cfg.FailureWindowMinutes <= 0 {
		cfg.FailureWindowMinutes = 60
	}
	if cfg.LockShortAfter <= 0 {
		cfg.LockShortAfter = 7
	}
	if cfg.LockLongAfter <= 0 {
		cfg.LockLongAfter = 15
	}
	if cfg.LockShortMinutes <= 0 {
		cfg.LockShortMinutes = 15
	}
	if cfg.LockLongMinutes <= 0 {
		cfg.LockLongMinutes = 60
	}
	if cfg.BloomCapacity <= 0 {
		cfg.BloomCapacity = 1_000_000
	}
	if cfg.AuditQueueSize <= 0 {
		cfg.AuditQueueSize = 10_000
	}
	return cfg
}

func (p *LoginProtector) ttlOrDefault(ctx context.Context, key string, fallback time.Duration) time.Duration {
	if p == nil || p.rdb == nil {
		return fallback
	}
	ttl, err := p.rdb.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		return fallback
	}
	return ttl
}

func loginLockedError(retryAfter time.Duration) *infraerrors.ApplicationError {
	return infraerrors.TooManyRequests(
		"LOGIN_LOCKED",
		fmt.Sprintf("Too many failed login attempts. Please try again in %s.", humanRetryAfter(retryAfter)),
	).WithMetadata(retryAfterMetadata(retryAfter))
}

func loginRateLimitedError(retryAfter time.Duration) *infraerrors.ApplicationError {
	return infraerrors.TooManyRequests(
		"LOGIN_RATE_LIMITED",
		fmt.Sprintf("Too many login attempts. Please try again in %s.", humanRetryAfter(retryAfter)),
	).WithMetadata(retryAfterMetadata(retryAfter))
}

func loginServiceBusyError(retryAfter time.Duration) *infraerrors.ApplicationError {
	return infraerrors.ServiceUnavailable(
		"LOGIN_SERVICE_BUSY",
		fmt.Sprintf("The login service is busy. Please try again in %s.", humanRetryAfter(retryAfter)),
	).WithMetadata(retryAfterMetadata(retryAfter))
}

func retryAfterMetadata(retryAfter time.Duration) map[string]string {
	if retryAfter <= 0 {
		retryAfter = time.Second
	}
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	minutes := int(math.Ceil(float64(seconds) / 60))
	if minutes < 1 {
		minutes = 1
	}
	return map[string]string{
		"retry_after_seconds": strconv.Itoa(seconds),
		"retry_after_minutes": strconv.Itoa(minutes),
	}
}

func humanRetryAfter(retryAfter time.Duration) string {
	metadata := retryAfterMetadata(retryAfter)
	seconds, _ := strconv.Atoi(metadata["retry_after_seconds"])
	if seconds < 60 {
		return metadata["retry_after_seconds"] + " seconds"
	}
	return metadata["retry_after_minutes"] + " minutes"
}
