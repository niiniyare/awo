/*
-- name: GetFlightByCode :one
SELECT * FROM flights_v
WHERE departure_airport  = $1 
AND arrival_airport = $2
AND scheduled_departure =$3
AND scheduled_arrival = $4
LIMIT 1;


-- name: ListFlightByDuration :many
SELECT *
FROM flights_v
ORDER BY scheduled_duration;

-- name: ListFlightByDepartureTime :many
SELECT *
FROM flights_v
ORDER BY departure_airport;

-- name: ListFlightByArrivalTime :many
SELECT *
FROM flights_v
ORDER BY scheduled_arrival;

-- name: DeleteFlight :exec
DELETE FROM flights_v
WHERE flight_id  = $1
RETURNING flight_no;
-- name: GetFlightListByName :many
SELECT f.* 
FROM flights_v f 
WHERE f.departure_city = $1
AND f.arrival_city = $2
AND f.scheduled_departure > now() 
ORDER BY f.scheduled_departure LIMIT 1;
*/