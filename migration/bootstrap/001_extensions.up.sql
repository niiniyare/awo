-- Bootstrap 001: PostgreSQL extensions required by the AWO framework.
-- These extensions must exist before any other DDL runs.
--
-- uuid-ossp   : gen_random_uuid() fallback (pgcrypto is preferred; both are installed)
-- pgcrypto    : gen_random_uuid(), pgp_sym_encrypt for future use
-- btree_gist  : GiST index support for exclusion constraints (e.g. non-overlapping periods)

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "btree_gist";
