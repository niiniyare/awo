
-- name: CreateAirports :one
INSERT INTO airports (
  airport_code , 
  airport_name ,
  city , 
  coordinates
) VALUES (
  $1 , $2 , $3 , $4
)
RETURNING * ;
-- name: CreateAirportList :many
INSERT INTO airports (
  airport_code , 
  airport_name ,
  city , 
  coordinates
) VALUES (
  $1 , $2 , $3 , POINT($4)
)
RETURNING * ;
-- name: GetAirports :one
SELECT * FROM airports
WHERE airport_code  = $1 LIMIT 1;

-- name: ListAirports :many
SELECT airport_code,
  airport_name ,
  city
FROM airports
ORDER BY airport_code;

-- name: DeleteAirports :exec
DELETE FROM airports
WHERE airport_code  = $1
RETURNING *;

