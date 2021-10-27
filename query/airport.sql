
-- name: CreateAirports :one
INSERT INTO airports_data (
  airport_code , 
  airport_name ,
  city , 
  
  
  coordinates
) VALUES (
  $1 , $2 , $3 , $4
)
RETURNING * ;
-- name: CreateAirportList :many
INSERT INTO airports_data (
  airport_code , 
  airport_name ,
  city , 
  coordinates
) VALUES (
  $1 , $2 , $3 , $4
)
RETURNING * ;
-- name: GetAirports :one
SELECT * FROM airports_data
WHERE airport_code  = $1 LIMIT 1;

-- name: ListAirports :many
SELECT airport_code,
  airport_name ,
  city
FROM airports_data
ORDER BY airport_code;

-- name: DeleteAirports :exec
DELETE FROM airports_data
WHERE airport_code  = $1
RETURNING *;

