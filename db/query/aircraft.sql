-- name: GetAircraft :one
SELECT * FROM aircrafts
WHERE id = $1 LIMIT 1;

-- name: ListAircraft :many
SELECT * FROM aircrafts
ORDER BY id
LIMIT $1
OFFSET $2;

-- name: CreateAircraft :one
INSERT INTO aircrafts (
  code, model, range, company_id
) VALUES (
  $1, $2, $3 ,$4
)
RETURNING *;

-- name: DeleteAircraft :exec
DELETE FROM aircrafts
WHERE id = $1;
/*
-- Example
INSERT INTO aircrafts (
  code, model, range, company_id
) VALUES (
773	,'{"Boeing": "Boeing 777-300"}' ,	11100, 1
)
RETURNING *;

*/