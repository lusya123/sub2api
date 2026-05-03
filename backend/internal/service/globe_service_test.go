package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestGlobeServiceNoDBReturnsEmptySnapshots(t *testing.T) {
	// With no DB attached the service must run as a no-op rather than panic —
	// boot path requirement: if PG is unreachable the homepage should still
	// load.
	s := NewGlobeService(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	defer s.Stop()

	// Give it a beat to tick.
	time.Sleep(50 * time.Millisecond)

	snap := s.Snapshot()
	if snap == nil {
		t.Fatal("expected non-nil snapshot from no-DB service")
	}
	if snap.ServerLocation == nil || snap.ServerLocation.Label == "" {
		t.Fatal("expected fallback server location to be populated")
	}
	if len(snap.Arcs) != 0 || snap.TotalCalls != 0 {
		t.Fatal("expected zero traffic when DB is nil")
	}

	sum, err := s.Summary(ctx)
	if err != nil {
		t.Fatalf("Summary returned error with nil DB: %v", err)
	}
	if sum == nil {
		t.Fatal("expected non-nil summary")
	}
}

func TestGlobeServiceSubscribeUnsubscribe(t *testing.T) {
	s := NewGlobeService(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	defer s.Stop()

	ch, unsub := s.Subscribe()
	if ch == nil {
		t.Fatal("Subscribe returned nil channel")
	}
	unsub()

	// After unsubscribing the channel should be closed; reading either yields
	// the zero value with ok=false (closed) or hits the default (depending on
	// goroutine scheduling).
	select {
	case _, ok := <-ch:
		if ok {
			t.Log("cached snapshot drained before close")
		}
	case <-time.After(20 * time.Millisecond):
		// Acceptable: no further events.
	}
}

func TestMaskIP(t *testing.T) {
	cases := map[string]string{
		"47.82.86.196": "47.82.86.•••",
		"1.2.3.4":      "1.2.3.•••",
		"2001:db8::1":  "2001:db8::•••",
		"":             "•••",
		"junk":         "•••",
	}
	for in, want := range cases {
		got := maskIP(in)
		if got != want {
			t.Errorf("maskIP(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGlobeServiceBuildsSnapshotFromRecentUsage(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)FROM usage_logs ul\s+JOIN ip_geo_cache g ON g\.ip = ul\.ip_address.*ORDER BY MAX\(ul\.created_at\) DESC, calls DESC`).
		WithArgs(int64(snapshotInterval / time.Millisecond)).
		WillReturnRows(sqlmock.NewRows([]string{
			"ip_address", "country", "country_code", "region", "city", "lat", "lng", "calls",
		}).AddRow("203.0.113.7", "China", "CN", "Shanghai", "Shanghai", 31.2304, 121.4737, 3))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ip_geo_cache`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT COUNT\(DISTINCT ul\.ip_address\).*LEFT JOIN ip_geo_cache g ON g\.ip = ul\.ip_address`).
		WithArgs(int64(snapshotInterval / time.Millisecond)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	s := NewGlobeService(db)
	s.now = func() time.Time { return time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC) }

	snap, err := s.buildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("buildSnapshot returned error: %v", err)
	}
	if len(snap.Arcs) != 1 {
		t.Fatalf("expected one replay arc, got %d", len(snap.Arcs))
	}
	if snap.TotalCalls != 3 || snap.UniqueIPs != 1 {
		t.Fatalf("snapshot should count resolved geo-cache IPs: total=%d unique=%d", snap.TotalCalls, snap.UniqueIPs)
	}
	if snap.Arcs[0].Calls != 3 {
		t.Fatalf("expected recent usage count to be carried into arc, got %+v", snap.Arcs[0])
	}
	if snap.Arcs[0].City != "Shanghai" || snap.Arcs[0].CountryCode != "CN" {
		t.Fatalf("unexpected replay arc: %+v", snap.Arcs[0])
	}
	if snap.Arcs[0].Region != "Shanghai" {
		t.Fatalf("expected region to be carried into replay arc, got %+v", snap.Arcs[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGlobeServiceSnapshotDoesNotSerializeRawIP(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)FROM usage_logs ul\s+JOIN ip_geo_cache g ON g\.ip = ul\.ip_address.*ORDER BY MAX\(ul\.created_at\) DESC, calls DESC`).
		WithArgs(int64(snapshotInterval / time.Millisecond)).
		WillReturnRows(sqlmock.NewRows([]string{
			"ip_address", "country", "country_code", "region", "city", "lat", "lng", "calls",
		}).AddRow("198.51.100.9", "United States", "US", "California", "San Jose", 37.3382, -121.8863, 2))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ip_geo_cache`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT COUNT\(DISTINCT ul\.ip_address\).*LEFT JOIN ip_geo_cache g ON g\.ip = ul\.ip_address`).
		WithArgs(int64(snapshotInterval / time.Millisecond)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	s := NewGlobeService(db)

	snap, err := s.buildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("buildSnapshot returned error: %v", err)
	}
	if len(snap.Arcs) != 1 {
		t.Fatalf("expected one recent arc, got %d", len(snap.Arcs))
	}
	if snap.Arcs[0].CountryCode != "US" || snap.Arcs[0].City != "San Jose" {
		t.Fatalf("unexpected recent arc: %+v", snap.Arcs[0])
	}
	if snap.Arcs[0].Region != "California" {
		t.Fatalf("expected region to be carried into recent arc, got %+v", snap.Arcs[0])
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if strings.Contains(string(payload), "198.51.100.9") || strings.Contains(string(payload), `"ip"`) {
		t.Fatalf("public snapshot serialized raw IP: %s", payload)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGlobeServiceGeoBackfillDisabledByDefault(t *testing.T) {
	s := NewGlobeService(nil)
	if s.geoBackfillOn {
		t.Fatal("geo backfill should be disabled unless explicitly configured")
	}
	if s.geoLookupURL != "" {
		t.Fatalf("geo lookup URL should default empty, got %q", s.geoLookupURL)
	}
	if _, err := s.batchGeoLookup(context.Background(), []string{"203.0.113.7"}); err == nil {
		t.Fatal("expected lookup to fail when endpoint is not configured")
	}
}

func TestEmptySnapshotShape(t *testing.T) {
	snap := emptySnapshot()
	if snap.IntervalMs != int64(snapshotInterval/time.Millisecond) {
		t.Errorf("default interval expected %dms, got %d", int64(snapshotInterval/time.Millisecond), snap.IntervalMs)
	}
	if snap.ServerLocation == nil {
		t.Fatal("default snapshot must carry a server location")
	}
	if !strings.Contains(snap.ServerLocation.Label, "sub2api") {
		t.Errorf("server location label should mention sub2api, got %q", snap.ServerLocation.Label)
	}
}
