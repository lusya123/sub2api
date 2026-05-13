package service

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestPublicSiteLogoForClientConvertsInlineImageToVersionedURL(t *testing.T) {
	raw := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png-bytes"))

	got := PublicSiteLogoForClient(raw)
	if !strings.HasPrefix(got, "/api/v1/settings/logo?v=") {
		t.Fatalf("expected versioned logo URL, got %q", got)
	}
	if strings.Contains(got, "base64") || strings.Contains(got, "png-bytes") {
		t.Fatalf("logo URL leaked inline payload: %q", got)
	}
}

func TestPublicSiteLogoForClientPreservesExternalOrEmptyLogo(t *testing.T) {
	if got := PublicSiteLogoForClient("https://cdn.example/logo.png"); got != "https://cdn.example/logo.png" {
		t.Fatalf("expected external logo to pass through, got %q", got)
	}
	if got := PublicSiteLogoForClient(""); got != "" {
		t.Fatalf("expected empty logo to pass through, got %q", got)
	}
}

func TestDecodeInlineImageDataURL(t *testing.T) {
	raw := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte("<svg></svg>"))

	data, contentType, ok := DecodeInlineImageDataURL(raw)
	if !ok {
		t.Fatal("expected inline image to decode")
	}
	if contentType != "image/svg+xml" {
		t.Fatalf("unexpected content type %q", contentType)
	}
	if string(data) != "<svg></svg>" {
		t.Fatalf("unexpected decoded data %q", string(data))
	}
}
