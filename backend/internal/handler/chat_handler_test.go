package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestChatSignInURLUsesConfiguredChatURL(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.Lobe.ChatURL = "https://chat.example.com/"

	got, err := h.chatSignInURL(context.Background())
	if err != nil {
		t.Fatalf("chatSignInURL returned error: %v", err)
	}
	want := "https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox"
	if got != want {
		t.Fatalf("chatSignInURL = %q, want %q", got, want)
	}
}

func TestChatSignInURLUsesSettingsChatURLBeforeConfig(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.Lobe.ChatURL = "https://configured.example.com"

	got, err := h.chatSignInURL(context.Background(), "https://settings.example.com/")
	if err != nil {
		t.Fatalf("chatSignInURL returned error: %v", err)
	}
	want := "https://settings.example.com/signin?callbackUrl=%2Fagent%2Finbox"
	if got != want {
		t.Fatalf("chatSignInURL = %q, want %q", got, want)
	}
}

func TestChatSignInURLFallsBackToOIDCRedirectOrigin(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.OIDCIssuer.RedirectURIs = []string{"https://chat.example.com/api/auth/callback/generic-oidc"}

	got, err := h.chatSignInURL(context.Background())
	if err != nil {
		t.Fatalf("chatSignInURL returned error: %v", err)
	}
	want := "https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox"
	if got != want {
		t.Fatalf("chatSignInURL = %q, want %q", got, want)
	}
}

func TestChatSignInURLFallsBackToChatSubdomain(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.Server.FrontendURL = "https://app.example.com"

	got, err := h.chatSignInURL(context.Background())
	if err != nil {
		t.Fatalf("chatSignInURL returned error: %v", err)
	}
	want := "https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox"
	if got != want {
		t.Fatalf("chatSignInURL = %q, want %q", got, want)
	}
}
