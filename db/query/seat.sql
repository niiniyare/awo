

-- name: CreateSeat :one
INSERT INTO seats ( 
aircraft_id, 
seat_no, 
fare_conditions)
VALUES($1, $2, $3)
RETURNING *;

-- name: GetSeats :many
SELECT * FROM seats;

-- iata_code, 
-- icao_code,
-- model,
-- range, 
-- company_id


-- name: GetAircraftSeatsWithIataCode :many
SELECT   a.iata_code,
         a.icao_code,
         a.model,
         a.range,
         s.seat_no,
         s.fare_conditions
FROM     aircrafts a
         JOIN seats s ON a.aircraft_code = s.aircraft_code
WHERE    a.iata_code = $1
ORDER BY s.seat_no;

-- -- name: InsertNewSeatMap :copyfrom
-- INSERT INTO seat_map (
--   aircraft_id, 
--   cabin_class, 
--   start_row, 
--   end_row, 
--   column_layout)
-- VALUES
-- ($1, $2, $3, $4, $5);
-- --(1, 'E', 49, 50, '###-DFG-###');
