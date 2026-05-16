package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const shopPasswordLoginSyncMinInterval = 6 * time.Hour

type shopPasswordLoginSyncRecord struct {
	passwordHash string
	syncedAt     time.Time
}

var shopPasswordLoginSyncState sync.Map

func (s *AuthService) syncShopPasswordReset(ctx context.Context, userID int64, email, newPassword string) error {
	if s == nil || s.cfg == nil {
		return nil
	}
	cfg := s.cfg.ShopAccountSync
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	secret := strings.TrimSpace(cfg.SharedSecret)
	if baseURL == "" || secret == "" {
		return nil
	}

	payload := map[string]any{
		"email":           strings.TrimSpace(email),
		"sub2api_user_id": userID,
		"new_password":    newPassword,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal shop password sync request: %w", err)
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/api/v1/integrations/sub2api/password-reset", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build shop password sync request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sub2API-Sync-Key", secret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("call shop password sync: %w", err)
	}
	defer resp.Body.Close()

	var parsed struct {
		StatusCode *int   `json:"status_code"`
		Code       *int   `json:"code"`
		Msg        string `json:"msg"`
		Message    string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("decode shop password sync response: %w", err)
	}
	businessOK := (parsed.StatusCode != nil && *parsed.StatusCode == 0) || (parsed.Code != nil && *parsed.Code == 0)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !businessOK {
		message := strings.TrimSpace(parsed.Msg)
		if message == "" {
			message = strings.TrimSpace(parsed.Message)
		}
		return fmt.Errorf("shop password sync failed: status=%d message=%s", resp.StatusCode, message)
	}
	return nil
}

// ScheduleShopPasswordLoginSync keeps Sub2API's native login flow local and fast,
// while preparing the matching Shop account in the background for unified access.
func (s *AuthService) ScheduleShopPasswordLoginSync(ctx context.Context, user *User, password string, source string) {
	if s == nil || s.cfg == nil || user == nil || user.ID <= 0 {
		return
	}
	if password == "" || !s.shopAccountSyncEnabled() {
		return
	}

	passwordHash := strings.TrimSpace(user.PasswordHash)
	if passwordHash == "" {
		return
	}
	if shouldSkipShopPasswordLoginSync(user.ID, passwordHash, time.Now()) {
		return
	}

	userID := user.ID
	email := strings.TrimSpace(user.Email)
	go func() {
		if err := s.syncShopPasswordReset(context.Background(), userID, email, password); err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Shop password login sync failed for user %d source=%s: %v", userID, source, err)
			return
		}
		shopPasswordLoginSyncState.Store(userID, shopPasswordLoginSyncRecord{
			passwordHash: passwordHash,
			syncedAt:     time.Now(),
		})
	}()
}

func (s *AuthService) shopAccountSyncEnabled() bool {
	if s == nil || s.cfg == nil {
		return false
	}
	cfg := s.cfg.ShopAccountSync
	return strings.TrimSpace(cfg.BaseURL) != "" && strings.TrimSpace(cfg.SharedSecret) != ""
}

func shouldSkipShopPasswordLoginSync(userID int64, passwordHash string, now time.Time) bool {
	if userID <= 0 || strings.TrimSpace(passwordHash) == "" {
		return true
	}
	value, ok := shopPasswordLoginSyncState.Load(userID)
	if !ok {
		return false
	}
	record, ok := value.(shopPasswordLoginSyncRecord)
	if !ok {
		shopPasswordLoginSyncState.Delete(userID)
		return false
	}
	if record.passwordHash != passwordHash {
		return false
	}
	if record.syncedAt.IsZero() || now.Sub(record.syncedAt) >= shopPasswordLoginSyncMinInterval {
		return false
	}
	return true
}
