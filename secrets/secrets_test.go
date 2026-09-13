package secrets_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/secrets"
)

func TestStaticProvider_Get(t *testing.T) {
	p := secrets.NewStaticProvider(map[string]string{"JWT_SECRET": "s3cr3t"})
	v, err := p.Get(context.Background(), "JWT_SECRET")
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t", v)
}

func TestStaticProvider_NotFound(t *testing.T) {
	p := secrets.NewStaticProvider(nil)
	_, err := p.Get(context.Background(), "MISSING")
	require.Error(t, err)
	assert.True(t, secrets.IsNotFound(err))
}

func TestStaticProvider_Set(t *testing.T) {
	p := secrets.NewStaticProvider(nil)
	p.Set("KEY", "value")
	v, err := p.Get(context.Background(), "KEY")
	require.NoError(t, err)
	assert.Equal(t, "value", v)
}

func TestEnvProvider_Get(t *testing.T) {
	t.Setenv("AWO_TEST_SECRET", "env-value")
	p := secrets.NewEnvProvider(secrets.WithPrefix("AWO_"))
	v, err := p.Get(context.Background(), "TEST_SECRET")
	require.NoError(t, err)
	assert.Equal(t, "env-value", v)
}

func TestEnvProvider_NotFound(t *testing.T) {
	p := secrets.NewEnvProvider()
	_, err := p.Get(context.Background(), "DEFINITELY_NOT_SET_XYZ")
	require.Error(t, err)
	assert.True(t, secrets.IsNotFound(err))
}

func TestEnvProvider_Uppercase(t *testing.T) {
	// WithUppercase(false) reads the key as-is, so env var must match exactly.
	t.Setenv("lowercase_key", "val")
	p := secrets.NewEnvProvider(secrets.WithUppercase(false))
	_, err := p.Get(context.Background(), "lowercase_key")
	require.NoError(t, err)
}

func TestRequireAll_AllPresent(t *testing.T) {
	p := secrets.NewStaticProvider(map[string]string{
		"A": "1", "B": "2", "C": "3",
	})
	m, err := secrets.RequireAll(context.Background(), p, "A", "B", "C")
	require.NoError(t, err)
	assert.Equal(t, "1", m["A"])
	assert.Equal(t, "2", m["B"])
	assert.Equal(t, "3", m["C"])
}

func TestRequireAll_SomeMissing(t *testing.T) {
	p := secrets.NewStaticProvider(map[string]string{"A": "1"})
	_, err := secrets.RequireAll(context.Background(), p, "A", "B", "C")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "B")
	assert.Contains(t, err.Error(), "C")
}

func TestMustGet_Panics(t *testing.T) {
	p := secrets.NewStaticProvider(nil)
	assert.Panics(t, func() {
		secrets.MustGet(context.Background(), p, "MISSING")
	})
}

func TestMustGet_OK(t *testing.T) {
	p := secrets.NewStaticProvider(map[string]string{"K": "v"})
	assert.NotPanics(t, func() {
		v := secrets.MustGet(context.Background(), p, "K")
		assert.Equal(t, "v", v)
	})
}

func TestErrNotFound_Message(t *testing.T) {
	p := secrets.NewStaticProvider(nil)
	_, err := p.Get(context.Background(), "MY_KEY")
	assert.Contains(t, err.Error(), "MY_KEY")
}
