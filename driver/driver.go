// Package driver defines the storage interface contracts that all persistence
// backends must satisfy. Application code programs against these interfaces;
// no concrete database types leak into the domain or API layers.
package driver

import "awo.so/awo/def"

// RecordRepository is the concrete specialisation of EntityRepository used
// throughout the framework when the record type is *def.EntityRecord.
type RecordRepository = EntityRepository[*def.EntityRecord]
