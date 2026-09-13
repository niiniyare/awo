package audit

import (
	"encoding/hex"
	"testing"
)

func TestHashSessionToken_IsHex(t *testing.T) {
	t.Parallel()

	h := HashSessionToken("token", "secret")
	if _, err := hex.DecodeString(h); err != nil {
		t.Errorf("HashSessionToken: output is not valid hex: %v", err)
	}
}

func TestHashSessionToken_EmptyInputs(t *testing.T) {
	t.Parallel()

	// Empty token or secret must not panic — returns a deterministic hash.
	h1 := HashSessionToken("", "secret")
	h2 := HashSessionToken("token", "")
	h3 := HashSessionToken("", "")

	if len(h1) != 64 {
		t.Errorf("empty token: expected 64 hex chars, got %d", len(h1))
	}
	if len(h2) != 64 {
		t.Errorf("empty secret: expected 64 hex chars, got %d", len(h2))
	}
	if len(h3) != 64 {
		t.Errorf("both empty: expected 64 hex chars, got %d", len(h3))
	}

	// All three must produce distinct hashes.
	if h1 == h2 || h1 == h3 || h2 == h3 {
		t.Error("different empty inputs must produce different hashes")
	}
}

func TestHashSessionToken_LongInputs(t *testing.T) {
	t.Parallel()

	longToken := make([]byte, 4096)
	for i := range longToken {
		longToken[i] = byte(i % 256)
	}
	h := HashSessionToken(string(longToken), "secret")
	if len(h) != 64 {
		t.Errorf("long token: expected 64 hex chars, got %d", len(h))
	}
}
