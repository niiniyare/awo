package module

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion_Valid(t *testing.T) {
	cases := []struct {
		s    string
		want Version
	}{
		{"1.2.3", Version{1, 2, 3}},
		{"0.0.0", Version{0, 0, 0}},
		{"10.20.30", Version{10, 20, 30}},
	}
	for _, tc := range cases {
		v, err := ParseVersion(tc.s)
		require.NoError(t, err, "ParseVersion(%q)", tc.s)
		assert.Equal(t, tc.want, v)
	}
}

func TestParseVersion_Invalid(t *testing.T) {
	bad := []string{"1.2", "1.2.x", "1.2.3.4", "", "v1.2.3", "-1.0.0"}
	for _, s := range bad {
		_, err := ParseVersion(s)
		assert.Error(t, err, "ParseVersion(%q) should fail", s)
	}
}

func TestVersion_String(t *testing.T) {
	assert.Equal(t, "1.2.3", Version{1, 2, 3}.String())
}

func TestVersion_Less(t *testing.T) {
	cases := []struct {
		a, b Version
		want bool
	}{
		{Version{1, 0, 0}, Version{2, 0, 0}, true},
		{Version{1, 1, 0}, Version{1, 2, 0}, true},
		{Version{1, 1, 1}, Version{1, 1, 2}, true},
		{Version{1, 0, 0}, Version{1, 0, 0}, false},
		{Version{2, 0, 0}, Version{1, 9, 9}, false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.a.Less(tc.b), "%v.Less(%v)", tc.a, tc.b)
	}
}

func TestVersion_Compatible(t *testing.T) {
	base := Version{1, 2, 0}
	cases := []struct {
		other Version
		want  bool
	}{
		{Version{1, 2, 0}, true},
		{Version{1, 3, 0}, true},
		{Version{1, 2, 1}, true},
		{Version{1, 1, 9}, false},
		{Version{2, 2, 0}, false},
		{Version{0, 9, 9}, false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, base.Compatible(tc.other), "%v.Compatible(%v)", base, tc.other)
	}
}
