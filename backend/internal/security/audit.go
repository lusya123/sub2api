package security

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
)

type LoginAuditor struct {
	db      *sql.DB
	enabled bool
	ch      chan loginAuditEvent
}

type loginAuditEvent struct {
	CreatedAt     time.Time
	Email         string
	IP            string
	XForwardedFor string
	UserAgent     string
	Fingerprint   string
	BodyHash      string
	Result        string
	DurationMS    int
}

func NewLoginAuditorFromEnt(entClient *dbent.Client, cfg config.LoginProtectionConfig) *LoginAuditor {
	cfg = withLoginProtectionDefaults(cfg)
	if !cfg.Enabled || !cfg.AuditEnabled {
		return &LoginAuditor{}
	}
	db, ok := SQLDBFromEnt(entClient)
	if !ok {
		slog.Warn("login auditor disabled: ent client does not expose sql db")
		return &LoginAuditor{}
	}
	a := &LoginAuditor{
		db:      db,
		enabled: true,
		ch:      make(chan loginAuditEvent, cfg.AuditQueueSize),
	}
	go a.consume()
	return a
}

func (a *LoginAuditor) AsyncLog(email, ip, xForwardedFor, userAgent, bodyHash, result string, duration time.Duration) {
	if a == nil || !a.enabled || os.Getenv("DEFENSE_LOGIN_AUDIT_ENABLED") == "false" {
		return
	}
	event := loginAuditEvent{
		CreatedAt:     time.Now().UTC(),
		Email:         truncateString(normalizeEmail(email), 255),
		IP:            truncateString(strings.TrimSpace(ip), 64),
		XForwardedFor: strings.TrimSpace(xForwardedFor),
		UserAgent:     userAgent,
		Fingerprint:   auditFingerprint(ip, xForwardedFor, userAgent),
		BodyHash:      truncateString(strings.TrimSpace(bodyHash), 16),
		Result:        truncateString(result, 32),
		DurationMS:    int(duration.Milliseconds()),
	}
	select {
	case a.ch <- event:
	default:
	}
}

func (a *LoginAuditor) consume() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	batch := make([]loginAuditEvent, 0, 100)
	for {
		select {
		case event := <-a.ch:
			batch = append(batch, event)
			if len(batch) >= 100 {
				a.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				a.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (a *LoginAuditor) flush(batch []loginAuditEvent) {
	if len(batch) == 0 || a == nil || a.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var b strings.Builder
	args := make([]any, 0, len(batch)*9)
	_, _ = b.WriteString("INSERT INTO login_attempts (created_at, email, ip, x_forwarded_for, user_agent, fingerprint, body_hash, result, duration_ms) VALUES ")
	for i, event := range batch {
		if i > 0 {
			_, _ = b.WriteString(",")
		}
		base := i*9 + 1
		_, _ = b.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)", base, base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8))
		args = append(args, event.CreatedAt, event.Email, event.IP, event.XForwardedFor, event.UserAgent, event.Fingerprint, event.BodyHash, event.Result, event.DurationMS)
	}
	if _, err := a.db.ExecContext(ctx, b.String(), args...); err != nil {
		slog.Warn("login audit flush failed", "error", err, "events", len(batch))
	}
}

func auditFingerprint(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func truncateString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
