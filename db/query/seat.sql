

-- name: CreateSeat :one
INSERT INTO seats ( 
aircraft_id, 
seat_no, 
fare_conditions)
VALUES($1, $2, $3)
RETURNING *;

-- name: GetSeats :many
SELECT * FROM seats;

