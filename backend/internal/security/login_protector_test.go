package security

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestLoginEmailKeyHashesEmail(t *testing.T) {
	got := loginEmailKey(" User@Example.COM ")
	if got == "" {
		t.Fatal("expected hashed key")
	}
	if strings.Contains(got, "user") || strings.Contains(got, "example.com") {
		t.Fatalf("email key leaks raw email: %q", got)
	}
	if got != loginEmailKey("user@example.com") {
		t.Fatal("email key should be stable after normalization")
	}
}

func TestLoginProtectionDefaults(t *testing.T) {
	cfg := withLoginProtectionDefaults(config.LoginProtectionConfig{})

	if cfg.KeyPrefix != "login:" {
		t.Fatalf("unexpected key prefix: %q", cfg.KeyPrefix)
	}
	if cfg.EmailRatePerMinute != 10 {
		t.Fatalf("unexpected email rate: %d", cfg.EmailRatePerMinute)
	}
	if cfg.GlobalRatePerSecond != 200 {
		t.Fatalf("unexpected global rate: %d", cfg.GlobalRatePerSecond)
	}
	if cfg.LockShortAfter != 7 || cfg.LockLongAfter != 15 {
		t.Fatalf("unexpected lock thresholds: short=%d long=%d", cfg.LockShortAfter, cfg.LockLongAfter)
	}
	if cfg.AuditQueueSize != 10000 {
		t.Fatalf("unexpected audit queue size: %d", cfg.AuditQueueSize)
	}
}

func TestLoginLockErrorIncludesRetryAfter(t *testing.T) {
	err := loginLockedError(15 * time.Minute)
	if !errors.Is(err, ErrLoginLocked) {
		t.Fatal("dynamic lock error should match ErrLoginLocked")
	}
	if err.Metadata["retry_after_seconds"] != "900" {
		t.Fatalf("unexpected retry_after_seconds: %q", err.Metadata["retry_after_seconds"])
	}
	if err.Metadata["retry_after_minutes"] != "15" {
		t.Fatalf("unexpected retry_after_minutes: %q", err.Metadata["retry_after_minutes"])
	}
	if !strings.Contains(err.Message, "15 minutes") {
		t.Fatalf("unexpected message: %q", err.Message)
	}
}

func TestLoginServiceBusyErrorIncludesRetryAfter(t *testing.T) {
	err := loginServiceBusyError(5 * time.Second)
	if !errors.Is(err, ErrLoginServiceBusy) {
		t.Fatal("dynamic service-busy error should match ErrLoginServiceBusy")
	}
	if err.Metadata["retry_after_seconds"] != "5" {
		t.Fatalf("unexpected retry_after_seconds: %q", err.Metadata["retry_after_seconds"])
	}
	if err.Metadata["retry_after_minutes"] != "1" {
		t.Fatalf("unexpected retry_after_minutes: %q", err.Metadata["retry_after_minutes"])
	}
}
