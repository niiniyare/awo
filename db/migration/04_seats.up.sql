CREATE TABLE IF NOT EXISTS seats (
    aircraft_id BIGSERIAL NOT NULL,
    seat_no VARCHAR(4) NOT NULL,
    fare_conditions VARCHAR(10) NOT NULL,
    CONSTRAINT seats_fare_conditions_check CHECK (((fare_conditions)::text = ANY (ARRAY[('Economy'::VARCHAR)::text, ('Comfort'::VARCHAR)::text, ('Business'::VARCHAR)::text])))
);




  COMMENT  ON COLUMN seats.aircraft_id IS 'Aircraft code, IATA';



  COMMENT  ON COLUMN seats.seat_no IS 'Seat number';



  COMMENT  ON COLUMN seats.fare_conditions IS 'Travel class';
