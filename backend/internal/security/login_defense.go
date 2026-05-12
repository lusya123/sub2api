package security

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
)

const maxServiceBusyAuditPerSecond = 20

const (
	LoginResultUserNotFound       = "user_not_found"
	LoginResultServiceBusy        = "service_busy"
	LoginResultLocked             = "locked"
	LoginResultRateLimited        = "rate_limited"
	LoginResultBlocked            = "blocked"
	LoginResultTurnstileFailed    = "turnstile_failed"
	LoginResultWrongPassword      = "wrong_password"
	LoginResultError              = "error"
	LoginResultBackendModeBlocked = "backend_mode_blocked"
	LoginResultPasswordOK2FA      = "password_ok_2fa_required"
	LoginResultSuccess            = "success"
)

type LoginAttempt struct {
	Email         string
	IP            string
	XForwardedFor string
	UserAgent     string
	Started       time.Time
}

func NewLoginAttempt(email, ip, xForwardedFor, userAgent string) LoginAttempt {
	return LoginAttempt{
		Email:         NormalizeEmail(email),
		IP:            ip,
		XForwardedFor: xForwardedFor,
		UserAgent:     userAgent,
		Started:       time.Now(),
	}
}

type LoginDefense struct {
	cfg             config.LoginProtectionConfig
	protector       *LoginProtector
	auditor         *LoginAuditor
	busyAuditMu     sync.Mutex
	busyAuditSecond int64
	busyAuditCount  int
}

func NewLoginDefense(cfg config.LoginProtectionConfig, entClient *dbent.Client, redisClient *redis.Client) *LoginDefense {
	cfg = withLoginProtectionDefaults(cfg)
	d := &LoginDefense{cfg: cfg}
	if !cfg.Enabled {
		return d
	}

	d.protector = NewLoginProtector(redisClient, cfg)
	d.auditor = NewLoginAuditorFromEnt(entClient, cfg)

	if cfg.BloomEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := InitEmailBloom(ctx, entClient, cfg); err != nil {
			slog.Warn("login email bloom initialization failed open", "error", err)
		}
	}
	return d
}

func (d *LoginDefense) RejectUnknownEmail(ctx context.Context, email string) (bool, error) {
	if d == nil || !d.cfg.Enabled || !d.cfg.BloomEnabled {
		return false, nil
	}
	if BloomMaybeContains(email) {
		return false, nil
	}
	if d.protector != nil {
		if err := d.protector.PrecheckGlobal(ctx); err != nil {
			return false, err
		}
	}
	ConstantTimeReject()
	return true, nil
}

func (d *LoginDefense) Precheck(ctx context.Context, email string) error {
	if d == nil || d.protector == nil {
		return nil
	}
	return d.protector.PrecheckEmail(ctx, email)
}

func (d *LoginDefense) RecordFailure(ctx context.Context, email string) {
	if d == nil || d.protector == nil {
		return
	}
	d.protector.RecordFailure(ctx, email)
}

func (d *LoginDefense) RecordSuccess(ctx context.Context, email string) {
	if d == nil || d.protector == nil {
		return
	}
	d.protector.RecordSuccess(ctx, email)
}

func (d *LoginDefense) Audit(attempt LoginAttempt, result string) {
	if d == nil || d.auditor == nil {
		return
	}
	if result == LoginResultServiceBusy && !d.allowServiceBusyAudit() {
		return
	}
	d.auditor.AsyncLog(
		attempt.Email,
		attempt.IP,
		attempt.XForwardedFor,
		attempt.UserAgent,
		result,
		time.Since(attempt.Started),
	)
}

func (d *LoginDefense) allowServiceBusyAudit() bool {
	now := time.Now().Unix()
	d.busyAuditMu.Lock()
	defer d.busyAuditMu.Unlock()
	if d.busyAuditSecond != now {
		d.busyAuditSecond = now
		d.busyAuditCount = 0
	}
	if d.busyAuditCount >= maxServiceBusyAuditPerSecond {
		return false
	}
	d.busyAuditCount++
	return true
}

func (d *LoginDefense) AuditPrecheckFailure(attempt LoginAttempt, err error) {
	d.Audit(attempt, LoginResultForPrecheckError(err))
}

func LoginResultForPrecheckError(err error) string {
	switch {
	case errors.Is(err, ErrLoginServiceBusy):
		return LoginResultServiceBusy
	case errors.Is(err, ErrLoginLocked):
		return LoginResultLocked
	case errors.Is(err, ErrLoginRateLimited):
		return LoginResultRateLimited
	default:
		return LoginResultBlocked
	}
}

func AddLoginEmail(email string) {
	BloomAdd(email)
}
