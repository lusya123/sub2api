package handler

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"
)

func TestModelMarketplaceUserListGzipHelpers(t *testing.T) {
	raw := []byte(`{"code":0,"message":"success","data":{"items":[{"name":"Codex Extreme"}]}}`)
	gz := gzipBytes(raw)
	if len(gz) == 0 {
		t.Fatal("expected gzip payload")
	}

	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatalf("new gzip reader: %v", err)
	}
	decoded, err := io.ReadAll(zr)
	_ = zr.Close()
	if err != nil {
		t.Fatalf("read gzip payload: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatalf("decoded payload mismatch: got %q want %q", decoded, raw)
	}

	for _, header := range []string{"gzip", "br, gzip", "gzip; q=1.0", "gzip; q=0.5"} {
		if !clientAcceptsGzip(header) {
			t.Fatalf("expected %q to accept gzip", header)
		}
	}
	for _, header := range []string{"", "br", "xgzip", "gzip; q=0"} {
		if clientAcceptsGzip(header) {
			t.Fatalf("expected %q not to accept gzip", header)
		}
	}
}
