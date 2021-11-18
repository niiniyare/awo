--
-- Name: airports_data; Type: TABLE; Schema: bookings; Owner: -
--



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



--
-- Name: TABLE airports_data; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON TABLE airports_data IS 'Airports (internal data)';


--
-- Name: COLUMN airports_data.airport_code; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports_data.airport_code IS 'Airport code';


--
-- Name: COLUMN airports_data.airport_name; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports_data.airport_name IS 'Airport name';


--
-- Name: COLUMN airports_data.city; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports_data.city IS 'City';

---- Name: COLUMN airports_data.country_code; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports_data.country_code IS 'Country';
  
--
-- Name: COLUMN airports_data.coordinates; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports_data.coordinates IS 'Airport coordinates (longitude and latitude)';


--
-- Name: COLUMN airports_data.timezone; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports_data.timezone IS 'Airport time zone';

-- Name: COLUMN airports_data.Created_at; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports_data.Created_at IS 'time airport record Created';


--
-- Name: airports; Type: VIEW; Schema: bookings; Owner: -
--

CREATE VIEW airports AS
 SELECT ml.airport_code,
    ml.airport_name AS airport_name,
    ml.country_code AS country ,
    ml.city AS city,
    ml.coordinates AS coordinates,
    ml.timezone AS timezone,
    ml.Created_at AS Created_at
   FROM airports_data ml;


--
-- Name: VIEW airports; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON VIEW airports IS 'Airports';


--
-- Name: COLUMN airports.airport_code; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports.airport_code IS 'Airport code';


--
-- Name: COLUMN airports.airport_name; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports.airport_name IS 'Airport name';


--
-- Name: COLUMN airports.city; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports.city IS 'City';


---- Name: COLUMN airports.country ; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports.country  IS 'Country';


-- Name: COLUMN airports.coordinates; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN airports.coordinates IS 'Airport coordinates (longitude and latitude)';
