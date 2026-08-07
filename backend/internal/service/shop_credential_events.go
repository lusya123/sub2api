package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
)

const (
	shopCredentialEventType           = "credential.changed"
	shopCredentialEventPath           = "/api/v1/integrations/sub2api/credential-events"
	shopCredentialEventMaxBodyBytes   = 64 * 1024
	shopCredentialEventMaxAckBytes    = 4 * 1024
	shopCredentialEventBatchSize      = 100
	shopCredentialEventConcurrency    = 8
	shopCredentialEventPollInterval   = time.Second
	shopCredentialEventLease          = 90 * time.Second
	shopCredentialEventReleaseTimeout = 2 * time.Second
)

type ShopCredentialEvent struct {
	ID                int64
	UserID            int64
	CredentialVersion uint64
	Attempts          int
	OccurredAt        time.Time
	CreatedAt         time.Time
}

type ShopCredentialEventOutboxStats struct {
	Pending         int64
	OldestCreatedAt *time.Time
	MaxAttempts     int
	LastError       string
}

type ShopCredentialEventOutboxRepository interface {
	Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]ShopCredentialEvent, error)
	DeleteClaimed(ctx context.Context, id int64, workerID string) error
	RetryClaimed(ctx context.Context, id int64, workerID string, availableAt time.Time, lastError string) error
	Stats(ctx context.Context) (ShopCredentialEventOutboxStats, error)
}

type ShopCredentialEventDeliverer interface {
	Deliver(ctx context.Context, event ShopCredentialEvent) error
}

type shopCredentialEventPayload struct {
	EventID           int64  `json:"event_id"`
	EventType         string `json:"event_type"`
	Sub2APIUserID     int64  `json:"sub2api_user_id"`
	CredentialVersion uint64 `json:"credential_version"`
	OccurredAt        string `json:"occurred_at"`
}

// shopCredentialEventAck is deliberately bound to the event identity. A
// generic 2xx response (for example, from a proxy fallback or a wrong TLS
// origin) must never cause the durable outbox row to be deleted.
type shopCredentialEventAck struct {
	OK                bool
	Applied           bool
	UserUpdated       bool
	EventID           int64
	Sub2APIUserID     int64
	CredentialVersion uint64
}

type ShopCredentialEventClient struct {
	enabled    bool
	endpoint   string
	secret     []byte
	httpClient *http.Client
	now        func() time.Time
}

func NewShopCredentialEventClient(cfg config.ShopCredentialEventsConfig) (*ShopCredentialEventClient, error) {
	return newShopCredentialEventClient(cfg, nil, time.Now)
}

func newShopCredentialEventClient(cfg config.ShopCredentialEventsConfig, baseClient *http.Client, now func() time.Time) (*ShopCredentialEventClient, error) {
	client := &ShopCredentialEventClient{enabled: cfg.Enabled, now: now}
	if !cfg.Enabled {
		return client, nil
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Opaque != "" {
		return nil, errors.New("shop credential event base URL must be an absolute HTTPS origin")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.EscapedPath() != "" && parsed.EscapedPath() != "/") {
		return nil, errors.New("shop credential event base URL must not contain credentials, path, query, or fragment")
	}
	secret := strings.TrimSpace(cfg.SharedSecret)
	if len([]byte(secret)) < 32 {
		return nil, errors.New("shop credential event shared secret must be at least 32 bytes")
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout < time.Second || timeout > 60*time.Second {
		return nil, errors.New("shop credential event timeout must be between 1 and 60 seconds")
	}

	parsed.Path = shopCredentialEventPath
	parsed.RawPath = ""
	client.endpoint = parsed.String()
	client.secret = []byte(secret)
	client.httpClient = secureShopCredentialHTTPClient(baseClient, timeout)
	if client.now == nil {
		client.now = time.Now
	}
	return client, nil
}

func secureShopCredentialHTTPClient(baseClient *http.Client, timeout time.Duration) *http.Client {
	var client http.Client
	if baseClient != nil {
		client = *baseClient
	} else {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		} else {
			transport.TLSClientConfig = transport.TLSClientConfig.Clone()
			if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
				transport.TLSClientConfig.MinVersion = tls.VersionTLS12
			}
		}
		client.Transport = transport
	}
	client.Timeout = timeout
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &client
}

func (c *ShopCredentialEventClient) Deliver(ctx context.Context, event ShopCredentialEvent) error {
	if c == nil || !c.enabled || c.httpClient == nil {
		return errors.New("shop credential event delivery is disabled")
	}
	if event.ID <= 0 || event.UserID <= 0 || event.CredentialVersion == 0 || event.OccurredAt.IsZero() {
		return fmt.Errorf("invalid shop credential event %d", event.ID)
	}
	payload := shopCredentialEventPayload{
		EventID:           event.ID,
		EventType:         shopCredentialEventType,
		Sub2APIUserID:     event.UserID,
		CredentialVersion: event.CredentialVersion,
		OccurredAt:        event.OccurredAt.UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal shop credential event %d: %w", event.ID, err)
	}
	if len(body) > shopCredentialEventMaxBodyBytes {
		return fmt.Errorf("shop credential event %d exceeds body limit", event.ID)
	}

	timestamp := strconv.FormatInt(c.now().UTC().Unix(), 10)
	mac := hmac.New(sha256.New, c.secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build shop credential event %d request: %w", event.ID, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sub2API-Timestamp", timestamp)
	req.Header.Set("X-Sub2API-Signature", signature)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deliver shop credential event %d: %w", event.ID, err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, shopCredentialEventMaxAckBytes+1))
	if err != nil {
		return fmt.Errorf("read shop credential event %d response: %w", event.ID, err)
	}
	if len(responseBody) > shopCredentialEventMaxAckBytes {
		return fmt.Errorf("shop credential event %d response exceeds body limit", event.ID)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("shop credential event %d returned HTTP %d", event.ID, resp.StatusCode)
	}
	if err := validateShopCredentialEventAckContentType(resp.Header); err != nil {
		return fmt.Errorf("shop credential event %d response content type: %w", event.ID, err)
	}
	ack, err := decodeShopCredentialEventAck(responseBody)
	if err != nil {
		return fmt.Errorf("shop credential event %d invalid semantic acknowledgement: %w", event.ID, err)
	}
	if !ack.OK {
		return fmt.Errorf("shop credential event %d was not acknowledged", event.ID)
	}
	if ack.UserUpdated && !ack.Applied {
		return fmt.Errorf("shop credential event %d acknowledgement state is inconsistent", event.ID)
	}
	if ack.EventID != event.ID || ack.Sub2APIUserID != event.UserID || ack.CredentialVersion != event.CredentialVersion {
		return fmt.Errorf("shop credential event %d acknowledgement identity mismatch", event.ID)
	}
	return nil
}

func validateShopCredentialEventAckContentType(header http.Header) error {
	values := header.Values("Content-Type")
	if len(values) != 1 {
		return errors.New("exactly one Content-Type header is required")
	}
	mediaType, params, err := mime.ParseMediaType(values[0])
	if err != nil || mediaType != "application/json" {
		return errors.New("application/json is required")
	}
	for key, value := range params {
		if !strings.EqualFold(key, "charset") || !strings.EqualFold(value, "utf-8") {
			return errors.New("only the UTF-8 charset parameter is allowed")
		}
	}
	return nil
}

func decodeShopCredentialEventAck(body []byte) (shopCredentialEventAck, error) {
	var ack shopCredentialEventAck
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil {
		return ack, err
	}
	if delimiter, ok := start.(json.Delim); !ok || delimiter != '{' {
		return ack, errors.New("JSON object is required")
	}

	seen := make(map[string]struct{}, 6)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return ack, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return ack, errors.New("invalid acknowledgement field name")
		}
		if _, duplicate := seen[key]; duplicate {
			return ack, fmt.Errorf("duplicate field %q", key)
		}
		seen[key] = struct{}{}

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return ack, fmt.Errorf("decode field %q: %w", key, err)
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return ack, fmt.Errorf("field %q must not be null", key)
		}
		switch key {
		case "ok":
			err = json.Unmarshal(raw, &ack.OK)
		case "applied":
			err = json.Unmarshal(raw, &ack.Applied)
		case "user_updated":
			err = json.Unmarshal(raw, &ack.UserUpdated)
		case "event_id":
			err = json.Unmarshal(raw, &ack.EventID)
		case "sub2api_user_id":
			err = json.Unmarshal(raw, &ack.Sub2APIUserID)
		case "credential_version":
			err = json.Unmarshal(raw, &ack.CredentialVersion)
		default:
			return ack, fmt.Errorf("unknown field %q", key)
		}
		if err != nil {
			return ack, fmt.Errorf("invalid field %q: %w", key, err)
		}
	}
	end, err := decoder.Token()
	if err != nil {
		return ack, err
	}
	if delimiter, ok := end.(json.Delim); !ok || delimiter != '}' {
		return ack, errors.New("invalid acknowledgement object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return ack, errors.New("trailing JSON value is not allowed")
		}
		return ack, fmt.Errorf("invalid trailing data: %w", err)
	}
	if len(seen) != 6 {
		return ack, errors.New("all six acknowledgement fields are required")
	}
	return ack, nil
}

type ShopCredentialEventWorkerHealth struct {
	Enabled     bool          `json:"enabled"`
	Running     bool          `json:"running"`
	Processed   uint64        `json:"processed"`
	Failures    uint64        `json:"failures"`
	Pending     int64         `json:"pending"`
	OldestLag   time.Duration `json:"oldest_lag"`
	MaxAttempts int           `json:"max_attempts"`
	LastError   string        `json:"last_error,omitempty"`
	StatsError  string        `json:"stats_error,omitempty"`
}

type ShopCredentialEventWorker struct {
	enabled   bool
	repo      ShopCredentialEventOutboxRepository
	deliverer ShopCredentialEventDeliverer
	workerID  string
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	start     sync.Once
	stop      sync.Once
	running   atomic.Bool
	processed atomic.Uint64
	failures  atomic.Uint64
	lastError atomic.Value
}

func NewShopCredentialEventWorker(repo ShopCredentialEventOutboxRepository, deliverer ShopCredentialEventDeliverer, enabled bool) *ShopCredentialEventWorker {
	ctx, cancel := context.WithCancel(context.Background())
	worker := &ShopCredentialEventWorker{
		enabled: enabled, repo: repo, deliverer: deliverer, workerID: uuid.NewString(), ctx: ctx, cancel: cancel,
	}
	worker.lastError.Store("")
	return worker
}

func (w *ShopCredentialEventWorker) Start() {
	if w == nil || !w.enabled || w.repo == nil || w.deliverer == nil {
		return
	}
	w.start.Do(func() {
		w.running.Store(true)
		w.wg.Add(1)
		go w.run()
	})
}

func (w *ShopCredentialEventWorker) Stop() {
	if w == nil {
		return
	}
	w.stop.Do(func() {
		w.cancel()
		w.wg.Wait()
		w.running.Store(false)
	})
}

func (w *ShopCredentialEventWorker) run() {
	defer w.wg.Done()
	defer w.running.Store(false)
	ticker := time.NewTicker(shopCredentialEventPollInterval)
	defer ticker.Stop()
	for {
		if err := w.processBatch(w.ctx); err != nil && w.ctx.Err() == nil {
			w.recordFailure(err)
		}
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *ShopCredentialEventWorker) processBatch(ctx context.Context) error {
	events, err := w.repo.Claim(ctx, w.workerID, shopCredentialEventBatchSize, shopCredentialEventLease)
	if err != nil {
		return fmt.Errorf("claim shop credential events: %w", err)
	}
	semaphore := make(chan struct{}, shopCredentialEventConcurrency)
	var wg sync.WaitGroup
	for i := range events {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case semaphore <- struct{}{}:
		}
		wg.Add(1)
		go func(event ShopCredentialEvent) {
			defer wg.Done()
			defer func() { <-semaphore }()
			w.processEvent(ctx, event)
		}(events[i])
	}
	wg.Wait()
	return nil
}

func (w *ShopCredentialEventWorker) processEvent(parent context.Context, event ShopCredentialEvent) {
	if err := w.deliverer.Deliver(parent, event); err != nil {
		w.recordFailure(err)
		retryAt := time.Now().UTC().Add(shopCredentialEventRetryDelay(event.Attempts + 1))
		retryCtx, cancel := context.WithTimeout(context.Background(), shopCredentialEventReleaseTimeout)
		retryErr := w.repo.RetryClaimed(retryCtx, event.ID, w.workerID, retryAt, boundedShopCredentialEventError(err))
		cancel()
		if retryErr != nil {
			w.recordFailure(fmt.Errorf("release failed shop credential event %d: %w", event.ID, retryErr))
		}
		return
	}

	ackCtx, cancel := context.WithTimeout(context.Background(), shopCredentialEventReleaseTimeout)
	err := w.repo.DeleteClaimed(ackCtx, event.ID, w.workerID)
	cancel()
	if err != nil {
		w.recordFailure(fmt.Errorf("ack shop credential event %d: %w", event.ID, err))
		return
	}
	w.processed.Add(1)
	w.lastError.Store("")
}

func shopCredentialEventRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 9 {
		attempt = 9
	}
	base := time.Second * time.Duration(1<<(attempt-1))
	return time.Duration(float64(base) * (0.8 + rand.Float64()*0.4))
}

func boundedShopCredentialEventError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 1024 {
		return message[:1024]
	}
	return message
}

func (w *ShopCredentialEventWorker) recordFailure(err error) {
	if err == nil {
		return
	}
	w.failures.Add(1)
	w.lastError.Store(boundedShopCredentialEventError(err))
	slog.Warn("shop credential event outbox processing failed", "error", err)
}

func (w *ShopCredentialEventWorker) Health(ctx context.Context) ShopCredentialEventWorkerHealth {
	health := ShopCredentialEventWorkerHealth{}
	if w == nil {
		return health
	}
	health.Enabled = w.enabled
	health.Running = w.running.Load()
	health.Processed = w.processed.Load()
	health.Failures = w.failures.Load()
	if value := w.lastError.Load(); value != nil {
		health.LastError, _ = value.(string)
	}
	if w.repo == nil {
		return health
	}
	stats, err := w.repo.Stats(ctx)
	if err != nil {
		health.StatsError = boundedShopCredentialEventError(err)
		return health
	}
	health.Pending = stats.Pending
	health.MaxAttempts = stats.MaxAttempts
	if health.LastError == "" {
		health.LastError = stats.LastError
	}
	if stats.OldestCreatedAt != nil {
		health.OldestLag = time.Since(*stats.OldestCreatedAt)
		if health.OldestLag < 0 {
			health.OldestLag = 0
		}
	}
	return health
}

func ProvideShopCredentialEventWorker(repo ShopCredentialEventOutboxRepository, cfg *config.Config) (*ShopCredentialEventWorker, error) {
	eventConfig := config.ShopCredentialEventsConfig{}
	if cfg != nil {
		eventConfig = cfg.ShopCredentialEvents
	}
	client, err := NewShopCredentialEventClient(eventConfig)
	if err != nil {
		return nil, err
	}
	worker := NewShopCredentialEventWorker(repo, client, eventConfig.Enabled)
	if eventConfig.Enabled {
		worker.Start()
	}
	return worker, nil
}
