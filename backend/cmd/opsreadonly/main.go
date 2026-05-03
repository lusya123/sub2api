package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
)

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type snapshotCacheEntry struct {
	expiresAt time.Time
	snapshot  *service.OperationAnalyticsSnapshot
}

func main() {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	if dsn == "" {
		dsn = buildDSNFromEnv()
	}
	addr := strings.TrimSpace(os.Getenv("OPS_READONLY_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:18081"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	analytics := service.NewOperationAnalyticsService(repository.NewOperationAnalyticsRepository(db))
	var cacheMu sync.Mutex
	snapshotCache := make(map[string]snapshotCacheEntry)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "ok", Data: map[string]string{"status": "ok"}})
	})
	mux.HandleFunc("/api/v1/admin/operations/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Code: 405, Message: "method not allowed"})
			return
		}
		filter, err := parseFilter(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 400, Message: err.Error()})
			return
		}
		cacheKey := r.URL.Query().Encode()
		cacheMu.Lock()
		if entry, ok := snapshotCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) && entry.snapshot != nil {
			cacheMu.Unlock()
			writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: entry.snapshot})
			return
		}
		cacheMu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		snapshot, err := analytics.GetSnapshot(ctx, filter)
		if err != nil {
			log.Printf("snapshot query failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiResponse{Code: 500, Message: "failed to query production operations snapshot"})
			return
		}
		cacheMu.Lock()
		snapshotCache[cacheKey] = snapshotCacheEntry{expiresAt: time.Now().Add(10 * time.Minute), snapshot: snapshot}
		cacheMu.Unlock()
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: snapshot})
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("readonly operations proxy listening on http://%s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func buildDSNFromEnv() string {
	host := envOr("DATABASE_HOST", "127.0.0.1")
	port := envOr("DATABASE_PORT", "15432")
	user := envOr("DATABASE_USER", "sub2api")
	password := os.Getenv("DATABASE_PASSWORD")
	dbName := envOr("DATABASE_DBNAME", "sub2api")
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + port,
		Path:   "/" + dbName,
	}
	q := u.Query()
	q.Set("sslmode", envOr("DATABASE_SSLMODE", "disable"))
	q.Set("options", "-c default_transaction_read_only=on -c statement_timeout=300000")
	u.RawQuery = q.Encode()
	return u.String()
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseFilter(r *http.Request) (service.OperationAnalyticsFilter, error) {
	q := r.URL.Query()
	tzName := strings.TrimSpace(q.Get("timezone"))
	if tzName == "" {
		tzName = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return service.OperationAnalyticsFilter{}, err
	}

	granularity := strings.TrimSpace(q.Get("granularity"))
	if granularity == "" {
		granularity = "day"
	}
	if granularity != "day" && granularity != "hour" {
		return service.OperationAnalyticsFilter{}, errText("invalid granularity, use day or hour")
	}

	now := time.Now().In(loc)
	startDate := strings.TrimSpace(q.Get("start_date"))
	endDate := strings.TrimSpace(q.Get("end_date"))
	rangeMode := strings.TrimSpace(q.Get("range"))
	modules, err := parseModules(strings.TrimSpace(q.Get("modules")))
	if err != nil {
		return service.OperationAnalyticsFilter{}, err
	}
	var start, end time.Time

	if rangeMode != "" && rangeMode != "all" {
		return service.OperationAnalyticsFilter{}, errText("invalid range, use all or omit it")
	}
	if rangeMode == "all" {
		start = time.Date(1970, 1, 1, 0, 0, 0, 0, loc)
		y, m, d := now.AddDate(0, 0, 1).Date()
		end = time.Date(y, m, d, 0, 0, 0, 0, loc)
		return service.OperationAnalyticsFilter{
			StartTime:   start,
			EndTime:     end,
			Granularity: "day",
			Timezone:    tzName,
			Modules:     restrictAllModules(modules),
			AllData:     true,
		}, nil
	}

	if startDate == "" {
		y, m, d := now.AddDate(0, 0, -13).Date()
		start = time.Date(y, m, d, 0, 0, 0, 0, loc)
	} else {
		start, err = time.ParseInLocation("2006-01-02", startDate, loc)
		if err != nil {
			return service.OperationAnalyticsFilter{}, errText("invalid start_date, use YYYY-MM-DD")
		}
	}
	if endDate == "" {
		y, m, d := now.AddDate(0, 0, 1).Date()
		end = time.Date(y, m, d, 0, 0, 0, 0, loc)
	} else {
		end, err = time.ParseInLocation("2006-01-02", endDate, loc)
		if err != nil {
			return service.OperationAnalyticsFilter{}, errText("invalid end_date, use YYYY-MM-DD")
		}
		end = end.Add(24 * time.Hour)
	}
	if !end.After(start) {
		return service.OperationAnalyticsFilter{}, errText("invalid date range")
	}
	if end.Sub(start) > 370*24*time.Hour {
		return service.OperationAnalyticsFilter{}, errText("date range is too large, maximum is 370 days")
	}
	return service.OperationAnalyticsFilter{
		StartTime:   start,
		EndTime:     end,
		Granularity: granularity,
		Timezone:    tzName,
		Modules:     modules,
	}, nil
}

func parseModules(raw string) ([]string, error) {
	if raw == "" || raw == "summary" {
		return []string{"core", "trend"}, nil
	}
	if raw == "all" {
		return []string{"core", "trend", "baselines", "funnel", "trial", "lists", "cohorts", "distribution", "churn", "pyramid", "financial", "product_matrix"}, nil
	}
	allowed := map[string]bool{
		"core": true, "trend": true, "baselines": true, "funnel": true, "trial": true, "lists": true,
		"cohorts": true, "distribution": true, "churn": true, "pyramid": true, "financial": true, "product_matrix": true,
	}
	seen := make(map[string]bool)
	modules := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		module := strings.TrimSpace(part)
		if module == "" {
			continue
		}
		if !allowed[module] {
			return nil, errText("invalid modules")
		}
		if !seen[module] {
			seen[module] = true
			modules = append(modules, module)
		}
	}
	if len(modules) == 0 {
		return []string{"core", "trend"}, nil
	}
	return modules, nil
}

func restrictAllModules(modules []string) []string {
	allowed := map[string]bool{"core": true}
	restricted := make([]string, 0, len(modules))
	for _, module := range modules {
		if allowed[module] {
			restricted = append(restricted, module)
		}
	}
	if len(restricted) == 0 {
		return []string{"core"}
	}
	return restricted
}

type errText string

func (e errText) Error() string { return string(e) }

func writeJSON(w http.ResponseWriter, status int, payload apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
