// Package service: GlobeService powers the live world map dashboard.
//
// # Architectural intent
//
// The globe view (public marketing showcase + admin ops board) is read entirely
// off a single in-process broadcaster. The hot path looks like this:
//
//	┌────────────────┐    every 5 min   ┌─────────────────┐    fan-out    ┌──────────┐
//	│ DB tick query  │ ───────────────▶ │ snapshot frame  │ ────────────▶ │ N SSE    │
//	│ usage_logs +   │                  │ (small JSON)    │               │ clients  │
//	│ ip_geo_cache   │                  │                 │               │          │
//	└────────────────┘                  └─────────────────┘               └──────────┘
//
// Crucially: 1 viewer or 10 000 viewers = SAME database load (one query / 5 min).
// All animation density (light arcs, particles) is synthesised client-side from
// the per-country call counts the snapshot reports — that's how Cloudflare Radar
// and GitHub Globe stay cheap.
//
// An optional second goroutine can geocode IPs we haven't seen before when a
// deployment explicitly enables and configures a lookup provider. It is off by
// default so production does not silently transmit user IPs to a third party.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ----- Public types --------------------------------------------------------

// GlobeArc is one bundle of synthetic events the frontend will paint as light
// arcs from origin → destination during the next snapshot interval.
type GlobeArc struct {
	IP          string  `json:"-"`
	CountryCode string  `json:"cc"`
	Country     string  `json:"country"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Calls       int     `json:"calls"`
	// IPMask is a privacy-friendly rendering of the IP for the public view
	// (e.g. "47.82.86.•••"). Raw IP is internal-only and never serialized by
	// the anonymous public globe endpoints.
	IPMask string `json:"ip_mask,omitempty"`
}

// GlobeCountry aggregates calls by ISO country for the side panel & heat layer.
type GlobeCountry struct {
	CountryCode string  `json:"cc"`
	Country     string  `json:"country"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Calls       int     `json:"calls"`
}

// GlobeSnapshot is the unit of data pushed via SSE every tick.
type GlobeSnapshot struct {
	GeneratedAt    time.Time      `json:"generated_at"`
	WindowMs       int64          `json:"window_ms"`
	IntervalMs     int64          `json:"interval_ms"`
	Arcs           []GlobeArc     `json:"arcs"`
	Countries      []GlobeCountry `json:"countries"`
	TotalCalls     int            `json:"total_calls"`
	UniqueIPs      int            `json:"unique_ips"`
	UnresolvedIPs  int            `json:"unresolved_ips"`
	GeoCacheSize   int            `json:"geo_cache_size"`
	ServerLocation *ServerPoint   `json:"server_location,omitempty"`
}

// ServerPoint is the rendered destination of every arc (i.e. our gateway).
// In the future this can be split per-region.
type ServerPoint struct {
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Label string  `json:"label"`
}

// GlobeSummary is the heavyweight rollup served via REST (cached 30s) for the
// public landing hero numbers — totals over 24h / 30d.
type GlobeSummary struct {
	GeneratedAt    time.Time          `json:"generated_at"`
	Window24h      GlobeSummaryBucket `json:"window_24h"`
	WindowAllTime  GlobeSummaryBucket `json:"window_all_time"`
	TopCountries   []GlobeCountry     `json:"top_countries"`
	HourlyHistory  []GlobeHourBucket  `json:"hourly_history_24h"`
	ServerLocation *ServerPoint       `json:"server_location,omitempty"`
	GeoCoverage    map[string]any     `json:"geo_coverage"`
}

// GlobeSummaryBucket is a totals-only block, used for both 24h and lifetime.
type GlobeSummaryBucket struct {
	Calls           int64 `json:"calls"`
	UniqueIPs       int64 `json:"unique_ips"`
	UniqueCountries int64 `json:"unique_countries"`
}

// GlobeHourBucket is one column of the activity sparkline.
type GlobeHourBucket struct {
	HourUTC string `json:"hour_utc"`
	Calls   int64  `json:"calls"`
}

// ----- Service -------------------------------------------------------------

// GlobeService is a singleton goroutine-backed service.
type GlobeService struct {
	db *sql.DB

	// broadcaster
	mu        sync.RWMutex
	subs      map[chan *GlobeSnapshot]struct{}
	lastSnap  atomic.Pointer[GlobeSnapshot]
	cachedSum atomic.Pointer[GlobeSummary]

	// config
	interval        time.Duration
	geoBackfillIntv time.Duration
	geoBurstIntv    time.Duration
	geoBurstWindow  time.Duration
	geoBatchSize    int
	geoBackfillOn   bool
	geoLookupURL    string
	httpClient      *http.Client
	startedAt       time.Time

	// optional override for tests
	now func() time.Time

	// lifecycle
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

const (
	snapshotInterval       = 5 * time.Minute
	snapshotReplayMaxCalls = 1
)

// NewGlobeService is the wire constructor. db can be nil — the service will
// run as a no-op (returning empty snapshots) in that case so the build never
// breaks just because PG isn't reachable yet.
func NewGlobeService(db *sql.DB) *GlobeService {
	geoLookupURL := strings.TrimSpace(os.Getenv("SUB2API_GLOBE_GEO_LOOKUP_URL"))
	return &GlobeService{
		db:              db,
		subs:            make(map[chan *GlobeSnapshot]struct{}),
		interval:        snapshotInterval,
		geoBackfillIntv: 30 * time.Second,
		// Burst settings: for the first minutes after boot we hammer ip-api
		// at the upper limit of its free tier (15 batches/min ≈ 1 batch / 4s)
		// so the globe lights up within ~1 minute on a fresh deploy. Once
		// every still-active IP is resolved we relax to the steady cadence.
		geoBurstIntv:   5 * time.Second,
		geoBurstWindow: 5 * time.Minute,
		geoBatchSize:   100,
		geoBackfillOn:  envBool("SUB2API_GLOBE_GEO_BACKFILL_ENABLED") && geoLookupURL != "",
		geoLookupURL:   geoLookupURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		now: time.Now,
	}
}

// Start launches background goroutines. Safe to call once at boot.
func (s *GlobeService) Start(ctx context.Context) {
	if s == nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.startedAt = s.now()

	s.wg.Add(2)
	go s.runSnapshotLoop(ctx)
	go s.runGeoBackfillLoop(ctx)
}

// Stop signals all goroutines and waits.
func (s *GlobeService) Stop() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
}

// Subscribe registers a new SSE client. The returned channel is buffered;
// slow consumers are dropped (we never block the broadcaster).
func (s *GlobeService) Subscribe() (<-chan *GlobeSnapshot, func()) {
	ch := make(chan *GlobeSnapshot, 4)

	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()

	// Push the latest snapshot immediately so first-paint isn't blank.
	if snap := s.lastSnap.Load(); snap != nil {
		select {
		case ch <- snap:
		default:
		}
	}

	unsub := func() {
		s.mu.Lock()
		if _, ok := s.subs[ch]; ok {
			delete(s.subs, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
	return ch, unsub
}

// Snapshot returns the most recent broadcast frame for one-shot REST callers.
func (s *GlobeService) Snapshot() *GlobeSnapshot {
	if s == nil {
		return emptySnapshot()
	}
	if snap := s.lastSnap.Load(); snap != nil {
		return snap
	}
	return emptySnapshot()
}

// SnapshotWithContext serves one-shot HTTP callers. If the background first
// tick failed during a slow production-DB startup, build once inline so the
// browser does not stay on an empty cached frame until the next 5-minute tick.
func (s *GlobeService) SnapshotWithContext(ctx context.Context) *GlobeSnapshot {
	if s == nil {
		return emptySnapshot()
	}
	if snap := s.lastSnap.Load(); snap != nil && (len(snap.Arcs) > 0 || snap.TotalCalls > 0 || snap.UniqueIPs > 0) {
		return snap
	}
	snap, err := s.buildSnapshot(ctx)
	if err != nil {
		if cached := s.lastSnap.Load(); cached != nil {
			return cached
		}
		return emptySnapshot()
	}
	s.lastSnap.Store(snap)
	return snap
}

// Summary returns a 24h / lifetime rollup — cached for 30s.
func (s *GlobeService) Summary(ctx context.Context) (*GlobeSummary, error) {
	if s == nil {
		return emptySummary(), nil
	}
	if cached := s.cachedSum.Load(); cached != nil {
		if time.Since(cached.GeneratedAt) < 30*time.Second {
			return cached, nil
		}
	}
	sum, err := s.buildSummary(ctx)
	if err != nil {
		// Serve stale on error rather than 500 — globe should never blank
		// out the homepage just because the DB hiccuped for a tick.
		if cached := s.cachedSum.Load(); cached != nil {
			return cached, nil
		}
		return emptySummary(), err
	}
	s.cachedSum.Store(sum)
	return sum, nil
}

// ----- Snapshot loop -------------------------------------------------------

func (s *GlobeService) runSnapshotLoop(ctx context.Context) {
	defer s.wg.Done()

	t := time.NewTicker(s.interval)
	defer t.Stop()

	// Tick immediately on boot so the first SSE client doesn't wait for the
	// next 5-minute refresh.
	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

func (s *GlobeService) tick(ctx context.Context) {
	snap, err := s.buildSnapshot(ctx)
	if err != nil {
		// Don't kill the loop on a bad tick. Carry the previous snapshot.
		return
	}
	s.lastSnap.Store(snap)

	s.mu.RLock()
	for ch := range s.subs {
		select {
		case ch <- snap:
		default:
			// Slow client — let the next frame catch them up. We do NOT
			// drop them, because they may simply be in middle of a render.
		}
	}
	s.mu.RUnlock()
}

func (s *GlobeService) buildSnapshot(ctx context.Context) (*GlobeSnapshot, error) {
	now := s.now()
	winMs := int64(s.interval / time.Millisecond)

	if s.db == nil {
		return &GlobeSnapshot{
			GeneratedAt:    now,
			WindowMs:       winMs,
			IntervalMs:     winMs,
			ServerLocation: defaultServerPoint(),
		}, nil
	}

	// Snapshot refreshes every 5 minutes and reflects real calls from that
	// window. Geo cache is only used to attach route geometry to recent usage.
	const q = `
SELECT
  ul.ip_address,
  COALESCE(g.country,        ''),
  COALESCE(g.country_code,   ''),
  COALESCE(g.region,         ''),
  COALESCE(g.city,           ''),
  COALESCE(g.lat,            0),
  COALESCE(g.lng,            0),
  COUNT(*) AS calls
FROM usage_logs ul
JOIN ip_geo_cache g ON g.ip = ul.ip_address
WHERE ul.created_at > NOW() - ($1 * interval '1 millisecond')
  AND ul.ip_address IS NOT NULL
  AND g.country_code <> ''
  AND NOT (COALESCE(g.lat, 0) = 0 AND COALESCE(g.lng, 0) = 0)
GROUP BY ul.ip_address, g.country, g.country_code, g.region, g.city, g.lat, g.lng
ORDER BY MAX(ul.created_at) DESC, calls DESC`

	rows, err := s.db.QueryContext(ctx, q, winMs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	arcs := make([]GlobeArc, 0, 256)
	countriesByCC := make(map[string]*GlobeCountry, 32)
	totalCalls := 0
	seenIPs := 0

	for rows.Next() {
		var (
			ip, country, cc, region, city string
			lat, lng                      float64
			calls                         int
		)
		if err := rows.Scan(&ip, &country, &cc, &region, &city, &lat, &lng, &calls); err != nil {
			return nil, err
		}
		if calls < 1 {
			calls = 1
		}
		seenIPs++
		totalCalls += calls
		arcs = append(arcs, GlobeArc{
			IP:          ip,
			CountryCode: cc,
			Country:     country,
			Region:      region,
			City:        city,
			Lat:         lat,
			Lng:         lng,
			Calls:       calls,
			IPMask:      maskIP(ip),
		})
		c := countriesByCC[cc]
		if c == nil {
			c = &GlobeCountry{
				CountryCode: cc,
				Country:     country,
				Lat:         lat,
				Lng:         lng,
			}
			countriesByCC[cc] = c
		}
		c.Calls += calls
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	countries := make([]GlobeCountry, 0, len(countriesByCC))
	for _, c := range countriesByCC {
		countries = append(countries, *c)
	}
	sort.Slice(countries, func(i, j int) bool { return countries[i].Calls > countries[j].Calls })

	snap := &GlobeSnapshot{
		GeneratedAt:    now,
		WindowMs:       winMs,
		IntervalMs:     winMs,
		Arcs:           arcs,
		Countries:      countries,
		TotalCalls:     totalCalls,
		UniqueIPs:      seenIPs,
		ServerLocation: defaultServerPoint(),
	}

	// Cheap cache size — single row, runs once per tick.
	if row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_geo_cache`); row != nil {
		_ = row.Scan(&snap.GeoCacheSize)
	}
	if row := s.db.QueryRowContext(ctx, `
SELECT COUNT(DISTINCT ul.ip_address)
FROM usage_logs ul
LEFT JOIN ip_geo_cache g ON g.ip = ul.ip_address
WHERE ul.created_at > NOW() - ($1 * interval '1 millisecond')
  AND ul.ip_address IS NOT NULL
  AND (
    g.ip IS NULL
    OR COALESCE(g.country_code, '') = ''
    OR (COALESCE(g.lat, 0) = 0 AND COALESCE(g.lng, 0) = 0)
  )`, winMs); row != nil {
		_ = row.Scan(&snap.UnresolvedIPs)
	}

	return snap, nil
}

// ----- Summary (REST) -------------------------------------------------------

func (s *GlobeService) buildSummary(ctx context.Context) (*GlobeSummary, error) {
	now := s.now()
	out := &GlobeSummary{
		GeneratedAt:    now,
		ServerLocation: defaultServerPoint(),
		GeoCoverage:    map[string]any{},
	}

	if s.db == nil {
		return out, nil
	}

	// 24h totals
	{
		row := s.db.QueryRowContext(ctx, `
SELECT
  COUNT(*),
  COUNT(DISTINCT ip_address),
  COUNT(DISTINCT g.country_code)
FROM usage_logs ul
LEFT JOIN ip_geo_cache g ON g.ip = ul.ip_address
WHERE ul.created_at > NOW() - interval '24 hours'
  AND ul.ip_address IS NOT NULL`)
		_ = row.Scan(&out.Window24h.Calls, &out.Window24h.UniqueIPs, &out.Window24h.UniqueCountries)
	}

	// All-time totals
	{
		row := s.db.QueryRowContext(ctx, `
SELECT
  COUNT(*),
  COUNT(DISTINCT ip_address),
  COUNT(DISTINCT g.country_code)
FROM usage_logs ul
LEFT JOIN ip_geo_cache g ON g.ip = ul.ip_address
WHERE ul.ip_address IS NOT NULL`)
		_ = row.Scan(&out.WindowAllTime.Calls, &out.WindowAllTime.UniqueIPs, &out.WindowAllTime.UniqueCountries)
	}

	// Top countries (24h, by call volume)
	{
		rows, err := s.db.QueryContext(ctx, `
SELECT
  COALESCE(g.country_code, ''),
  COALESCE(g.country, ''),
  COALESCE(g.lat, 0),
  COALESCE(g.lng, 0),
  COUNT(*) AS n
FROM usage_logs ul
JOIN ip_geo_cache g ON g.ip = ul.ip_address
WHERE ul.created_at > NOW() - interval '24 hours'
  AND g.country_code <> ''
GROUP BY g.country_code, g.country, g.lat, g.lng
ORDER BY n DESC
LIMIT 12`)
		if err == nil {
			for rows.Next() {
				var c GlobeCountry
				var n int64
				if err := rows.Scan(&c.CountryCode, &c.Country, &c.Lat, &c.Lng, &n); err == nil {
					c.Calls = int(n)
					out.TopCountries = append(out.TopCountries, c)
				}
			}
			_ = rows.Close()
		}
	}

	// 24h hourly sparkline
	{
		rows, err := s.db.QueryContext(ctx, `
SELECT
  date_trunc('hour', created_at AT TIME ZONE 'UTC') AS h,
  COUNT(*)
FROM usage_logs
WHERE created_at > NOW() - interval '24 hours'
  AND ip_address IS NOT NULL
GROUP BY h
ORDER BY h ASC`)
		if err == nil {
			for rows.Next() {
				var h time.Time
				var n int64
				if err := rows.Scan(&h, &n); err == nil {
					out.HourlyHistory = append(out.HourlyHistory, GlobeHourBucket{
						HourUTC: h.UTC().Format("2006-01-02T15:04:05Z"),
						Calls:   n,
					})
				}
			}
			_ = rows.Close()
		}
	}

	// Geo-cache coverage
	{
		var total, resolved int64
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT ip_address) FROM usage_logs WHERE ip_address IS NOT NULL`).Scan(&total)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_geo_cache WHERE country_code <> ''`).Scan(&resolved)
		out.GeoCoverage["total_distinct_ips"] = total
		out.GeoCoverage["resolved_ips"] = resolved
		if total > 0 {
			out.GeoCoverage["coverage_pct"] = float64(resolved) * 100 / float64(total)
		}
	}

	return out, nil
}

// ----- Geo backfill --------------------------------------------------------

func (s *GlobeService) runGeoBackfillLoop(ctx context.Context) {
	defer s.wg.Done()

	if s.db == nil {
		return
	}
	if !s.geoBackfillOn || strings.TrimSpace(s.geoLookupURL) == "" {
		return
	}

	// Adaptive ticker: burst cadence for the first geoBurstWindow after
	// boot (so a fresh deploy resolves its IP backlog within ~1 minute),
	// then relax to the steady cadence (every 30s) for ongoing maintenance.
	intvFor := func() time.Duration {
		if s.now().Sub(s.startedAt) < s.geoBurstWindow {
			return s.geoBurstIntv
		}
		return s.geoBackfillIntv
	}

	t := time.NewTimer(0) // fire immediately on boot
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.geoBackfillTick(ctx)
			t.Reset(intvFor())
		}
	}
}

func (s *GlobeService) geoBackfillTick(ctx context.Context) {
	ips, err := s.fetchPendingIPs(ctx, s.geoBatchSize)
	if err != nil || len(ips) == 0 {
		return
	}
	results, err := s.batchGeoLookup(ctx, ips)
	if err != nil {
		// Mark these IPs as failed so we don't infinite-retry — but only
		// after a few tries. For simplicity here: insert with status=fail
		// for IPs we couldn't resolve, so we move on and try fresh ones.
		for _, ip := range ips {
			_, _ = s.db.ExecContext(ctx, `
INSERT INTO ip_geo_cache (ip, status, looked_up_at) VALUES ($1, 'fail', NOW())
ON CONFLICT (ip) DO NOTHING`, ip)
		}
		return
	}
	for _, r := range results {
		if r.Query == "" {
			continue
		}
		status := r.Status
		if status == "" {
			status = "ok"
		}
		_, _ = s.db.ExecContext(ctx, `
INSERT INTO ip_geo_cache (ip, country, country_code, region, city, lat, lng, asn, isp, status, looked_up_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
ON CONFLICT (ip) DO UPDATE SET
  country = EXCLUDED.country,
  country_code = EXCLUDED.country_code,
  region = EXCLUDED.region,
  city = EXCLUDED.city,
  lat = EXCLUDED.lat,
  lng = EXCLUDED.lng,
  asn = EXCLUDED.asn,
  isp = EXCLUDED.isp,
  status = EXCLUDED.status,
  looked_up_at = NOW()`,
			r.Query, r.Country, r.CountryCode, r.RegionName, r.City, r.Lat, r.Lon, r.AS, r.Isp, status)
	}
}

func (s *GlobeService) fetchPendingIPs(ctx context.Context, limit int) ([]string, error) {
	const q = `
SELECT DISTINCT ul.ip_address
FROM usage_logs ul
LEFT JOIN ip_geo_cache g ON g.ip = ul.ip_address
WHERE ul.ip_address IS NOT NULL
	  AND (g.ip IS NULL OR (g.status = 'fail' AND g.looked_up_at < NOW() - interval '24 hours'))
LIMIT $1`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ips []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err == nil && ip != "" {
			ips = append(ips, ip)
		}
	}
	return ips, rows.Err()
}

// ipApiResult mirrors the JSON returned by ip-api.com /batch.
type ipApiResult struct {
	Query       string  `json:"query"`
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	AS          string  `json:"as"`
	Isp         string  `json:"isp"`
}

// batchGeoLookup uses an explicitly configured provider URL. The default
// service configuration never calls this path; deployments must opt in with
// SUB2API_GLOBE_GEO_BACKFILL_ENABLED=true and SUB2API_GLOBE_GEO_LOOKUP_URL.
func (s *GlobeService) batchGeoLookup(ctx context.Context, ips []string) ([]ipApiResult, error) {
	if len(ips) == 0 {
		return nil, nil
	}
	endpoint := strings.TrimSpace(s.geoLookupURL)
	if endpoint == "" {
		return nil, fmt.Errorf("globe geo lookup endpoint is not configured")
	}
	body, _ := json.Marshal(ips)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ip-api: %d: %s", resp.StatusCode, string(b))
	}
	var out []ipApiResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// ----- helpers --------------------------------------------------------------

func emptySnapshot() *GlobeSnapshot {
	return &GlobeSnapshot{
		GeneratedAt:    time.Now(),
		IntervalMs:     int64(snapshotInterval / time.Millisecond),
		WindowMs:       int64(snapshotInterval / time.Millisecond),
		ServerLocation: defaultServerPoint(),
	}
}

func emptySummary() *GlobeSummary {
	return &GlobeSummary{
		GeneratedAt:    time.Now(),
		ServerLocation: defaultServerPoint(),
		GeoCoverage:    map[string]any{},
	}
}

// defaultServerPoint pins the visual ORIGIN of every arc to Beijing — the
// brand-side narrative location that sub2api radiates token to the world
// from. (The physical gateway runs in Tencent Cloud HK, but storytelling
// trumps geography here: customers see "we ship token from Beijing to your
// city".) If you re-skin this for a different brand, update both lat/lng
// and label.
func defaultServerPoint() *ServerPoint {
	return &ServerPoint{
		Lat:   39.9042,
		Lng:   116.4074,
		Label: "sub2api · Beijing",
	}
}

// maskIP returns a privacy-friendly string for the public globe view.
//
//	"47.82.86.196"            → "47.82.86.•••"
//	"2001:db8::1"             → "2001:db8::•••"
func maskIP(ip string) string {
	if strings.Contains(ip, ".") {
		idx := strings.LastIndex(ip, ".")
		if idx <= 0 {
			return "•••"
		}
		return ip[:idx+1] + "•••"
	}
	if strings.Contains(ip, ":") {
		idx := strings.LastIndex(ip, ":")
		if idx <= 0 {
			return "•••"
		}
		return ip[:idx+1] + "•••"
	}
	return "•••"
}
