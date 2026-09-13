// Package crypto provides field-level encryption, HMAC signing, and
// password hashing helpers used throughout the Awo platform.
//
// # Field encryption
//
// Encrypt and Decrypt use AES-256-GCM authenticated encryption.
// Ciphertext is stored as "v{version}:{base64(nonce+ciphertext)}" so that
// key rotation can be detected and old versions re-encrypted.
//
//	enc, err := crypto.Encrypt(key, []byte("sensitive data"), 1)
//	plain, err := crypto.Decrypt(key, enc)
//
// # HMAC signing
//
// Sign produces a constant-time-comparable HMAC-SHA256 signature.
//
//	sig := crypto.Sign(secret, []byte("payload"))
//	ok  := crypto.Verify(secret, []byte("payload"), sig)
//
// # Password hashing
//
// HashPassword / VerifyPassword wrap bcrypt at cost 12.
//
//	hash, err := crypto.HashPassword("correct-horse-battery-staple")
//	ok, err   := crypto.VerifyPassword(hash, "correct-horse-battery-staple")
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// --- Field encryption ---

// Encrypt encrypts plaintext with AES-256-GCM using key.
// key must be exactly 32 bytes. keyVersion is embedded in the ciphertext
// envelope so callers can detect which key version was used.
//
// Output format: "v{version}:{base64(nonce||ciphertext)}"
func Encrypt(key []byte, plaintext []byte, keyVersion int) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("crypto.Encrypt: key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto.Encrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto.Encrypt: new GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto.Encrypt: read nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return fmt.Sprintf("v%d:%s", keyVersion, encoded), nil
}

// Decrypt decrypts a ciphertext produced by Encrypt.
// Returns the plaintext and the key version embedded in the envelope.
func Decrypt(key []byte, envelope string) (plaintext []byte, keyVersion int, err error) {
	if len(key) != 32 {
		return nil, 0, fmt.Errorf("crypto.Decrypt: key must be 32 bytes, got %d", len(key))
	}

	parts := strings.SplitN(envelope, ":", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "v") {
		return nil, 0, fmt.Errorf("crypto.Decrypt: invalid envelope format")
	}
	ver, err := strconv.Atoi(parts[0][1:])
	if err != nil {
		return nil, 0, fmt.Errorf("crypto.Decrypt: parse version: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, 0, fmt.Errorf("crypto.Decrypt: base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, 0, fmt.Errorf("crypto.Decrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, 0, fmt.Errorf("crypto.Decrypt: new GCM: %w", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, 0, fmt.Errorf("crypto.Decrypt: ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("crypto.Decrypt: decrypt: %w", err)
	}
	return plain, ver, nil
}

// KeyVersion extracts the key version from an encrypted envelope without
// decrypting it. Returns 0 and an error if the format is invalid.
func KeyVersion(envelope string) (int, error) {
	parts := strings.SplitN(envelope, ":", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "v") {
		return 0, fmt.Errorf("crypto.KeyVersion: invalid envelope format")
	}
	return strconv.Atoi(parts[0][1:])
}

// DeriveKey derives a 32-byte AES key from a passphrase using SHA-256.
// For production use, prefer a proper KDF (Argon2id, PBKDF2) with a salt.
// This is provided for environments where the passphrase is already high-entropy
// (e.g. a 256-bit secret from a secret manager).
func DeriveKey(passphrase string) []byte {
	h := sha256.Sum256([]byte(passphrase))
	return h[:]
}

// --- HMAC signing ---

// Sign produces an HMAC-SHA256 signature of payload using secret.
// The returned bytes are suitable for constant-time comparison.
func Sign(secret, payload []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return mac.Sum(nil)
}

// Verify reports whether sig is the correct HMAC-SHA256 of payload under secret.
// Uses constant-time comparison to prevent timing attacks.
func Verify(secret, payload, sig []byte) bool {
	expected := Sign(secret, payload)
	return hmac.Equal(expected, sig)
}

// Hash returns the SHA-256 hash of data as a hex string.
// Use for non-secret identifiers (e.g. cache keys, idempotency keys).
// Never use for passwords — use HashPassword instead.
func Hash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// --- Password hashing ---

// HashPassword hashes password using bcrypt at cost 12.
// Returns a string suitable for storage in the iam_user.password_hash column.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("crypto.HashPassword: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword reports whether password matches the stored bcrypt hash.
// Returns (false, nil) for wrong passwords. Returns (false, err) for
// malformed hashes.
func VerifyPassword(hash, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("crypto.VerifyPassword: %w", err)
	}
	return true, nil
}
