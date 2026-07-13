//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestScheduleShopPasswordLoginSync_SkipsStalePasswordHash(t *testing.T) {
	oldUser := &User{ID: 9101, Email: "stale-sync@example.com"}
	require.NoError(t, oldUser.SetPassword("old-password"))

	currentUser := &User{ID: oldUser.ID, Email: oldUser.Email}
	require.NoError(t, currentUser.SetPassword("new-password"))

	called := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 0})
	}))
	defer server.Close()

	svc := &AuthService{
		userRepo: &mockUserRepo{getByIDUser: currentUser},
		cfg: &config.Config{ShopAccountSync: config.ShopAccountSyncConfig{
			BaseURL:        server.URL,
			SharedSecret:   "secret",
			TimeoutSeconds: 2,
		}},
	}

	svc.ScheduleShopPasswordLoginSync(context.Background(), oldUser, "old-password", "unit")

	select {
	case <-called:
		t.Fatal("stale login sync should not call shop")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestScheduleShopPasswordLoginSync_SkipsBlockedPasswordHash(t *testing.T) {
	user := &User{ID: 9103, Email: "blocked-sync@example.com"}
	require.NoError(t, user.SetPassword("old-password"))

	called := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 0})
	}))
	defer server.Close()

	svc := &AuthService{
		userRepo: &mockUserRepo{getByIDUser: user},
		cfg: &config.Config{ShopAccountSync: config.ShopAccountSyncConfig{
			BaseURL:        server.URL,
			SharedSecret:   "secret",
			TimeoutSeconds: 2,
		}},
	}
	svc.BlockShopPasswordLoginSyncHash(user.ID, user.PasswordHash)

	svc.ScheduleShopPasswordLoginSync(context.Background(), user, "old-password", "unit")

	select {
	case <-called:
		t.Fatal("blocked login sync should not call shop")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestScheduleShopPasswordLoginSync_SyncsWhenPasswordHashStillCurrent(t *testing.T) {
	user := &User{ID: 9102, Email: "current-sync@example.com"}
	require.NoError(t, user.SetPassword("current-password"))

	called := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		called <- payload
		_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 0})
	}))
	defer server.Close()

	svc := &AuthService{
		userRepo: &mockUserRepo{getByIDUser: user},
		cfg: &config.Config{ShopAccountSync: config.ShopAccountSyncConfig{
			BaseURL:        server.URL,
			SharedSecret:   "secret",
			TimeoutSeconds: 2,
		}},
	}

	svc.ScheduleShopPasswordLoginSync(context.Background(), user, "current-password", "unit")

	select {
	case payload := <-called:
		require.Equal(t, "current-sync@example.com", payload["email"])
		require.Equal(t, "current-password", payload["new_password"])
	case <-time.After(2 * time.Second):
		t.Fatal("expected current login sync to call shop")
	}
}
