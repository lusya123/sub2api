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
var shopPasswordLoginSyncBlockedHash sync.Map
var shopPasswordSyncLocks sync.Map

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
	defer func() { _ = resp.Body.Close() }()

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

// SyncShopPasswordChange applies a user-initiated Sub2API password change to the
// matching Shop account before Sub2API commits the local password update.
func (s *AuthService) SyncShopPasswordChange(ctx context.Context, userID int64, email, newPassword string) error {
	lock := shopPasswordSyncLockForUser(userID)
	lock.Lock()
	defer lock.Unlock()
	return s.syncShopPasswordReset(ctx, userID, email, newPassword)
}

func (s *AuthService) BlockShopPasswordLoginSyncHash(userID int64, passwordHash string) {
	if userID <= 0 || strings.TrimSpace(passwordHash) == "" {
		return
	}
	shopPasswordLoginSyncBlockedHash.Store(userID, passwordHash)
}

func (s *AuthService) UnblockShopPasswordLoginSyncHash(userID int64, passwordHash string) {
	if userID <= 0 || strings.TrimSpace(passwordHash) == "" {
		return
	}
	value, ok := shopPasswordLoginSyncBlockedHash.Load(userID)
	if !ok {
		return
	}
	blockedHash, ok := value.(string)
	if !ok || blockedHash == passwordHash {
		shopPasswordLoginSyncBlockedHash.Delete(userID)
	}
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
		lock := shopPasswordSyncLockForUser(userID)
		lock.Lock()
		defer lock.Unlock()
		if isShopPasswordLoginSyncHashBlocked(userID, passwordHash) {
			logger.LegacyPrintf("service.auth", "[Auth] Skip blocked Shop password login sync for user %d source=%s", userID, source)
			return
		}
		if !s.shopPasswordLoginSyncStillCurrent(context.Background(), userID, passwordHash) {
			logger.LegacyPrintf("service.auth", "[Auth] Skip stale Shop password login sync for user %d source=%s", userID, source)
			return
		}
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

func shopPasswordSyncLockForUser(userID int64) *sync.Mutex {
	value, _ := shopPasswordSyncLocks.LoadOrStore(userID, &sync.Mutex{})
	lock, ok := value.(*sync.Mutex)
	if !ok {
		shopPasswordSyncLocks.Delete(userID)
		return shopPasswordSyncLockForUser(userID)
	}
	return lock
}

func isShopPasswordLoginSyncHashBlocked(userID int64, passwordHash string) bool {
	value, ok := shopPasswordLoginSyncBlockedHash.Load(userID)
	if !ok {
		return false
	}
	blockedHash, ok := value.(string)
	if !ok {
		shopPasswordLoginSyncBlockedHash.Delete(userID)
		return false
	}
	return blockedHash == passwordHash
}

func (s *AuthService) shopPasswordLoginSyncStillCurrent(ctx context.Context, userID int64, passwordHash string) bool {
	if s == nil || s.userRepo == nil {
		return true
	}
	latest, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Skip Shop password login sync for user %d: reload failed: %v", userID, err)
		return false
	}
	return strings.TrimSpace(latest.PasswordHash) == passwordHash
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
