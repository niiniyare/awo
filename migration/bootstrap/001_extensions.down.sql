-- Rollback: Drop extensions installed by bootstrap 001.
-- WARNING: dropping pgcrypto or uuid-ossp will break any column using gen_random_uuid().
-- Only run this in development environments with no data.

DROP EXTENSION IF EXISTS "btree_gist";
DROP EXTENSION IF EXISTS "pgcrypto";
DROP EXTENSION IF EXISTS "uuid-ossp";
