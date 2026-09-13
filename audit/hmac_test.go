package audit

import (
	"testing"
)

func TestHashSessionToken_Deterministic(t *testing.T) {
	t.Parallel()

	a := HashSessionToken("my-session-token", "signing-secret")
	b := HashSessionToken("my-session-token", "signing-secret")
	if a != b {
		t.Errorf("HashSessionToken not deterministic: %q != %q", a, b)
	}
}

func TestHashSessionToken_Length(t *testing.T) {
	t.Parallel()

	h := HashSessionToken("token", "secret")
	// HMAC-SHA256 = 32 bytes = 64 hex chars.
	if len(h) != 64 {
		t.Errorf("HashSessionToken: expected 64 hex chars, got %d", len(h))
	}
}

func TestHashSessionToken_DifferentTokens(t *testing.T) {
	t.Parallel()

	a := HashSessionToken("token-A", "secret")
	b := HashSessionToken("token-B", "secret")
	if a == b {
		t.Error("HashSessionToken: different tokens produced the same digest")
	}
}

func TestHashSessionToken_DifferentSecrets(t *testing.T) {
	t.Parallel()

	a := HashSessionToken("token", "secret-1")
	b := HashSessionToken("token", "secret-2")
	if a == b {
		t.Error("HashSessionToken: different secrets produced the same digest")
	}
}

func TestHashSessionToken_NeverRawToken(t *testing.T) {
	t.Parallel()

	raw := "super-secret-session-token"
	got := HashSessionToken(raw, "signing-key")
	if got == raw {
		t.Error("HashSessionToken must never return the raw token")
	}
}
