-- name: CreateAirports :one
INSERT INTO airport 
(iata_code, icao_code, name, subdivision_code, city, coordinates) VALUES
(  $1 , $2 , $3 , $4, $5, $6)
RETURNING * ;
-- name: CreateAirportList :many
INSERT INTO airports (
    iata_code
    name ,
    city ,
    coordinates
) VALUES (
    $1 , $2 , $3 , SRID=4326;POINT($4)
)
RETURNING * ;

-- name: GetAirports :one
SELECT * FROM airports
WHERE airport_code = $1 LIMIT 1;

-- name: ListAirports :many
SELECT id, iata_code
name ,
city
FROM airports
ORDER BY id;

-- name: DeleteAirports :exec
DELETE FROM airports
WHERE id = $1
RETURNING *;