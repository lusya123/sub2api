package service

import (
	"context"
	"database/sql"
)

// ProvideGlobeService is the wire constructor that also starts the live
// snapshot + geo-backfill goroutines on application boot. The service
// degrades gracefully if `db` is nil — handlers will return empty payloads
// instead of erroring out.
func ProvideGlobeService(db *sql.DB) *GlobeService {
	svc := NewGlobeService(db)
	svc.Start(context.Background())
	return svc
}
