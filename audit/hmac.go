package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HashSessionToken derives a stable, non-reversible session identifier for
// storage in audit records. The raw session token is never stored.
//
// The result is HMAC-SHA256(token, secret) encoded as a lowercase hex string.
// Using HMAC (keyed hash) rather than a plain hash prevents offline
// pre-computation attacks against the session token.
//
// sessionToken — the raw session token value (never logged or stored).
// serverSecret — server-side secret (e.g. SESSION_HMAC_SECRET env var).
//
// Returns a 64-character hex string.
func HashSessionToken(sessionToken, serverSecret string) string {
	mac := hmac.New(sha256.New, []byte(serverSecret))
	mac.Write([]byte(sessionToken))
	return hex.EncodeToString(mac.Sum(nil))
}
