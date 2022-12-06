-- name: CreateFlight :one
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


-- SELECT f.flight_id, f.flight_no, f.company_id, f.scheduled_departure, f.scheduled_departure_local, f.scheduled_arrival, f.scheduled_arrival_local, f.scheduled_duration, f.departure_airport, f.departure_airport_name, f.departure_city, f.arrival_airport, f.arrival_airport_name, f.arrival_city, f.status, f.aircraft_id, f.actual_departure, f.actual_departure_local, f.actual_arrival, f.actual_arrival_local, COALESCE(f.actual_duration ,0) as  actual_duration 
-- FROM flights_v f 
-- WHERE f.departure_airport = $1
-- AND f.arrival_airport = $2
-- AND f.scheduled_departure > now()
-- AND f.company_id = $3
-- ORDER BY f.scheduled_departure LIMIT 2
--
-- name: FlightAvailability :many

SELECT 
  f.flight_id
, f.flight_no
, f.company_id
, f.scheduled_departure
, f.scheduled_departure_local
, f.scheduled_arrival
, f.scheduled_arrival_local
, f.scheduled_duration
, f.departure_airport
, f.departure_airport_name
, f.departure_city
, f.arrival_airport
, f.arrival_airport_name
, f.arrival_city
, f.status
, f.aircraft_id
, f.actual_departure
, f.actual_departure_local
, f.actual_arrival
, f.actual_arrival_local
, f.actual_duration
FROM flights_v f 
WHERE   f.departure_airport = $1
    AND f.arrival_airport = $2
    AND f.scheduled_departure > now()
    AND f.company_id = $3
ORDER BY f.scheduled_departure ;
--
--
-- SELECT f.* 
-- FROM flights_v f 
-- WHERE f.departure_airport = 'MGQ'
-- AND f.arrival_airport = 'HGA'
-- AND f.scheduled_departure > now()
-- AND f.company_id = 1
-- ORDER BY f.scheduled_departure;
--

--
-- -- name: GetAllFlight :many
--
-- SELECT * FROM flights
-- LIMIT $1
-- OFFSET $2
-- ;



--
-- -- name: GetFlightByCode :one
-- SELECT * FROM flights_v
-- WHERE departure_airport  = $1 
-- AND arrival_airport = $2
-- AND scheduled_departure =$3
-- AND scheduled_arrival = $4
-- LIMIT 1;
--
--
-- -- name: ListFlightByDuration :many
-- SELECT *
-- FROM flights_v
-- ORDER BY scheduled_duration;
--
-- -- name: ListFlightByDepartureTime :many
-- SELECT *
-- FROM flights_v
-- ORDER BY departure_airport;
--
-- -- name: ListFlightByArrivalTime :many
-- SELECT *
-- FROM flights_v
-- ORDER BY scheduled_arrival;
--
-- -- name: DeleteFlight :exec
-- DELETE FROM flights_v
-- WHERE flight_id  = $1
-- RETURNING flight_no;
-- -- name: GetFlightListByName :many
-- SELECT f.* 
-- FROM flights_v f 
-- WHERE f.departure_city = $1
-- AND f.arrival_city = $2
-- AND f.scheduled_departure > now() 
-- ORDER BY f.scheduled_departure LIMIT 1;
-- 


-- Example CreateFlight 

-- INSERT INTO flights (
--     flight_no,
--     company_id ,
--     scheduled_departure  ,                                                     
--     scheduled_arrival  ,
--     departure_airport ,
--     arrival_airport ,
--     status ,                                                                    
--     aircraft_id,
--     actual_departure ,
--     actual_arrival
-- )
-- VALUES 
-- ( 'AY26', 1, '2022-09-12 21:00:00+3', '2022-09-12 23:00:00+3', 'MGQ','HGA', 'Scheduled', 1, '2022-09-12 21:00:00+3', '2022-09-12 23:00:00+3')
--RETURNING *
--

--

