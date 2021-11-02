
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

-- name: GetAircraft :one
SELECT * FROM aircrafts_data
WHERE aircraft_code = $1 LIMIT 1;

-- name: ListAircraft :many
SELECT * FROM aircrafts_data
ORDER BY name;

-- name: CreateAircraft :one
INSERT INTO aircrafts_data (
  aircraft_code, model, range, company_id
) VALUES (
  $1, $2, $3 ,$4
)
RETURNING *;

-- name: DeleteAircraft :exec
DELETE FROM aircrafts_data
WHERE aircraft_code = $1;
/*
-- Example
INSERT INTO aircrafts_data (
  aircraft_code, model, range, company_id
) VALUES (
773	,'{"Boeing": "Boeing 777-300"}' ,	11100, 1
)
RETURNING *;

*/


-- name: GetAirlineCompany :one
SELECT * FROM airline_company
WHERE company_id = $1 
LIMIT 1;

-- name: ListAirlineCompany :many
SELECT * FROM airline_company
ORDER BY name;

-- name: CreateAirlineCompany :one
INSERT INTO airline_company (
  company_name,
  iata_code,
  main_airport
) VALUES (
  $1, $2, $3
)
RETURNING *;

-- name: DeleteAirlineCompany :exec
DELETE FROM airline_company
WHERE company_id = $1;
/*

INSERT INTO airline_company (
  company_name,
  iata_code,
  main_airport
) VALUES (
 'Turkish Airlines', 'TK', 'IST'
)
RETURNING *;

\copy airports_data(airport_code,airport_name,country,city,coordinates,ACOA,timezone) FROM '/data/data/com.termux/files/home/downloads/airports_data.csv' DELIMITER ','  CSV HEADER;
INSERT INTO airports_data (airport_code, airport_name ,city ,coordinates,timezone)
VALUES ('IST', '{'name' : 'Istanbul Internationa Airport'}','{'name' : 'Istanbul'}', POINT(40.982555, 28.820829), 'Europe/Istanbul')
RETURNING *;

INSERT INTO airports_data (airport_code, airport_name ,city ,coordinates,timezone)
VALUES ('IST', 'Istanbul Internationa Airport', 'Istanbul', POINT(40.982555, 28.820829), 'Europe/Istanbul')
RETURNING *;

*/