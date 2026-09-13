package metadata

import (
	"context"
	"regexp"

	"awo.so/awo/def"
	"awo.so/awo/runtime"
)

// fieldNameRe enforces the "cf_" prefix and snake_case convention for custom fields.
var fieldNameRe = regexp.MustCompile(`^cf_[a-z][a-z0-9_]*$`)

// FieldNameValidator rejects custom field names that don't start with "cf_".
type FieldNameValidator struct{}

func (v *FieldNameValidator) BeforeCreate(_ context.Context, rec *def.EntityRecord) error {
	name := rec.GetString("field_name")
	if !fieldNameRe.MatchString(name) {
		return &runtime.ValidationError{
			Fields: map[string]string{
				"field_name": `must start with "cf_" and contain only lowercase letters, digits, and underscores`,
			},
		}
	}
	return nil
}
