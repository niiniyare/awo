
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET client_min_messages = warning;
SET row_security = off;

--SET search_path = bookings, pg_catalog;


--CREATE DATABASE flightBookings;


--\connect flightBookings



--CREATE SCHEMA bookings;



-- COMMENT  ON SCHEMA bookings IS 'Airlines flightBookings database schema';



--CREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;



--  COMMENT  ON EXTENSION plpgsql IS 'PL/pgSQL procedural language';


--SET search_path = bookings, pg_catalog;

CREATE TABLE IF NOT EXISTS airlines (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    company_name VARCHAR(50) NOT NULL,
    iata_code VARCHAR(2) NOT NULL,
    -- icao_code   VARCHAR(3),
    -- callsign VARCHAR(15),
    registared_country VARCHAR(2)NOT NULL,
    -- main_airport VARCHAR(3) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT (now()),
    updated_at timestamptz NOT NULL DEFAULT('0001-01-01 00:00:00Z')
 --   CONSTRAINT airlines_pk PRIMARY KEY (id)
  );

CREATE SEQUENCE IF NOT EXISTS airlines_id_seq
    START WITH 4001
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE airlines_id_seq OWNED BY airlines.id;


CREATE TABLE IF NOT EXISTS aircrafts (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    iata_code VARCHAR(3)  NOT NULL,
    icao_code VARCHAR(4)  NOT NULL,
    model VARCHAR(100) NOT NULL,
    range integer NOT NULL DEFAULT 1,
    company_id BIGSERIAL NOT NULL,
    created_at timestamptz NOT NULL DEFAULT (now()),
    CONSTRAINT aircrafts_range_check CHECK ((range > 0))
   --CONSTRAINT aircrafts_pkey PRIMARY KEY (code),
    -- CONSTRAINT airlines_pk FOREIGN KEY (company_id) REFERENCES airlines (id)
);


COMMENT  ON TABLE aircrafts IS 'Aircrafts (internal data)';


COMMENT ON COLUMN aircrafts.iata_code IS 'Aircraft code, IATA';
COMMENT ON COLUMN aircrafts.icao_code IS 'Aircraft code, ICAO';
COMMENT ON COLUMN aircrafts.model IS 'Aircraft model';

COMMENT  ON COLUMN aircrafts.range IS 'Maximal flying distance, km';
 /* CREATE EXTENSION IF NOT EXISTS postgis; */

CREATE TABLE IF NOT EXISTS airports (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    iata_code text NOT NULL,
    -- Check if it is a valid airport iata code, e.g. TLV, LAX, etc.
    CHECK (iata_code ~ '\A[A-Z]{3}\Z'),
    icao_code text NOT NULL,
    -- Check if it is a valid airport icao code, e.g. LLBG, KLAX, etc.
    CHECK (icao_code ~ '\A[A-Z0-9]{4}\Z'),
    name text NOT NULL,
  --  subdivision_code text NOT NULL,
    -- Check if it is a valid ISO 3166-2 subdivision code, e.g. IL-M, US-CA, etc.
    --CHECK (subdivision_code ~ '\A[A-Z]{2}-[A-Z0-9]{1,3}\Z'),
    country VARCHAR(50) NOT NULL,
    state VARCHAR(50) NOT NULL,

    city text NOT NULL,
    elevation text NULL NULL,
    
  -- we use below command if PostGIS is installed in our system 
    
  --  coordinates geography(point) NOT NULL,
  
  -- If PostGIS not available flowing row will work.
     lat NUMERIC(10, 2) NOT NULL,
     lon NUMERIC(10, 2) NOT NULL,
---   coordinates point NOT NULL,
    UNIQUE (iata_code, icao_code),
    timezone text NOT NULL
);

CREATE TABLE IF NOT EXISTS seats (
    aircraft_id BIGSERIAL NOT NULL,
    seat_no VARCHAR(4) NOT NULL,
    fare_conditions VARCHAR(10) NOT NULL,
    CONSTRAINT seats_fare_conditions_check CHECK (((fare_conditions)::text = ANY (ARRAY[('Economy'::VARCHAR)::text, ('Comfort'::VARCHAR)::text, ('Business'::VARCHAR)::text])))
);




  COMMENT  ON COLUMN seats.aircraft_id IS 'Aircraft code, IATA';



  COMMENT  ON COLUMN seats.seat_no IS 'Seat number';



  COMMENT  ON COLUMN seats.fare_conditions IS 'Travel class';
-- CREATE TABLE IF NOT EXISTS Schedules (
--     schedule_id BIGSERIAL PRIMARY KEY NOT NULL,
--     flight_no VARCHAR(6) NOT NULL,
--     departure_airport VARCHAR(3) NOT NULL,
--     arrival_airport VARCHAR(3) NOT NULL,
--     scheduled_departure_date DATE NOT NULL,
--     scheduled_arrival_date DATE NOT NULL,
--     company_id BIGSERIAL REFERENCES airlines(id)NOT NULL,
--
--     CONSTRAINT flights_check CHECK ((scheduled_arrival_date > scheduled_departure_date)),
--
--
--     CONSTRAINT flights_flight_no_scheduled_departure_key UNIQUE (flight_no, scheduled_departure),
--
--     CONSTRAINT flights_check_airlines_key UNIQUE (flight_no, company_id)
--     );
--
CREATE TABLE IF NOT EXISTS flights (
    flight_id BIGSERIAL PRIMARY KEY NOT NULL,
    flight_no VARCHAR(6) NOT NULL,
    company_id BIGSERIAL REFERENCES airlines(id)NOT NULL,
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

    CONSTRAINT flights_check_airlines_key UNIQUE (flight_no, company_id),


    CONSTRAINT flights_check1 CHECK (((actual_arrival IS NULL) OR ((actual_departure IS NOT NULL) AND (actual_arrival IS NOT NULL) AND (actual_arrival > actual_departure)))),

    CONSTRAINT flights_status_check CHECK (((status)::text = ANY (ARRAY[('On Time'::VARCHAR)::text, ('Delayed'::VARCHAR)::text, ('Departed'::VARCHAR)::text, ('Arrived'::VARCHAR)::text, ('Scheduled'::VARCHAR)::text, ('Cancelled'::VARCHAR)::text])))
);


-- CREATE SEQUENCE IF NOT EXISTS flight_id_seq
--     START WITH 100
--     INCREMENT BY 1
--     NO MINVALUE
--     NO MAXVALUE
--     CACHE 1;
--
--
-- ALTER SEQUENCE flight_no_seq OWNED BY flights.flight_no;


CREATE VIEW flights_v AS
 SELECT f.flight_id,
    f.flight_no,
    f.company_id,
    f.scheduled_departure,
    timezone(dep.timezone, f.scheduled_departure)::timestamp  AS scheduled_departure_local,
    f.scheduled_arrival,
    timezone(arr.timezone, f.scheduled_arrival)::timestamp AS scheduled_arrival_local,
    (f.scheduled_arrival - f.scheduled_departure) AS scheduled_duration,

-- SELECT to_char(interval '20 hours 20 minutes', 'HH24:MI:SS');
    f.departure_airport,
    dep.name AS departure_airport_name,
    dep.city AS departure_city,
    f.arrival_airport,
    arr.name AS arrival_airport_name,
    arr.city AS arrival_city,
    f.status,
    f.aircraft_id,
    f.actual_departure,
    timezone(dep.timezone, f.actual_departure)::timestamp AS actual_departure_local,
    f.actual_arrival,
    timezone(arr.timezone, f.actual_arrival)::timestamp AS actual_arrival_local,
    (f.actual_arrival - f.actual_departure) AS actual_duration
   FROM flights f,
    airports dep, 
    airports arr
  WHERE ((f.departure_airport = dep.iata_code) AND (f.arrival_airport = arr.iata_code));

  COMMENT  ON VIEW flights_v IS 'Flights (extended)';

  COMMENT  ON COLUMN flights_v.flight_id IS 'Flight ID';

  COMMENT  ON COLUMN flights_v.flight_no IS 'Flight number';

  COMMENT  ON COLUMN flights_v.company_id IS 'Airline company';

  COMMENT  ON COLUMN flights_v.scheduled_departure IS 'Scheduled departure time';


  COMMENT  ON COLUMN flights_v.scheduled_departure_local IS 'Scheduled departure time, local time at the point of departure';



  COMMENT  ON COLUMN flights_v.scheduled_arrival IS 'Scheduled arrival time';



  COMMENT  ON COLUMN flights_v.scheduled_arrival_local IS 'Scheduled arrival time, local time at the point of destination';

  COMMENT  ON COLUMN flights_v.scheduled_duration IS 'Scheduled flight duration';


  COMMENT  ON COLUMN flights_v.departure_airport IS 'Deprature airport code';


  COMMENT  ON COLUMN flights_v.departure_airport_name IS 'Departure airport name';



  COMMENT  ON COLUMN flights_v.departure_city IS 'City of departure';



  COMMENT  ON COLUMN flights_v.arrival_airport IS 'Arrival airport code';



  COMMENT  ON COLUMN flights_v.arrival_airport_name IS 'Arrival airport name';



  COMMENT  ON COLUMN flights_v.arrival_city IS 'City of arrival';



  COMMENT  ON COLUMN flights_v.status IS 'Flight status';



  COMMENT  ON COLUMN flights_v.aircraft_id IS 'Aircraft code, IATA';



  COMMENT  ON COLUMN flights_v.actual_departure IS 'Actual departure time';



  COMMENT  ON COLUMN flights_v.actual_departure_local IS 'Actual departure time, local time at the point of departure';



  COMMENT  ON COLUMN flights_v.actual_arrival IS 'Actual arrival time';



  COMMENT  ON COLUMN flights_v.actual_arrival_local IS 'Actual arrival time, local time at the point of destination';



  COMMENT  ON COLUMN flights_v.actual_duration IS 'Actual flight duration';



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



  COMMENT  ON VIEW routes IS 'Routes';



  COMMENT  ON COLUMN routes.flight_no IS 'Flight number';



  COMMENT  ON COLUMN routes.departure_airport IS 'Code of airport of departure';



  COMMENT  ON COLUMN routes.departure_airport_name IS 'Name of airport of departure';



  COMMENT  ON COLUMN routes.departure_city IS 'City of departure';



  COMMENT  ON COLUMN routes.arrival_airport IS 'Code of airport of arrival';



  COMMENT  ON COLUMN routes.arrival_airport_name IS 'Name of airport of arrival';



  COMMENT  ON COLUMN routes.arrival_city IS 'City of arrival';



  COMMENT  ON COLUMN routes.aircraft_id IS 'Aircraft code, IATA';



  COMMENT  ON COLUMN routes.duration IS 'Scheduled duration of flight';



  COMMENT  ON COLUMN routes.days_of_week IS 'Days of week on which flights are scheduled';
/*

flight_id, 
flight_no, 
company_id, 
scheduled_departure,
scheduled_departure_local, 
scheduled_arrival,
scheduled_arrival_local, 
scheduled_duration, 
departure_airport, 
departure_airport_name, 
departure_city, 
arrival_airport, 
arrival_airport_name, 
arrival_city,
status, 
aircraft_id, 
actual_departure,
actual_departure_local,
actual_arrival,
 actual_arrival_local,
 actual_duration
 
 */




CREATE TABLE cabin_class (
    value text PRIMARY KEY,
    description text NOT NULL
);
INSERT INTO cabin_class VALUES
    ('E', 'Economy class'),
    ('B', 'Business class'),
    ('F', 'First class');


/*
 * Define functions
 */

CREATE OR REPLACE FUNCTION  column_layout_seats_count(column_layout text)
RETURNS integer AS $total$  
declare  
    total integer;  
BEGIN  
   char_length(translate(column_layout, '-#', ''))into total;  
   RETURN total;  
END;  
$total$ LANGUAGE plpgsql;


-- CREATE FUNCTION column_layout_seats_count(column_layout text) RETURNS integer
-- LANGUAGE SQL
-- IMMUTABLE
-- RETURNS NULL ON NULL INPUT
-- RETURN char_length(translate(column_layout, '-#', ''));



/*
 * Create tables
 */




CREATE TABLE seat_map (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    aircraft_id integer REFERENCES aircrafts (id) NOT NULL,
    cabin_class text REFERENCES cabin_class (value) NOT NULL,
    start_row integer NOT NULL CHECK (start_row > 0),
    end_row integer NOT NULL CHECK (end_row > 0),
    CHECK (start_row <= end_row),
    column_layout text NOT NULL,
    -- Check if column layout is in the correct form, e.g. ABC-EF-GHI, ABC, ABC-DEF, etc.)
    CHECK (column_layout ~ '\A[A-Z#]+(?:-[A-Z#]+)*\Z'),
    UNIQUE (aircraft_id, start_row, end_row)
);

CREATE TABLE booked_seat (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    flight_id integer NOT NULL REFERENCES flights (flight_id),
    cabin_class text REFERENCES cabin_class (value) NOT NULL,
    seat_row integer NOT NULL CHECK (seat_row > 0),
    seat_column text NOT NULL,
    -- Check if the seat's column is a single uppercase letter (A-Z).
    CHECK (seat_column ~ '\A[A-Z]\Z'),
    UNIQUE (flight_id, seat_row, seat_column)
);


/*
 * Create views
 */

CREATE VIEW cabin_seats_count AS
    SELECT aircraft_id,
           cabin_class,
           sum((end_row - start_row + 1) * column_layout_seats_count(column_layout)) AS seat_count
    FROM seat_map
    GROUP BY aircraft_id, cabin_class;
--
-- CREATE VIEW cabin_seats_count AS
--     SELECT aircraft_id,
--            cabin_class,
--            sum((end_row - start_row + 1) * column_layout_seats_count(column_layout)) AS seat_count
--     FROM seat_map
--     GROUP BY aircraft_model_id, cabin_class;

--
--
-- INSERT INTO
--     seat_map (
--         aircraft_id,
--         cabin_class,
--         start_row,
--         end_row,
--         column_layout
--     )
-- VALUES
--     (1, 'F', 1, 8, 'A-DG-K'),
--     (1, 'B', 10, 14, 'AC-DFG-HK'),
--     (1, 'E', 21, 28, 'ABC-DFG-HJK'),
--     (1, 'E', 29, 30, '###-###-HJK'),
--     (1, 'E', 35, 36, 'ABC-###-HJK'),
--     (1, 'E', 37, 48, 'ABC-DFG-HJK'),
--     (1, 'E', 49, 50, '###-DFG-###');

-- PostgreSQL schema for flight data

-- CREATE TABLE flights (
--   -- Airline code
--   airline_code VARCHAR(2) NOT NULL,
--   -- Flight number
--   flight_number INTEGER NOT NULL,
--   -- Date range of the flight
--   date_range DATERANGE NOT NULL,
--   -- Day of week of the flight
--   dow INTEGER NOT NULL,
--   -- Legs of the flight
--   legs JSONB NOT NULL,
--   -- Segments of the flight
--   segments JSONB NOT NULL,
--   -- Primary key
--   PRIMARY KEY (airline_code, flight_number),
--   -- GUID
--   guid UUID NOT NULL DEFAULT uuid_generate_v4(),
--   -- Created at
--   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
--   -- Updated at
--   updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
-- );

-- CREATE TABLE legs (
--   -- Boarding point
--   board_point VARCHAR(3) NOT NULL,
--   -- Off point
--   off_point VARCHAR(3) NOT NULL,
--   -- Boarding time
--   board_time TIME NOT NULL,
--   -- Arrival date offset
--   arrival_date_offset INTEGER NOT NULL,
--   -- Arrival time
--   arrival_time TIME NOT NULL,
--   -- Elapsed time
--   elapsed_time TIME NOT NULL,
--   -- Leg cabins
--   leg_cabins JSONB NOT NULL,
--   -- GUID
--   guid UUID NOT NULL DEFAULT uuid_generate_v4(),
--   -- Created at
--   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
--   -- Updated at
--   updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

--   -- Foreign key to flights table
--   FOREIGN KEY (airline_code, flight_number) REFERENCES flights (airline_code, flight_number),
--   -- Index on boarding point and off point
--   INDEX (board_point, off_point)
-- );

-- CREATE TABLE leg_cabins (
--   -- Cabin code
--   cabin_code VARCHAR(1) NOT NULL,
--   -- Cabin capacity
--   capacity INTEGER NOT NULL,
--   -- Primary key
--   PRIMARY KEY (cabin_code),
--   -- GUID
--   guid UUID NOT NULL DEFAULT uuid_generate_v4(),
--   -- Created at
--   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
--   -- Updated at
--   updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
-- );

-- CREATE TABLE segments (
--   -- Whether the segment is specific
--   specific BOOLEAN NOT NULL,
--   -- Fare family
--   fare_family VARCHAR(255) NOT NULL,
--   -- Disutility curve
--   disutility_curve VARCHAR(255) NOT NULL,
--   -- Class
--   class VARCHAR(1) NOT NULL,
--   -- Foreign key to legs table
--   FOREIGN KEY (airline_code, flight_number, leg_number) REFERENCES legs (airline_code, flight_number, leg_number),
--   -- GUID
--   guid UUID NOT NULL DEFAULT uuid_generate_v4(),
--   -- Created at
--   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
--   -- Updated at
--   updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
-- );

