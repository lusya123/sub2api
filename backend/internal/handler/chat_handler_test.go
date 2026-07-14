package handler

import (
	"context"
	"net/url"
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

func TestChatSignInURLWithPreferenceUsesTopLevelCallback(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.Lobe.ChatURL = "https://chat.example.com"

	got, err := h.chatSignInURLWithPreference(context.Background(), &chatLaunchPreference{
		Provider: "sub2api-group-7",
		Model:    "gpt-5.4-mini",
	})
	if err != nil {
		t.Fatalf("chatSignInURLWithPreference returned error: %v", err)
	}

	signInURL, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse sign-in URL: %v", err)
	}
	if embed := signInURL.Query().Get("embed"); embed != "" {
		t.Fatalf("sign-in URL unexpectedly enables embed mode: %q", embed)
	}

	callbackURL, err := url.Parse(signInURL.Query().Get("callbackUrl"))
	if err != nil {
		t.Fatalf("parse callback URL: %v", err)
	}
	if callbackURL.Path != "/agent/inbox" {
		t.Fatalf("callback path = %q, want %q", callbackURL.Path, "/agent/inbox")
	}
	if embed := callbackURL.Query().Get("embed"); embed != "" {
		t.Fatalf("callback URL unexpectedly enables embed mode: %q", embed)
	}
	if provider := callbackURL.Query().Get("provider"); provider != "sub2api-group-7" {
		t.Fatalf("callback provider = %q, want %q", provider, "sub2api-group-7")
	}
	if model := callbackURL.Query().Get("modelId"); model != "gpt-5.4-mini" {
		t.Fatalf("callback model = %q, want %q", model, "gpt-5.4-mini")
	}
}
