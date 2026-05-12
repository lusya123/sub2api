package security

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/cespare/xxhash/v2"

	entsql "entgo.io/ent/dialect/sql"
)

const defaultBloomFalsePositiveRate = 0.001

var (
	emailBloom   *emailBloomFilter
	emailBloomMu sync.RWMutex
)

type emailBloomFilter struct {
	bits []uint64
	m    uint64
	k    uint64
}

func newEmailBloomFilter(capacity int, falsePositiveRate float64) *emailBloomFilter {
	if capacity < 1 {
		capacity = 1
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = defaultBloomFalsePositiveRate
	}
	m := uint64(math.Ceil(-float64(capacity) * math.Log(falsePositiveRate) / math.Pow(math.Ln2, 2)))
	if m < 64 {
		m = 64
	}
	k := uint64(math.Round((float64(m) / float64(capacity)) * math.Ln2))
	if k < 1 {
		k = 1
	}
	return &emailBloomFilter{
		bits: make([]uint64, (m+63)/64),
		m:    m,
		k:    k,
	}
}

func (f *emailBloomFilter) add(value string) {
	value = normalizeEmail(value)
	if value == "" || f == nil {
		return
	}
	h1 := xxhash.Sum64String(value)
	h2 := xxhash.Sum64String("sub2api-login-defense:" + value)
	if h2 == 0 {
		h2 = 0x9e3779b97f4a7c15
	}
	for i := uint64(0); i < f.k; i++ {
		idx := (h1 + i*h2) % f.m
		f.bits[idx/64] |= uint64(1) << (idx % 64)
	}
}

func (f *emailBloomFilter) maybeContains(value string) bool {
	value = normalizeEmail(value)
	if value == "" || f == nil {
		return true
	}
	h1 := xxhash.Sum64String(value)
	h2 := xxhash.Sum64String("sub2api-login-defense:" + value)
	if h2 == 0 {
		h2 = 0x9e3779b97f4a7c15
	}
	for i := uint64(0); i < f.k; i++ {
		idx := (h1 + i*h2) % f.m
		if f.bits[idx/64]&(uint64(1)<<(idx%64)) == 0 {
			return false
		}
	}
	return true
}

func InitEmailBloom(ctx context.Context, entClient *dbent.Client, cfg config.LoginProtectionConfig) error {
	cfg = withLoginProtectionDefaults(cfg)
	if !cfg.Enabled || !cfg.BloomEnabled {
		return nil
	}
	db, ok := SQLDBFromEnt(entClient)
	if !ok {
		return fmt.Errorf("ent client does not expose sql db")
	}

	capacity := cfg.BloomCapacity
	if capacity < 1 {
		capacity = 1_000_000
	}
	bf := newEmailBloomFilter(capacity, defaultBloomFalsePositiveRate)

	rows, err := db.QueryContext(ctx, `
SELECT LOWER(TRIM(email)) AS email
FROM users
WHERE deleted_at IS NULL
  AND TRIM(COALESCE(email, '')) <> ''
UNION
SELECT LOWER(TRIM(provider_subject)) AS email
FROM auth_identities
WHERE provider_type = 'email'
  AND provider_key = 'email'
  AND TRIM(COALESCE(provider_subject, '')) <> ''
`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return err
		}
		bf.add(email)
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}

	emailBloomMu.Lock()
	emailBloom = bf
	emailBloomMu.Unlock()

	slog.Info("login email bloom initialized", "emails", count, "capacity", capacity)
	return nil
}

func BloomMaybeContains(email string) bool {
	emailBloomMu.RLock()
	defer emailBloomMu.RUnlock()
	bf := emailBloom
	if bf == nil {
		return true
	}
	return bf.maybeContains(email)
}

func BloomAdd(email string) {
	email = normalizeEmail(email)
	if email == "" {
		return
	}
	emailBloomMu.Lock()
	defer emailBloomMu.Unlock()
	if emailBloom != nil {
		emailBloom.add(email)
	}
}

func ConstantTimeReject() {
	time.Sleep(time.Millisecond * time.Duration(50+rand.Intn(150)))
}

func SQLDBFromEnt(entClient *dbent.Client) (*sql.DB, bool) {
	if entClient == nil {
		return nil, false
	}
	drv, ok := entClient.Driver().(*entsql.Driver)
	if !ok || drv == nil {
		return nil, false
	}
	return drv.DB(), true
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func NormalizeEmail(email string) string {
	return normalizeEmail(email)
}
