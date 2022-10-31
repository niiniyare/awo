



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

-- CREATE FUNCTION column_layout_seats_count(column_layout text) RETURNS integer
-- LANGUAGE SQL
-- IMMUTABLE
-- RETURNS NULL ON NULL INPUT
-- RETURN char_length(translate(column_layout, '-#', ''));
--

/*
 * Create tables
 */

<<<<<<< HEAD
-- CREATE TABLE airport (
--     id BIGSERIAL PRIMARY KEY NOT NULL,
--     iata_code text NOT NULL,
--     -- Check if it is a valid airport iata code, e.g. TLV, LAX, etc.
--     CHECK (iata_code ~ '\A[A-Z]{3}\Z'),
--     icao_code text NOT NULL,
--     -- Check if it is a valid airport icao code, e.g. LLBG, KLAX, etc.
--     CHECK (icao_code ~ '\A[A-Z]{4}\Z'),
--     name text NOT NULL,
--     subdivision_code text NOT NULL,
--     -- Check if it is a valid ISO 3166-2 subdivision code, e.g. IL-M, US-CA, etc.
--     CHECK (subdivision_code ~ '\A[A-Z]{2}-[A-Z0-9]{1,3}\Z'),
--     city text NOT NULL,
--     geo_location point NOT NULL,
--     -- geo_location geography(point) NOT NULL,
--     UNIQUE (iata_code, icao_code)
-- -- );
-- --
-- CREATE TABLE service (
--     id integer PRIMARY KEY,
--     origin_airport_id integer NOT NULL REFERENCES airport (id),
--     destination_airport_id integer NOT NULL REFERENCES airport (id),
--     UNIQUE (origin_airport_id, destination_airport_id)
-- );
--
-- CREATE TABLE aircraft_model (
--     id BIGSERIAL PRIMARY KEY NOT NULL,
--     icao_code text NOT NULL,
--     -- Check if it is a valid aircraft icao code, e.g. A5, B487, etc.
--     CHECK (icao_code ~ '\A[A-Z0-9]{2,4}\Z'),
--     iata_code text NOT NULL,
--     -- Check if it is a valid aircraft iata code, e.g. A4F, 313, etc.
--     CHECK (iata_code ~ '\A[A-Z0-9]{3}\Z'),
--     name text NOT NULL,
--     UNIQUE (icao_code, iata_code)
-- );
--
=======



>>>>>>> 41c416a (corrected 'seats extanded' migration)
CREATE TABLE seat_map (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    aircraft_id integer REFERENCES aircrafts (id),
    cabin_class text REFERENCES cabin_class (value) NOT NULL,
    start_row integer NOT NULL CHECK (start_row > 0),
    end_row integer NOT NULL CHECK (end_row > 0),
    CHECK (start_row <= end_row),
    column_layout text NOT NULL,
    -- Check if column layout is in the correct form, e.g. ABC-EF-GHI, ABC, ABC-DEF, etc.)
    CHECK (column_layout ~ '\A[A-Z#]+(?:-[A-Z#]+)*\Z'),
    UNIQUE (aircraft_id, start_row, end_row)
);
<<<<<<< HEAD
--
-- CREATE TABLE flight (
--     id BIGSERIAL PRIMARY KEY NOT NULL,
--     service_id integer NOT NULL REFERENCES service (id),
--     departure_terminal text NOT NULL,
--     departure_time timestamptz NOT NULL,
--     arrival_terminal text NOT NULL,
--     arrival_time timestamptz NOT NULL,
--     CHECK (departure_time < arrival_time),
--     aircraft_id integer NOT NULL REFERENCES aircrafts (id)
-- );
--
=======




>>>>>>> 41c416a (corrected 'seats extanded' migration)
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

-- CREATE VIEW cabin_seats_count AS
--     SELECT aircraft_id,
--            cabin_class,
--            sum((end_row - start_row + 1) * column_layout_seats_count(column_layout)) AS seat_count
--     FROM seat_map
--     GROUP BY aircraft_id, cabin_class;
--
--
--
--
