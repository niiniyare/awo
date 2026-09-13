package crypto_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/crypto"
)

// --- Field encryption ---

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := crypto.DeriveKey("a-very-long-and-random-passphrase-for-testing")
	plain := []byte("sensitive field value")

	enc, err := crypto.Encrypt(key, plain, 1)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(enc, "v1:"))

	got, ver, err := crypto.Decrypt(key, enc)
	require.NoError(t, err)
	assert.Equal(t, plain, got)
	assert.Equal(t, 1, ver)
}

func TestEncrypt_DifferentNonceEachTime(t *testing.T) {
	key := crypto.DeriveKey("passphrase")
	plain := []byte("same plaintext")

	enc1, err := crypto.Encrypt(key, plain, 1)
	require.NoError(t, err)
	enc2, err := crypto.Encrypt(key, plain, 1)
	require.NoError(t, err)

	assert.NotEqual(t, enc1, enc2, "each encryption must use a unique nonce")
}

func TestEncrypt_BadKeyLength(t *testing.T) {
	_, err := crypto.Encrypt([]byte("short"), []byte("data"), 1)
	require.Error(t, err)
}

func TestDecrypt_BadKeyLength(t *testing.T) {
	_, _, err := crypto.Decrypt([]byte("short"), "v1:abc")
	require.Error(t, err)
}

func TestDecrypt_Tampered(t *testing.T) {
	key := crypto.DeriveKey("passphrase")
	enc, _ := crypto.Encrypt(key, []byte("data"), 1)
	// Flip last character to corrupt ciphertext.
	tampered := enc[:len(enc)-1] + "X"
	_, _, err := crypto.Decrypt(key, tampered)
	require.Error(t, err)
}

func TestDecrypt_InvalidFormat(t *testing.T) {
	key := crypto.DeriveKey("passphrase")
	_, _, err := crypto.Decrypt(key, "notanenvelope")
	require.Error(t, err)
}

func TestKeyVersion(t *testing.T) {
	key := crypto.DeriveKey("passphrase")
	enc, _ := crypto.Encrypt(key, []byte("data"), 3)
	ver, err := crypto.KeyVersion(enc)
	require.NoError(t, err)
	assert.Equal(t, 3, ver)
}

func TestKeyVersion_InvalidFormat(t *testing.T) {
	_, err := crypto.KeyVersion("invalid")
	require.Error(t, err)
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	key := crypto.DeriveKey("passphrase")
	enc, err := crypto.Encrypt(key, []byte{}, 1)
	require.NoError(t, err)
	got, _, err := crypto.Decrypt(key, enc)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// --- HMAC ---

func TestSign_Verify_RoundTrip(t *testing.T) {
	secret := []byte("hmac-secret")
	payload := []byte("payload data")

	sig := crypto.Sign(secret, payload)
	assert.True(t, crypto.Verify(secret, payload, sig))
}

func TestVerify_WrongPayload(t *testing.T) {
	secret := []byte("hmac-secret")
	sig := crypto.Sign(secret, []byte("original"))
	assert.False(t, crypto.Verify(secret, []byte("tampered"), sig))
}

func TestVerify_WrongSecret(t *testing.T) {
	sig := crypto.Sign([]byte("secret1"), []byte("data"))
	assert.False(t, crypto.Verify([]byte("secret2"), []byte("data"), sig))
}

func TestHash(t *testing.T) {
	h := crypto.Hash([]byte("hello"))
	assert.Len(t, h, 64)                             // SHA-256 = 32 bytes = 64 hex chars
	assert.Equal(t, h, crypto.Hash([]byte("hello"))) // deterministic
	assert.NotEqual(t, h, crypto.Hash([]byte("world")))
}

// --- Password hashing ---

func TestHashPassword_VerifyPassword(t *testing.T) {
	hash, err := crypto.HashPassword("correct-horse-battery-staple")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "correct-horse-battery-staple", hash) // must be hashed

	ok, err := crypto.VerifyPassword(hash, "correct-horse-battery-staple")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, _ := crypto.HashPassword("correct")
	ok, err := crypto.VerifyPassword(hash, "wrong")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	_, err := crypto.VerifyPassword("not-a-bcrypt-hash", "password")
	require.Error(t, err)
}

func TestDeriveKey_Is32Bytes(t *testing.T) {
	key := crypto.DeriveKey("any passphrase")
	assert.Len(t, key, 32)
}
