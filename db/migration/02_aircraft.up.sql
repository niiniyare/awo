
CREATE TABLE IF NOT EXISTS aircrafts (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    code VARCHAR(3)  NOT NULL,
    model VARCHAR(50) NOT NULL,
    range integer NOT NULL,
    company_id BIGSERIAL NOT NULL,
    created_at timestamptz NOT NULL DEFAULT (now()),
    CONSTRAINT aircrafts_range_check CHECK ((range > 0)),
   --CONSTRAINT aircrafts_pkey PRIMARY KEY (code),
    CONSTRAINT airlines_pk FOREIGN KEY (company_id) REFERENCES airlines (id)
);


COMMENT  ON TABLE aircrafts IS 'Aircrafts (internal data)';


COMMENT ON COLUMN aircrafts.code IS 'Aircraft code, IATA';
COMMENT  ON COLUMN aircrafts.model IS 'Aircraft model';

COMMENT  ON COLUMN aircrafts.range IS 'Maximal flying distance, km';