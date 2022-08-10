CREATE TABLE IF NOT EXISTS flights (
    flight_id integer NOT NULL,
    flight_no VARCHAR(6) NOT NULL,
    company_id BIGSERIAL NOT NULL,
    scheduled_departure timestamp with time zone NOT NULL,
    scheduled_arrival timestamp with time zone NOT NULL,
    departure_airport VARCHAR(3) NOT NULL,
    arrival_airport VARCHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL,
    aircraft_id BIGSERIAL NOT NULL,
    actual_departure timestamp with time zone,
    actual_arrival timestamp with time zone,
    
    CONSTRAINT flights_check CHECK ((scheduled_arrival > scheduled_departure)),
    
    CONSTRAINT flights_flight_no_scheduled_departure_key UNIQUE (flight_no, scheduled_departure),
    
    CONSTRAINT flights_check_airlines_key UNIQUE (flight_id, company_id),
    
    CONSTRAINT flights_check1 CHECK (((actual_arrival IS NULL) OR ((actual_departure IS NOT NULL) AND (actual_arrival IS NOT NULL) AND (actual_arrival > actual_departure)))),
    
    CONSTRAINT flights_status_check CHECK (((status)::text = ANY (ARRAY[('On Time'::VARCHAR)::text, ('Delayed'::VARCHAR)::text, ('Departed'::VARCHAR)::text, ('Arrived'::VARCHAR)::text, ('Scheduled'::VARCHAR)::text, ('Cancelled'::VARCHAR)::text])))
);


CREATE SEQUENCE IF NOT EXISTS flights_flight_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE flights_flight_id_seq OWNED BY flights.flight_id;


CREATE VIEW flights_v AS
 SELECT f.flight_id,
    f.flight_no,
    f.company_id,
    f.scheduled_departure,
    timezone(dep.timezone, f.scheduled_departure) AS scheduled_departure_local,
    f.scheduled_arrival,
    timezone(arr.timezone, f.scheduled_arrival) AS scheduled_arrival_local,
    (f.scheduled_arrival - f.scheduled_departure) AS scheduled_duration,
    f.departure_airport,
    dep.name AS departure_airport_name,
    dep.city AS departure_city,
    f.arrival_airport,
    arr.name AS arrival_airport_name,
    arr.city AS arrival_city,
    f.status,
    f.aircraft_id,
    f.actual_departure,
    timezone(dep.timezone, f.actual_departure) AS actual_departure_local,
    f.actual_arrival,
    timezone(arr.timezone, f.actual_arrival) AS actual_arrival_local,
    (f.actual_arrival - f.actual_departure) AS actual_duration
   FROM flights f,
    airports dep, 
    airports arr
  WHERE ((f.departure_airport = dep.iata_code) AND (f.arrival_airport = arr.iata_code));


--
-- Name: VIEW flights_v; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON VIEW flights_v IS 'Flights (extended)';


--
-- Name: COLUMN flights_v.flight_id; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.flight_id IS 'Flight ID';


--
-- Name: COLUMN flights_v.flight_no; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.flight_no IS 'Flight number';


--
-- Name: COLUMN flights_v.scheduled_departure; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.scheduled_departure IS 'Scheduled departure time';


--
-- Name: COLUMN flights_v.scheduled_departure_local; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.scheduled_departure_local IS 'Scheduled departure time, local time at the point of departure';


--
-- Name: COLUMN flights_v.scheduled_arrival; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.scheduled_arrival IS 'Scheduled arrival time';



  COMMENT  ON COLUMN flights_v.scheduled_arrival_local IS 'Scheduled arrival time, local time at the point of destination';

  COMMENT  ON COLUMN flights_v.scheduled_duration IS 'Scheduled flight duration';


  COMMENT  ON COLUMN flights_v.departure_airport IS 'Deprature airport code';


  COMMENT  ON COLUMN flights_v.departure_airport_name IS 'Departure airport name';


--
-- Name: COLUMN flights_v.departure_city; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.departure_city IS 'City of departure';


--
-- Name: COLUMN flights_v.arrival_airport; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.arrival_airport IS 'Arrival airport code';


--
-- Name: COLUMN flights_v.arrival_airport_name; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.arrival_airport_name IS 'Arrival airport name';


--
-- Name: COLUMN flights_v.arrival_city; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.arrival_city IS 'City of arrival';


--
-- Name: COLUMN flights_v.status; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.status IS 'Flight status';


--
-- Name: COLUMN flights_v.aircraft_id; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.aircraft_id IS 'Aircraft code, IATA';


--
-- Name: COLUMN flights_v.actual_departure; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.actual_departure IS 'Actual departure time';


--
-- Name: COLUMN flights_v.actual_departure_local; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.actual_departure_local IS 'Actual departure time, local time at the point of departure';


--
-- Name: COLUMN flights_v.actual_arrival; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.actual_arrival IS 'Actual arrival time';


--
-- Name: COLUMN flights_v.actual_arrival_local; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.actual_arrival_local IS 'Actual arrival time, local time at the point of destination';


--
-- Name: COLUMN flights_v.actual_duration; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN flights_v.actual_duration IS 'Actual flight duration';


--
-- Name: routes; Type: VIEW; Schema: bookings; Owner: -
--

CREATE VIEW routes AS
 WITH f3 AS (
         SELECT f2.flight_no,
            f2.company_id,
            f2.departure_airport,
            f2.arrival_airport,
            f2.aircraft_id,
            f2.duration,
            array_agg(f2.days_of_week) AS days_of_week
           FROM ( SELECT f1.flight_no,
                    f1.company_id,
                    f1.departure_airport,
                    f1.arrival_airport,
                    f1.aircraft_id,
                    f1.duration,
                    f1.days_of_week
                   FROM ( SELECT flights.flight_no,
                   flights.company_id,
                            flights.departure_airport,
                            flights.arrival_airport,
                            flights.aircraft_id,
                            (flights.scheduled_arrival - flights.scheduled_departure) AS duration,
                            (to_char(flights.scheduled_departure, 'ID'::text))::integer AS days_of_week
                           FROM flights) f1
                  GROUP BY f1.flight_no, f1.company_id, f1.departure_airport, f1.arrival_airport, f1.aircraft_id, f1.duration, f1.days_of_week
                  ORDER BY f1.flight_no, f1.company_id, f1.departure_airport, f1.arrival_airport, f1.aircraft_id, f1.duration, f1.days_of_week) f2
          GROUP BY f2.flight_no,
          f2.company_id, f2.departure_airport, f2.arrival_airport, f2.aircraft_id, f2.duration
        )
 SELECT f3.flight_no,
    f3.company_id,
    f3.departure_airport,
    dep.name AS departure_airport_name,
    dep.city AS departure_city,
    f3.arrival_airport,
    arr.name AS arrival_airport_name,
    arr.city AS arrival_city,
    f3.aircraft_id,
    f3.duration,
    f3.days_of_week
   FROM f3,
    airports dep,
    airports arr
  WHERE ((f3.departure_airport = dep.iata_code) AND (f3.arrival_airport = arr.iata_code));


--
-- Name: VIEW routes; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON VIEW routes IS 'Routes';


--
-- Name: COLUMN routes.flight_no; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.flight_no IS 'Flight number';


--
-- Name: COLUMN routes.departure_airport; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.departure_airport IS 'Code of airport of departure';


--
-- Name: COLUMN routes.departure_airport_name; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.departure_airport_name IS 'Name of airport of departure';


--
-- Name: COLUMN routes.departure_city; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.departure_city IS 'City of departure';


--
-- Name: COLUMN routes.arrival_airport; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.arrival_airport IS 'Code of airport of arrival';


--
-- Name: COLUMN routes.arrival_airport_name; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.arrival_airport_name IS 'Name of airport of arrival';


--
-- Name: COLUMN routes.arrival_city; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.arrival_city IS 'City of arrival';


--
-- Name: COLUMN routes.aircraft_id; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.aircraft_id IS 'Aircraft code, IATA';


--
-- Name: COLUMN routes.duration; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.duration IS 'Scheduled duration of flight';


--
-- Name: COLUMN routes.days_of_week; Type: COMMENT; Schema: bookings; Owner: -
--

  COMMENT  ON COLUMN routes.days_of_week IS 'Days of week on which flights are scheduled';

