package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const publicLogoRoute = "/api/v1/settings/logo"

// PublicSiteLogoForClient returns a cache-friendly public URL when the
// configured logo is an inline data URI. Stored settings stay unchanged; only
// public responses avoid embedding large base64 images into HTML/JSON.
func PublicSiteLogoForClient(siteLogo string) string {
	trimmed := strings.TrimSpace(siteLogo)
	if trimmed == "" {
		return ""
	}
	if !IsInlineImageDataURL(trimmed) {
		return trimmed
	}
	return publicLogoRoute + "?v=" + PublicSiteLogoVersion(trimmed)
}

func PublicSiteLogoVersion(siteLogo string) string {
	sum := sha256.Sum256([]byte(siteLogo))
	return hex.EncodeToString(sum[:6])
}

func IsInlineImageDataURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	comma := strings.IndexByte(trimmed, ',')
	if comma <= len("data:") {
		return false
	}
	meta := strings.ToLower(trimmed[:comma])
	return strings.HasPrefix(meta, "data:image/") && strings.Contains(meta, ";base64")
}

func DecodeInlineImageDataURL(value string) ([]byte, string, bool) {
	trimmed := strings.TrimSpace(value)
	comma := strings.IndexByte(trimmed, ',')
	if comma <= len("data:") || !IsInlineImageDataURL(trimmed) {
		return nil, "", false
	}

	meta := trimmed[len("data:"):comma]
	mediaType := strings.TrimSpace(strings.SplitN(meta, ";", 2)[0])
	if !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return nil, "", false
	}

	payload := trimmed[comma+1:]
	if len(payload) > 4*1024*1024 {
		return nil, "", false
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(data) == 0 || len(data) > 3*1024*1024 {
		return nil, "", false
	}
	return data, mediaType, true
}
