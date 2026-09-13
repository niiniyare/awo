package migration

import "fmt"

// VerifyChecksums checks that previously-applied migration scripts have not
// been modified since they were applied.
//
// appliedChecksums maps version → SHA-256 hex checksum as recorded in the
// schema_migrations table (or equivalent). steps is the full list of up-
// direction steps discovered by Scan.
//
// Returns the first mismatch found, or nil if all applied versions match.
func VerifyChecksums(steps []Step, appliedChecksums map[uint64]string) error {
	for _, s := range steps {
		if s.Direction != DirectionUp {
			continue
		}
		recorded, ok := appliedChecksums[s.Version]
		if !ok {
			// Not yet applied — nothing to verify.
			continue
		}
		if recorded != s.Checksum {
			return fmt.Errorf(
				"migration.VerifyChecksums: version %d (%s) checksum mismatch: recorded=%s current=%s — do not edit applied migrations",
				s.Version, s.Description, recorded, s.Checksum,
			)
		}
	}
	return nil
}
