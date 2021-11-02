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