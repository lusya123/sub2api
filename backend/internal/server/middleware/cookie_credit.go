package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	CreditChangeNormalRequest = 1
	CreditChangeAttackPayload = -50
	CreditChangeScannerPath   = -30
	CreditChangeRapidUse      = -20
	CreditChangePoWBypass     = -100
	CreditChangeHoneypot      = -100
	CreditChangeCookieAbuse   = -50

	CreditDefault    = 50
	CreditMax        = 100
	CreditMinTrust   = 30
	CreditMinService = 10
)

type CookieCreditSystem struct {
	rdb *redis.Client
}

func NewCookieCreditSystem(rdb *redis.Client) *CookieCreditSystem {
	return &CookieCreditSystem{rdb: rdb}
}

func (c *CookieCreditSystem) GetCredit(ctx context.Context, cookieHash string) int {
	if c == nil || c.rdb == nil || strings.TrimSpace(cookieHash) == "" {
		return CreditDefault
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := "credit:" + cookieHash
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		_ = c.rdb.Set(ctx, key, CreditDefault, 24*time.Hour).Err()
		return CreditDefault
	}
	if err != nil {
		return CreditDefault
	}
	credit, err := strconv.Atoi(val)
	if err != nil {
		return CreditDefault
	}
	return credit
}

func (c *CookieCreditSystem) ChangeCredit(ctx context.Context, cookieHash string, delta int, reason string) int {
	if c == nil || c.rdb == nil || strings.TrimSpace(cookieHash) == "" {
		return CreditDefault
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := "credit:" + cookieHash
	script := `
		local current = tonumber(redis.call('GET', KEYS[1]) or 50)
		local new = current + tonumber(ARGV[1])
		if new > 100 then new = 100 end
		if new < 0 then new = 0 end
		redis.call('SET', KEYS[1], new, 'EX', 86400)
		return new
	`
	result, err := c.rdb.Eval(ctx, script, []string{key}, delta).Result()
	if err != nil {
		return CreditDefault
	}
	newCredit, _ := result.(int64)

	logKey := "credit:log:" + cookieHash
	logEntry := time.Now().Format(time.RFC3339) + " " + reason + " delta=" + strconv.Itoa(delta) + " new=" + strconv.FormatInt(newCredit, 10)
	_ = c.rdb.RPush(ctx, logKey, logEntry).Err()
	_ = c.rdb.LTrim(ctx, logKey, -50, -1).Err()
	_ = c.rdb.Expire(ctx, logKey, 7*24*time.Hour).Err()

	return int(newCredit)
}

func (c *CookieCreditSystem) Reset(ctx context.Context, cookieHash string) {
	if c == nil || c.rdb == nil || strings.TrimSpace(cookieHash) == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_ = c.rdb.Set(ctx, "credit:"+cookieHash, CreditDefault, 24*time.Hour).Err()
}

func (c *CookieCreditSystem) StartAutoRecover() {
	if c == nil || c.rdb == nil || os.Getenv("DEFENSE_CREDIT_ENABLED") == "false" {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			c.recoverCredits()
		}
	}()
}

func (c *CookieCreditSystem) recoverCredits() {
	if c == nil || c.rdb == nil {
		return
	}
	ctx := context.Background()
	recoverBy := defenseEnvInt("DEFENSE_CREDIT_AUTO_RECOVER_PER_HOUR", 5)
	if recoverBy <= 0 {
		return
	}
	iter := c.rdb.Scan(ctx, 0, "credit:*", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if strings.HasPrefix(key, "credit:log:") {
			continue
		}
		val, err := c.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		credit, err := strconv.Atoi(val)
		if err != nil || credit >= CreditDefault {
			continue
		}
		newCredit := credit + recoverBy
		if newCredit > CreditDefault {
			newCredit = CreditDefault
		}
		_ = c.rdb.Set(ctx, key, newCredit, 24*time.Hour).Err()
	}
}

func CookieHash(cookieValue string) string {
	h := sha256.Sum256([]byte(cookieValue))
	return hex.EncodeToString(h[:8])
}

func (c *CookieCreditSystem) IsBlocked(ctx context.Context, cookieHash string) bool {
	if os.Getenv("DEFENSE_CREDIT_ENABLED") == "false" {
		return false
	}
	return c.GetCredit(ctx, cookieHash) < CreditMinService
}

func (c *CookieCreditSystem) IsRateLimited(ctx context.Context, cookieHash string) bool {
	if os.Getenv("DEFENSE_CREDIT_ENABLED") == "false" {
		return false
	}
	credit := c.GetCredit(ctx, cookieHash)
	return credit < CreditMinTrust && credit >= CreditMinService
}
