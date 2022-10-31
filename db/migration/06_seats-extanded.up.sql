



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
