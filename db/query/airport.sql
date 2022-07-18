-- name: CreateAirport :one
INSERT INTO airports(
iata_code, 
icao_code, 
name, 
--subdivision_code
city 
--coordinates
) VALUES
(  $1 , $2 , $3 , $4
)
RETURNING * ;
-- name: GetAirport :one
SELECT * FROM airports
WHERE iata_code = $1 LIMIT 1;

-- name: ListAirport :many
SELECT id, iata_code,
name ,
city
FROM airports
ORDER BY id;

-- name: DeleteAirports :exec
DELETE FROM airports
WHERE id = $1
RETURNING *;