-- name: createFlight :one
INSERT INTO flights (
    flight_no,
    company_id ,
    scheduled_departure  ,
    scheduled_arrival  ,
    departure_airport ,
    arrival_airport ,
    status ,
    aircraft_id ,
    actual_departure ,
    actual_arrival 
    


)
VALUES (
$1, $2, $3, $4, $5, $6, $7, $8, $9,$10
)
RETURNING *;
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