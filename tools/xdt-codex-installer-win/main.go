package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const officialAPIURL = "https://xuedingtoken1.com/v1"
const currentOfficialBaseURL = "https://xuedingtoken1.com"
const legacyOfficialBaseURL = "https://xuedingtoken.com"

var embeddedScriptB64 string
var payloadSHA256 string

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[XueDingToken] %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	token := firstNonEmpty(os.Getenv("XDT_TOKEN"), os.Getenv("CODEX_TOKEN"), os.Getenv("OPENAI_API_KEY"))
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("missing CODEX_TOKEN")
	}

	if err := rejectCustomEndpoint(os.Getenv("XDT_API_URL")); err != nil {
		return err
	}
	if err := rejectCustomEndpoint(os.Getenv("CODEX_API_URL")); err != nil {
		return err
	}

	script, err := decodePayload()
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "xdt-codex-installer-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "install-codex-win.ps1")
	if err := os.WriteFile(scriptPath, script, 0600); err != nil {
		return fmt.Errorf("write embedded installer: %w", err)
	}

	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(filteredEnv(os.Environ()),
		"XDT_TOKEN="+token,
		"CODEX_TOKEN="+token,
		"XDT_API_URL="+officialAPIURL,
		"CODEX_API_URL="+officialAPIURL,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installer failed: %w", err)
	}
	return nil
}

func decodePayload() ([]byte, error) {
	if strings.TrimSpace(embeddedScriptB64) == "" {
		return nil, fmt.Errorf("installer payload is missing")
	}
	compressed, err := base64.StdEncoding.DecodeString(embeddedScriptB64)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("open payload: %w", err)
	}
	defer gz.Close()

	script, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}
	if payloadSHA256 != "" {
		sum := fmt.Sprintf("%x", sha256.Sum256(script))
		if !strings.EqualFold(sum, payloadSHA256) {
			return nil, fmt.Errorf("embedded installer checksum mismatch")
		}
	}
	return script, nil
}

func rejectCustomEndpoint(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	value = migrateOfficialBaseURL(strings.TrimRight(value, "/"))
	if value == currentOfficialBaseURL || value == officialAPIURL {
		return nil
	}
	return fmt.Errorf("this installer is locked to %s and does not support custom API endpoints", officialAPIURL)
}

func migrateOfficialBaseURL(value string) string {
	if value == legacyOfficialBaseURL || strings.HasPrefix(value, legacyOfficialBaseURL+"/") {
		return currentOfficialBaseURL + strings.TrimPrefix(value, legacyOfficialBaseURL)
	}
	return value
}

func filteredEnv(input []string) []string {
	out := make([]string, 0, len(input))
	for _, item := range input {
		key := item
		if idx := strings.IndexByte(item, '='); idx >= 0 {
			key = item[:idx]
		}
		switch strings.ToUpper(key) {
		case "XDT_TOKEN", "CODEX_TOKEN", "OPENAI_API_KEY", "XDT_API_URL", "CODEX_API_URL":
			continue
		default:
			out = append(out, item)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
