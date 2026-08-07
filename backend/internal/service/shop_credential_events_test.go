package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

const testShopCredentialSecret = "0123456789abcdef0123456789abcdef"

func TestShopCredentialEventClient_UsesFixedContractAndNoCredentialMaterial(t *testing.T) {
	fixedNow := time.Unix(1_800_000_000, 0).UTC()
	occurredAt := time.Date(2026, time.August, 2, 1, 2, 3, 0, time.UTC)
	var handlerErr atomic.Value
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, shopCredentialEventMaxBodyBytes+1))
		if err != nil {
			handlerErr.Store(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != shopCredentialEventPath {
			handlerErr.Store(errors.New("unexpected method or path"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		timestamp := r.Header.Get("X-Sub2API-Timestamp")
		mac := hmac.New(sha256.New, []byte(testShopCredentialSecret))
		_, _ = mac.Write([]byte(timestamp + "\n"))
		_, _ = mac.Write(body)
		if r.Header.Get("X-Sub2API-Signature") != hex.EncodeToString(mac.Sum(nil)) {
			handlerErr.Store(errors.New("signature mismatch"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if timestamp != "1800000000" {
			handlerErr.Store(errors.New("timestamp mismatch"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		lowerBody := strings.ToLower(string(body))
		if strings.Contains(lowerBody, "password") || strings.Contains(lowerBody, "hash") {
			handlerErr.Store(errors.New("credential material in payload"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var payload shopCredentialEventPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			handlerErr.Store(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload.EventID != 11 || payload.EventType != "credential.changed" || payload.Sub2APIUserID != 42 || payload.CredentialVersion != 8 || payload.OccurredAt != "2026-08-02T01:02:03Z" {
			handlerErr.Store(errors.New("payload mismatch"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, `{"ok":true,"applied":true,"user_updated":true,"event_id":11,"sub2api_user_id":42,"credential_version":8}`)
	}))
	defer server.Close()

	client, err := newShopCredentialEventClient(config.ShopCredentialEventsConfig{
		Enabled: true, BaseURL: server.URL, SharedSecret: testShopCredentialSecret, TimeoutSeconds: 2,
	}, server.Client(), func() time.Time { return fixedNow })
	require.NoError(t, err)
	require.NoError(t, client.Deliver(context.Background(), ShopCredentialEvent{
		ID: 11, UserID: 42, CredentialVersion: 8, OccurredAt: occurredAt,
	}))
	if value := handlerErr.Load(); value != nil {
		require.NoError(t, value.(error))
	}
}

func TestShopCredentialEventClient_RejectsInsecureURLAndRedirects(t *testing.T) {
	_, err := NewShopCredentialEventClient(config.ShopCredentialEventsConfig{
		Enabled: true, BaseURL: "http://shop.example.com", SharedSecret: testShopCredentialSecret, TimeoutSeconds: 2,
	})
	require.ErrorContains(t, err, "HTTPS")

	var redirected atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Add(1)
	}))
	defer target.Close()
	redirector := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	client, err := newShopCredentialEventClient(config.ShopCredentialEventsConfig{
		Enabled: true, BaseURL: redirector.URL, SharedSecret: testShopCredentialSecret, TimeoutSeconds: 2,
	}, redirector.Client(), time.Now)
	require.NoError(t, err)
	err = client.Deliver(context.Background(), ShopCredentialEvent{
		ID: 12, UserID: 42, CredentialVersion: 9, OccurredAt: time.Now().UTC(),
	})
	require.ErrorContains(t, err, "HTTP 307")
	require.Zero(t, redirected.Load())
}

func TestShopCredentialEventClient_BoundsResponseBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.Repeat("x", shopCredentialEventMaxAckBytes+1))
	}))
	defer server.Close()
	client, err := newShopCredentialEventClient(config.ShopCredentialEventsConfig{
		Enabled: true, BaseURL: server.URL, SharedSecret: testShopCredentialSecret, TimeoutSeconds: 2,
	}, server.Client(), time.Now)
	require.NoError(t, err)
	err = client.Deliver(context.Background(), ShopCredentialEvent{
		ID: 13, UserID: 42, CredentialVersion: 10, OccurredAt: time.Now().UTC(),
	})
	require.ErrorContains(t, err, "response exceeds body limit")
}

func TestShopCredentialEventClient_RequiresExactSemanticAcknowledgement(t *testing.T) {
	event := ShopCredentialEvent{
		ID: 31, UserID: 73, CredentialVersion: 12, OccurredAt: time.Date(2026, time.August, 2, 3, 4, 5, 0, time.UTC),
	}
	validBody := `{"ok":true,"applied":false,"user_updated":false,"event_id":31,"sub2api_user_id":73,"credential_version":12}`
	tests := []struct {
		name         string
		status       int
		contentTypes []string
		body         string
		wantError    string
	}{
		{name: "created is not ack", status: http.StatusCreated, contentTypes: []string{"application/json"}, body: validBody, wantError: "HTTP 201"},
		{name: "no content is not ack", status: http.StatusNoContent, contentTypes: []string{"application/json"}, wantError: "HTTP 204"},
		{name: "empty body", status: http.StatusOK, contentTypes: []string{"application/json"}, wantError: "invalid semantic acknowledgement"},
		{name: "html fallback", status: http.StatusOK, contentTypes: []string{"text/html"}, body: "<html>ok</html>", wantError: "content type"},
		{name: "json suffix media type", status: http.StatusOK, contentTypes: []string{"application/problem+json"}, body: validBody, wantError: "application/json"},
		{name: "non utf8 charset", status: http.StatusOK, contentTypes: []string{"application/json; charset=iso-8859-1"}, body: validBody, wantError: "UTF-8"},
		{name: "multiple content types", status: http.StatusOK, contentTypes: []string{"application/json", "application/json"}, body: validBody, wantError: "exactly one"},
		{name: "malformed json", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{`, wantError: "invalid semantic acknowledgement"},
		{name: "error json", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"error":"fallback"}`, wantError: "unknown field"},
		{name: "wrong event id", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":true,"applied":true,"user_updated":true,"event_id":32,"sub2api_user_id":73,"credential_version":12}`, wantError: "identity mismatch"},
		{name: "wrong user id", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":true,"applied":true,"user_updated":true,"event_id":31,"sub2api_user_id":74,"credential_version":12}`, wantError: "identity mismatch"},
		{name: "wrong version", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":true,"applied":true,"user_updated":true,"event_id":31,"sub2api_user_id":73,"credential_version":13}`, wantError: "identity mismatch"},
		{name: "extra field", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":true,"applied":true,"user_updated":true,"event_id":31,"sub2api_user_id":73,"credential_version":12,"message":"ok"}`, wantError: "unknown field"},
		{name: "missing field", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":true,"applied":true,"user_updated":true,"event_id":31,"credential_version":12}`, wantError: "all six"},
		{name: "duplicate field", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":true,"ok":true,"applied":true,"user_updated":true,"event_id":31,"sub2api_user_id":73,"credential_version":12}`, wantError: "duplicate field"},
		{name: "null field", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":true,"applied":true,"user_updated":true,"event_id":31,"sub2api_user_id":73,"credential_version":null}`, wantError: "must not be null"},
		{name: "negative identity", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":true,"applied":true,"user_updated":true,"event_id":31,"sub2api_user_id":-73,"credential_version":12}`, wantError: "identity mismatch"},
		{name: "not acknowledged", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":false,"applied":true,"user_updated":true,"event_id":31,"sub2api_user_id":73,"credential_version":12}`, wantError: "was not acknowledged"},
		{name: "inconsistent state", status: http.StatusOK, contentTypes: []string{"application/json"}, body: `{"ok":true,"applied":false,"user_updated":true,"event_id":31,"sub2api_user_id":73,"credential_version":12}`, wantError: "state is inconsistent"},
		{name: "trailing json", status: http.StatusOK, contentTypes: []string{"application/json"}, body: validBody + `{}`, wantError: "trailing"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for _, contentType := range test.contentTypes {
					w.Header().Add("Content-Type", contentType)
				}
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			client, err := newShopCredentialEventClient(config.ShopCredentialEventsConfig{
				Enabled: true, BaseURL: server.URL, SharedSecret: testShopCredentialSecret, TimeoutSeconds: 2,
			}, server.Client(), time.Now)
			require.NoError(t, err)
			err = client.Deliver(context.Background(), event)
			require.ErrorContains(t, err, test.wantError)
		})
	}
}

type shopCredentialEventRepoStub struct {
	mu        sync.Mutex
	events    []ShopCredentialEvent
	deleted   []int64
	retried   []int64
	retryText string
	stats     ShopCredentialEventOutboxStats
}

func (r *shopCredentialEventRepoStub) Claim(context.Context, string, int, time.Duration) ([]ShopCredentialEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ShopCredentialEvent(nil), r.events...), nil
}

func (r *shopCredentialEventRepoStub) DeleteClaimed(_ context.Context, id int64, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *shopCredentialEventRepoStub) RetryClaimed(_ context.Context, id int64, _ string, _ time.Time, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retried = append(r.retried, id)
	r.retryText = text
	return nil
}

func (r *shopCredentialEventRepoStub) Stats(context.Context) (ShopCredentialEventOutboxStats, error) {
	return r.stats, nil
}

type shopCredentialDelivererFunc func(context.Context, ShopCredentialEvent) error

func (f shopCredentialDelivererFunc) Deliver(ctx context.Context, event ShopCredentialEvent) error {
	return f(ctx, event)
}

func TestShopCredentialEventWorker_AcksSuccessAndRetriesFailure(t *testing.T) {
	repo := &shopCredentialEventRepoStub{}
	worker := NewShopCredentialEventWorker(repo, shopCredentialDelivererFunc(func(context.Context, ShopCredentialEvent) error {
		return nil
	}), true)
	worker.processEvent(context.Background(), ShopCredentialEvent{ID: 20})
	require.Equal(t, []int64{20}, repo.deleted)
	require.Equal(t, uint64(1), worker.Health(context.Background()).Processed)

	repo = &shopCredentialEventRepoStub{}
	worker = NewShopCredentialEventWorker(repo, shopCredentialDelivererFunc(func(context.Context, ShopCredentialEvent) error {
		return errors.New("shop unavailable")
	}), true)
	worker.processEvent(context.Background(), ShopCredentialEvent{ID: 21})
	require.Equal(t, []int64{21}, repo.retried)
	require.Contains(t, repo.retryText, "shop unavailable")
	require.Equal(t, uint64(1), worker.Health(context.Background()).Failures)
}

func TestShopCredentialEventWorker_DoesNotDeleteOnGenericHTTP200(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()
	client, err := newShopCredentialEventClient(config.ShopCredentialEventsConfig{
		Enabled: true, BaseURL: server.URL, SharedSecret: testShopCredentialSecret, TimeoutSeconds: 2,
	}, server.Client(), time.Now)
	require.NoError(t, err)

	repo := &shopCredentialEventRepoStub{}
	worker := NewShopCredentialEventWorker(repo, client, true)
	event := ShopCredentialEvent{ID: 41, UserID: 81, CredentialVersion: 19, OccurredAt: time.Now().UTC()}
	worker.processEvent(context.Background(), event)

	require.Empty(t, repo.deleted)
	require.Equal(t, []int64{41}, repo.retried)
	require.Contains(t, repo.retryText, "invalid semantic acknowledgement")
	require.Equal(t, uint64(1), worker.Health(context.Background()).Failures)
}

func TestShopCredentialEventWorker_DefaultDisabledDoesNotStart(t *testing.T) {
	worker := NewShopCredentialEventWorker(&shopCredentialEventRepoStub{}, shopCredentialDelivererFunc(func(context.Context, ShopCredentialEvent) error {
		return nil
	}), false)
	worker.Start()
	time.Sleep(20 * time.Millisecond)
	require.False(t, worker.Health(context.Background()).Enabled)
	require.False(t, worker.Health(context.Background()).Running)
	require.NotPanics(t, func() { worker.Stop(); worker.Stop() })
}
