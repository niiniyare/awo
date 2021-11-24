-- name: GetAircraft :one
SELECT * FROM aircrafts_data
WHERE aircraft_code = $1 LIMIT 1;

-- name: ListAircraft :many
SELECT * FROM aircrafts_data
ORDER BY name;
/*
-- name: CreateAircraft :one
INSERT INTO aircrafts_data (
  aircraft_code, model, range ,company_id
) VALUES (
  '773','Boeing 777-300',11100,1
'763','Boeing 767-300',7900,
'SU9','Sukhoi Superjet-100' ,3000, 
'320','Airbus A320-200',5700,
'321','Airbus A321-200',5600,
'319','Airbus A319-100',6700,
'733','Boeing 737-300',4200,
'CN1','Cessna 208 Caravan',1200,
'CR2','Bombardier CRJ-200',2700,
)
RETURNING *;

-- name: DeleteAircraft :exec
DELETE FROM aircrafts_data
WHERE aircraft_code = $1;

-- Example
INSERT INTO aircrafts_data (
  aircraft_code, model, range, company_id
) VALUES (
773	,'{'Boeing': 'Boeing 777-300'}' ,	11100, 1
)
RETURNING *;

*/