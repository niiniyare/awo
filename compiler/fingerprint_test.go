package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"awo.so/awo/def"
)

func TestFingerprint_Deterministic(t *testing.T) {
	defs := []def.EntityDefinition{
		simpleDef("mod_a", "mod"),
		simpleDef("mod_b", "mod"),
	}
	s1 := mustCompile(defs)
	s2 := mustCompile(defs)
	assert.Equal(t, Fingerprint(s1), Fingerprint(s2), "Fingerprint must be deterministic")
}

func TestFingerprint_ChangesOnFieldAdd(t *testing.T) {
	base := mustCompile([]def.EntityDefinition{simpleDef("mod_a", "mod")})

	withField := mustCompile([]def.EntityDefinition{
		&def.SystemDefinition{
			Name:        "mod_a",
			Module:      "mod",
			Label:       "mod_a",
			LabelPlural: "mod_as",
			Fields: []def.FieldDef{
				{Name: "title", Type: def.FieldTypeData},
			},
		},
	})

	assert.NotEqual(t, Fingerprint(base), Fingerprint(withField), "Fingerprint must change when a field is added")
}

func TestFingerprint_StableAcrossRegistrationOrder(t *testing.T) {
	a := simpleDef("mod_a", "mod")
	b := simpleDef("mod_b", "mod")

	s1 := mustCompile([]def.EntityDefinition{a, b})
	s2 := mustCompile([]def.EntityDefinition{b, a})

	assert.Equal(t, Fingerprint(s1), Fingerprint(s2), "Fingerprint must be stable regardless of registration order")
}

func TestFingerprint_NonEmpty(t *testing.T) {
	s := mustCompile([]def.EntityDefinition{simpleDef("mod_x", "mod")})
	f := Fingerprint(s)
	assert.NotEmpty(t, f)
	assert.Len(t, f, 64, "SHA-256 hex fingerprint must be 64 chars")
}
