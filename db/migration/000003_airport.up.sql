

CREATE TABLE airports_data (
  airport_code VARCHAR(3) NOT NULL,
  airport_name TEXT NOT NULL,
  country_code VARCHAR(3) NOT NULL,
  city TEXT NOT NULL,
  coordinates POINT NOT NULL,
  timezone TEXT NOT NULL,
  created_at timestamptz NOT NULL DEFAULT (now()),
  CONSTRAINT airports_data_pkey PRIMARY KEY (airport_code)
  );


 COMMENT  ON TABLE airports_data IS 'Airports (internal data)';
 COMMENT  ON COLUMN airports_data.airport_code IS 'Airport code';


 COMMENT  ON COLUMN airports_data.airport_name IS 'Airport name';

 COMMENT  ON COLUMN airports_data.city IS 'City';

 COMMENT  ON COLUMN airports_data.country_code IS 'Country';
  
COMMENT  ON COLUMN airports_data.coordinates IS 'Airport coordinates (longitude and latitude)';

COMMENT  ON COLUMN airports_data.timezone IS 'Airport time zone';
COMMENT  ON COLUMN airports_data.Created_at IS 'time airport record Created';
