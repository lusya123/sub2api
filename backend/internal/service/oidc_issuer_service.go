package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrOIDCIssuerNotConfigured = infraerrors.ServiceUnavailable("OIDC_NOT_CONFIGURED", "oidc is not configured")
	ErrOIDCIssuerInvalidClient = infraerrors.Unauthorized("OIDC_INVALID_CLIENT", "invalid oidc client")
	ErrOIDCIssuerInvalidCode   = infraerrors.Unauthorized("OIDC_INVALID_CODE", "invalid or expired authorization code")
)

// OIDCIssuerService implements Sub2API as an OIDC Provider/Issuer.
type OIDCIssuerService struct {
	cfg        *config.Config
	auth       *AuthService
	userRepo   UserRepository
	privateKey *rsa.PrivateKey
	keyID      string
	mu         sync.Mutex
	codes      map[string]oidcAuthCode
}

type oidcAuthCode struct {
	UserID      int64
	ClientID    string
	RedirectURI string
	Scope       string
	Nonce       string
	ExpiresAt   time.Time
}

type OIDCIssuerTokenResult struct {
	AccessToken string
	IDToken     string
	ExpiresIn   int
	User        *User
}

func NewOIDCIssuerService(cfg *config.Config, auth *AuthService, userRepo UserRepository) (*OIDCIssuerService, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate oidc signing key: %w", err)
	}
	sum := sha256.Sum256(key.N.Bytes())
	return &OIDCIssuerService{
		cfg:        cfg,
		auth:       auth,
		userRepo:   userRepo,
		privateKey: key,
		keyID:      base64.RawURLEncoding.EncodeToString(sum[:8]),
		codes:      map[string]oidcAuthCode{},
	}, nil
}

func (s *OIDCIssuerService) Issuer() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	if issuer := strings.TrimRight(strings.TrimSpace(s.cfg.OIDCIssuer.Issuer), "/"); issuer != "" {
		return issuer
	}
	return strings.TrimRight(strings.TrimSpace(s.cfg.Server.FrontendURL), "/")
}

func (s *OIDCIssuerService) CookieName() string {
	if s == nil || s.cfg == nil || strings.TrimSpace(s.cfg.OIDCIssuer.CookieName) == "" {
		return "sub2api_access_token"
	}
	return strings.TrimSpace(s.cfg.OIDCIssuer.CookieName)
}

func (s *OIDCIssuerService) CookieDomain() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	return strings.TrimSpace(s.cfg.OIDCIssuer.CookieDomain)
}

func (s *OIDCIssuerService) ClientID() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	return strings.TrimSpace(s.cfg.OIDCIssuer.ClientID)
}

func (s *OIDCIssuerService) IsConfigured() bool {
	return s != nil && s.Issuer() != "" && s.ClientID() != "" && strings.TrimSpace(s.cfg.OIDCIssuer.ClientSecret) != "" && len(s.cfg.OIDCIssuer.RedirectURIs) > 0
}

func (s *OIDCIssuerService) ValidateClient(clientID, clientSecret string) error {
	if !s.IsConfigured() {
		return ErrOIDCIssuerNotConfigured
	}
	if clientID != s.ClientID() {
		return ErrOIDCIssuerInvalidClient
	}
	expected := strings.TrimSpace(s.cfg.OIDCIssuer.ClientSecret)
	if subtle.ConstantTimeCompare([]byte(clientSecret), []byte(expected)) != 1 {
		return ErrOIDCIssuerInvalidClient
	}
	return nil
}

func (s *OIDCIssuerService) ValidateRedirectURI(redirectURI string) bool {
	if s == nil || s.cfg == nil {
		return false
	}
	for _, item := range s.cfg.OIDCIssuer.RedirectURIs {
		if strings.TrimSpace(item) == redirectURI {
			return true
		}
	}
	return false
}

func (s *OIDCIssuerService) UserFromAccessToken(ctx context.Context, token string) (*User, error) {
	claims, err := s.auth.ValidateToken(token)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive() || resolvedTokenVersion(user) != claims.TokenVersion {
		return nil, ErrTokenRevoked
	}
	return user, nil
}

func (s *OIDCIssuerService) CreateCode(userID int64, clientID, redirectURI, scope, nonce string) (string, error) {
	if !s.IsConfigured() {
		return "", ErrOIDCIssuerNotConfigured
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	code := base64.RawURLEncoding.EncodeToString(bytes)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredCodesLocked()
	s.codes[code] = oidcAuthCode{
		UserID:      userID,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		Scope:       scope,
		Nonce:       nonce,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	return code, nil
}

func (s *OIDCIssuerService) ExchangeCode(ctx context.Context, code, clientID, clientSecret, redirectURI string) (*OIDCIssuerTokenResult, error) {
	if err := s.ValidateClient(clientID, clientSecret); err != nil {
		return nil, err
	}

	s.mu.Lock()
	authCode, ok := s.codes[code]
	if ok {
		delete(s.codes, code)
	}
	s.mu.Unlock()
	if !ok || time.Now().After(authCode.ExpiresAt) || authCode.ClientID != clientID || authCode.RedirectURI != redirectURI {
		return nil, ErrOIDCIssuerInvalidCode
	}

	user, err := s.userRepo.GetByID(ctx, authCode.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive() {
		return nil, ErrUserNotActive
	}

	accessToken, err := s.auth.GenerateToken(user)
	if err != nil {
		return nil, err
	}
	idToken, err := s.signIDToken(user, authCode)
	if err != nil {
		return nil, err
	}

	return &OIDCIssuerTokenResult{
		AccessToken: accessToken,
		IDToken:     idToken,
		ExpiresIn:   s.auth.GetAccessTokenExpiresIn(),
		User:        user,
	}, nil
}

func (s *OIDCIssuerService) JWK() map[string]any {
	n := base64.RawURLEncoding.EncodeToString(s.privateKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(s.privateKey.E)).Bytes())
	return map[string]any{
		"kty": "RSA",
		"use": "sig",
		"kid": s.keyID,
		"alg": "RS256",
		"n":   n,
		"e":   e,
	}
}

func (s *OIDCIssuerService) signIDToken(user *User, authCode oidcAuthCode) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":                s.Issuer(),
		"sub":                fmt.Sprintf("%d", user.ID),
		"aud":                authCode.ClientID,
		"exp":                now.Add(time.Hour).Unix(),
		"iat":                now.Unix(),
		"auth_time":          now.Unix(),
		"email":              user.Email,
		"email_verified":     true,
		"name":               oidcUserDisplayName(user),
		"preferred_username": user.Username,
	}
	if authCode.Nonce != "" {
		claims["nonce"] = authCode.Nonce
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.keyID
	return token.SignedString(s.privateKey)
}

func (s *OIDCIssuerService) cleanupExpiredCodesLocked() {
	now := time.Now()
	for code, item := range s.codes {
		if now.After(item.ExpiresAt) {
			delete(s.codes, code)
		}
	}
}

func oidcUserDisplayName(user *User) string {
	if user == nil {
		return ""
	}
	if strings.TrimSpace(user.Username) != "" {
		return user.Username
	}
	if at := strings.IndexByte(user.Email, '@'); at > 0 {
		return user.Email[:at]
	}
	return user.Email
}

func BuildRedirectWithParams(rawURL string, values url.Values) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for key, vals := range values {
		for _, val := range vals {
			q.Add(key, val)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func IsOIDCIssuerInvalidAuth(err error) bool {
	return errors.Is(err, ErrOIDCIssuerInvalidClient) || errors.Is(err, ErrOIDCIssuerInvalidCode)
}
