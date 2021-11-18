
-- name: CreateAirport :one
INSERT INTO airports_data (
  airport_code , 
  airport_name ,
  country_code,
  city , 
  coordinates,
  timezone
) VALUES (
  $1 , $2 , $3 , $4 , Point($5),$6
)
RETURNING * ;

-- name: GetAirport :one
SELECT * FROM airports_data
WHERE airport_code  = $1 LIMIT 1;

-- name: UpdateAirport :one
UPDATE airports_data 
SET coordinates = ST_GeomFromText('Point($2)') WHERE airport_code = $1
RETURNING * ;

-- name: ListAirport :many
SELECT airport_code,
  airport_name ,
  city
FROM airports_data
ORDER BY airport_code;

-- name: DeletedAirport :exec
DELETE FROM airports_data
WHERE airport_code  = $1
RETURNING *;

