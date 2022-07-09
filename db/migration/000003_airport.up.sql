

CREATE TABLE airports (
  airport_code VARCHAR(3) NOT NULL,
  airport_name TEXT NOT NULL,
  country_code VARCHAR(3) NOT NULL,
  city TEXT NOT NULL,
  coordinates POINT NOT NULL,
  timezone TEXT NOT NULL,
  created_at timestamptz NOT NULL DEFAULT (now()),
  CONSTRAINT airports_pkey PRIMARY KEY (airport_code)
  );


 COMMENT  ON TABLE airports IS 'Airports (internal data)';
 COMMENT  ON COLUMN airports.airport_code IS 'Airport code';


 COMMENT  ON COLUMN airports.airport_name IS 'Airport name';

 COMMENT  ON COLUMN airports.city IS 'City';

 COMMENT  ON COLUMN airports.country_code IS 'Country';
  
COMMENT  ON COLUMN airports.coordinates IS 'Airport coordinates (longitude and latitude)';

COMMENT  ON COLUMN airports.timezone IS 'Airport time zone';
COMMENT  ON COLUMN airports.Created_at IS 'time airport record Created';
