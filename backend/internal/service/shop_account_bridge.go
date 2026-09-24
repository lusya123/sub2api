package service

import (
	"context"
	"errors"
	"strings"
)

// LookupShopAccountBridgeUser returns only the authoritative identity row used
// by the scoped Shop bridge. It does not create a session or touch login state.
func (s *AuthService) LookupShopAccountBridgeUser(ctx context.Context, email string) (*User, error) {
	if s == nil || s.userRepo == nil {
		return nil, ErrServiceUnavailable
	}
	user, err := s.userRepo.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, ErrServiceUnavailable
	}
	// The Shop bridge is an ordinary-customer identity surface. Never expose,
	// verify, or bind an administrator/operator account through it: doing so
	// could let an unrelated Shop credential become an authentication alias for
	// a privileged Main account during or after account unification.
	if user == nil || user.Role != RoleUser {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// VerifyShopAccountBridgePassword validates both authoritative password
// verifiers without issuing tokens, creating a session, or recording a login.
// A correct password may return a disabled or TOTP-enabled user so the Shop can
// preserve Main's status and second-factor policy while still failing closed.
func (s *AuthService) VerifyShopAccountBridgePassword(ctx context.Context, email, password string) (*User, error) {
	if password == "" {
		return nil, ErrInvalidCredentials
	}
	user, err := s.LookupShopAccountBridgeUser(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !user.CheckPassword(password) {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

// CreateShopAccountBridgeUser creates a zero-credit Main identity for a Shop
// customer. It performs no login and returns no token. Signup grants must not
// cross the Shop/Main financial boundary, even if Main's email signup defaults
// are changed after rollout.
func (s *AuthService) CreateShopAccountBridgeUser(ctx context.Context, email, password, username string) (*User, error) {
	if s == nil || s.userRepo == nil {
		return nil, ErrServiceUnavailable
	}
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.TrimSpace(username)
	if username == "" {
		username = email
	}
	if isReservedEmail(email) {
		return nil, ErrEmailReserved
	}

	exists, err := s.existsByEmailOrAlias(ctx, email)
	if err != nil {
		return nil, ErrServiceUnavailable
	}
	if exists {
		return nil, ErrEmailExists
	}

	hashedPassword, err := s.HashPassword(password)
	if err != nil {
		return nil, err
	}
	plan := s.resolveSignupGrantPlan(ctx, "email")
	plan.Balance = 0
	defaultRPMLimit := 0
	if s.settingService != nil {
		defaultRPMLimit = s.settingService.GetDefaultUserRPMLimit(ctx)
	}
	user := &User{
		Email:        email,
		Username:     username,
		PasswordHash: hashedPassword,
		Role:         RoleUser,
		Balance:      plan.Balance,
		Concurrency:  plan.Concurrency,
		RPMLimit:     defaultRPMLimit,
		Status:       StatusActive,
	}
	if err := s.userRepo.CreateWithEmailAliasGuard(ctx, user); err != nil {
		if errors.Is(err, ErrEmailExists) {
			return nil, ErrEmailExists
		}
		return nil, ErrServiceUnavailable
	}

	s.postAuthUserBootstrap(ctx, user, "email", false)
	return user, nil
}
