package service

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID             int64
	Email          string
	Username       string
	Notes          string
	AvatarURL      string
	AvatarSource   string
	AvatarMIME     string
	AvatarByteSize int
	AvatarSHA256   string
	PasswordHash   string
	// LegacyShopPasswordHash is the optional bcrypt verifier imported from the
	// Shop account during account unification. New passwords replace both old
	// credentials, so every active password-setting path must clear this field.
	LegacyShopPasswordHash *string
	// CredentialVersion is trigger-managed and advances whenever either stored
	// password verifier, email identity, account status, or TOTP policy changes.
	// It is independent from the JWT fingerprint.
	CredentialVersion uint64
	Role              string
	Balance           float64
	FrozenBalance     float64
	Concurrency       int
	Status            string
	AllowedGroups     []int64
	TokenVersion      int64 // Incremented on password change to invalidate existing tokens
	// TokenVersionResolved indicates TokenVersion already contains the fingerprint-derived
	// value expected in JWT claims and refresh-token state.
	TokenVersionResolved bool
	SignupSource         string
	LastLoginAt          *time.Time
	LastActiveAt         *time.Time
	LastUsedAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            *time.Time // 非 nil 表示用户已软删除

	// GroupRates 用户专属分组倍率配置
	// map[groupID]rateMultiplier
	GroupRates map[int64]float64

	// TOTP 双因素认证字段
	TotpSecretEncrypted *string    // AES-256-GCM 加密的 TOTP 密钥
	TotpEnabled         bool       // 是否启用 TOTP
	TotpEnabledAt       *time.Time // TOTP 启用时间

	// 余额不足通知
	BalanceNotifyEnabled       bool
	BalanceNotifyThresholdType string // "fixed" (default) | "percentage"
	BalanceNotifyThreshold     *float64
	BalanceNotifyExtraEmails   []NotifyEmailEntry
	TotalRecharged             float64

	// RPMLimit 用户级每分钟请求数上限（0 = 不限制）。仅在所用分组未设置 rpm_limit
	// 且该 (用户, 分组) 无 rpm_override 时作为全局兜底生效，计数键 rpm:u:{userID}:{min}。
	RPMLimit int

	// UserGroupRPMOverride 来自 auth cache snapshot 的 (user, group) RPM 覆盖值。
	// nil = 该 API Key 对应的 (user, group) 无 override；非 nil 时 checkRPM 直接使用，
	// 避免每请求查 DB。字段不持久化到数据库。
	UserGroupRPMOverride *int

	APIKeys       []APIKey
	Subscriptions []UserSubscription
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u *User) IsOperator() bool {
	return u.Role == RoleOperator
}

func (u *User) CanAccessAdmin() bool {
	return CanAccessAdminRole(u.Role)
}

func IsSuperAdminRole(role string) bool {
	return role == RoleAdmin
}

func IsOperatorRole(role string) bool {
	return role == RoleOperator
}

func CanAccessAdminRole(role string) bool {
	return role == RoleAdmin || role == RoleOperator
}

func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// CanBindGroup checks whether a user can bind to a given group.
// For standard groups:
// - Public groups (non-exclusive): all users can bind
// - Exclusive groups: only users with the group in AllowedGroups can bind
func (u *User) CanBindGroup(groupID int64, isExclusive bool) bool {
	// 公开分组（非专属）：所有用户都可以绑定
	if !isExclusive {
		return true
	}
	// 专属分组：需要在 AllowedGroups 中
	for _, id := range u.AllowedGroups {
		if id == groupID {
			return true
		}
	}
	return false
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	u.LegacyShopPasswordHash = nil
	return nil
}

func (u *User) CheckPassword(password string) bool {
	return u.checkPasswordWithComparator(password, bcrypt.CompareHashAndPassword)
}

func (u *User) checkPasswordWithComparator(password string, compare func([]byte, []byte) error) bool {
	if u == nil {
		return false
	}
	primaryOK := false
	if u.PasswordHash != "" {
		primaryOK = compare([]byte(u.PasswordHash), []byte(password)) == nil
	}
	// The migration-era Shop verifier is an ordinary-customer compatibility
	// credential, never an administrator/operator credential.  The planner and
	// apply bundle already prohibit writing it to privileged rows; keep this
	// runtime guard as an independent last line of defense if a row is ever
	// corrupted or changed outside the reviewed migration path.
	if u.Role != RoleUser {
		return primaryOK
	}
	if u.LegacyShopPasswordHash == nil || *u.LegacyShopPasswordHash == "" || *u.LegacyShopPasswordHash == u.PasswordHash {
		return primaryOK
	}
	// Always evaluate the distinct legacy verifier, even when the primary one
	// matched, so callers cannot infer which stored credential accepted the
	// password from a one-vs-two-bcrypt timing difference.
	legacyOK := compare([]byte(*u.LegacyShopPasswordHash), []byte(password)) == nil
	return primaryOK || legacyOK
}

// checkPrimaryPassword is intentionally limited to admin password-replacement
// decisions. Login authentication must use CheckPassword so a distinct legacy
// verifier is evaluated without a source-dependent timing shortcut.
func (u *User) checkPrimaryPassword(password string) bool {
	if u == nil || u.PasswordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}
