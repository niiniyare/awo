
CREATE TABLE IF NOT EXISTS aircrafts_data (
    aircraft_code VARCHAR(3) NOT NULL,
    model jsonb NOT NULL,
    range integer NOT NULL,
    company_id BIGSERIAL NOT NULL,
    created_at timestamptz NOT NULL DEFAULT (now()),
    CONSTRAINT aircrafts_range_check CHECK ((range > 0)),
    CONSTRAINT aircrafts_pkey PRIMARY KEY (aircraft_code),
    CONSTRAINT airline_company_pk FOREIGN KEY (company_id) REFERENCES airline_company (company_id)
);


--
-- Name: TABLE aircrafts_data; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON TABLE aircrafts_data IS 'Aircrafts (internal data)';


--
-- Name: COLUMN aircrafts_data.aircraft_code; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN aircrafts_data.aircraft_code IS 'Aircraft code, IATA';


--
-- Name: COLUMN aircrafts_data.model; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN aircrafts_data.model IS 'Aircraft model';


--
-- Name: COLUMN aircrafts_data.range; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN aircrafts_data.range IS 'Maximal flying distance, km';


--
-- Name: aircrafts; Type: VIEW; Schema: bookings; Owner: -
--

CREATE VIEW aircrafts AS
 SELECT ml.aircraft_code,
    ml.model AS model,
    ml.range
   FROM aircrafts_data ml;


--
-- Name: VIEW aircrafts; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON VIEW aircrafts IS 'Aircrafts';


--
-- Name: COLUMN aircrafts.aircraft_code; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN aircrafts.aircraft_code IS 'Aircraft code, IATA';


--
-- Name: COLUMN aircrafts.model; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN aircrafts.model IS 'Aircraft model';


--
-- Name: COLUMN aircrafts.range; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN aircrafts.range IS 'Maximal flying distance, km';
