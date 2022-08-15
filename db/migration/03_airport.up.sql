 /* CREATE EXTENSION IF NOT EXISTS postgis; */

CREATE TABLE IF NOT EXISTS airports (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    iata_code text NOT NULL,
    -- Check if it is a valid airport iata code, e.g. TLV, LAX, etc.
    CHECK (iata_code ~ '\A[A-Z]{3}\Z'),
    icao_code text NOT NULL,
    -- Check if it is a valid airport icao code, e.g. LLBG, KLAX, etc.
    CHECK (icao_code ~ '\A[A-Z]{4}\Z'),
    name text NOT NULL,
  --  subdivision_code text NOT NULL,
    -- Check if it is a valid ISO 3166-2 subdivision code, e.g. IL-M, US-CA, etc.
    --CHECK (subdivision_code ~ '\A[A-Z]{2}-[A-Z0-9]{1,3}\Z'),
    city text NOT NULL,
  -- we use below command if PostGIS is installed in our system 
    
  --  coordinates geography(point) NOT NULL,
  
  -- If PostGIS not available flowing row will work.
   -- coordinates point NOT NULL,
    UNIQUE (iata_code, icao_code),
    timezone text NOT NULL
);
