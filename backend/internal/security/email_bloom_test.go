package security

import "testing"

func TestEmailBloomFailOpenBeforeInit(t *testing.T) {
	resetEmailBloomForTest(t)

	if !BloomMaybeContains("missing@example.com") {
		t.Fatal("uninitialized bloom must fail open")
	}
}

func TestEmailBloomAddAndNormalize(t *testing.T) {
	resetEmailBloomForTest(t)

	emailBloomMu.Lock()
	emailBloom = newEmailBloomFilter(1000, 0.000001)
	emailBloomMu.Unlock()

	BloomAdd(" User@Example.COM ")

	if !BloomMaybeContains("user@example.com") {
		t.Fatal("expected normalized email to be present")
	}
	if !BloomMaybeContains(" USER@example.com ") {
		t.Fatal("expected lookup to normalize whitespace and case")
	}

	misses := 0
	for _, email := range []string{
		"missing-1@example.com",
		"missing-2@example.com",
		"missing-3@example.com",
		"missing-4@example.com",
		"missing-5@example.com",
	} {
		if !BloomMaybeContains(email) {
			misses++
		}
	}
	if misses == 0 {
		t.Fatal("expected at least one unrelated email to miss the bloom filter")
	}
}

func resetEmailBloomForTest(t *testing.T) {
	t.Helper()
	emailBloomMu.Lock()
	old := emailBloom
	emailBloom = nil
	emailBloomMu.Unlock()
	t.Cleanup(func() {
		emailBloomMu.Lock()
		emailBloom = old
		emailBloomMu.Unlock()
	})
}
