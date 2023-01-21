-- name: GetAirline :one
SELECT * FROM airlines
WHERE id = $1 
LIMIT 1;

-- name: ListAirline :many
SELECT * FROM airlines
ORDER BY company_name
LIMIT $1
OFFSET $2;

-- name: CreateAirline :one
INSERT INTO airlines (
      company_name,
      iata_code, 
      icao_code, 
      callsign, 
      registared_country, 
      main_airport
) VALUES (
  $1, $2, $3, $4,$5,$6
)
RETURNING *;

-- name: DeleteAirline :exec
DELETE FROM airlines
WHERE id = $1;


-- \copy airports_data(airport_code,airport_name,country,city,coordinates,ACOA,timezone) FROM '/data/data/com.termux/files/home/downloads/airports_data.csv' DELIMITER ','  CSV HEADER;



-- INSERT INTO airports_data (airport_code, airport_name ,city ,coordinates,timezone)
-- VALUES ('IST', '{'name' : 'Istanbul Internationa Airport'}','{'name' : 'Istanbul'}', POINT(40.982555, 28.820829), 'Europe/Istanbul')
-- RETURNING *;



-- INSERT INTO airports_data (airport_code, airport_name ,city ,coordinates,timezone)
-- VALUES ('IST', 'Istanbul Internationa Airport', 'Istanbul', POINT(40.982555, 28.820829), 'Europe/Istanbul')
-- RETURNING *;
--


-- INSERT INTO airlines 
-- (company_name,  iata_code,  main_airport) 
-- VALUES 
--
-- ('Somali Airline', 'SO', 'MGQ'),
-- ('Turkish Airways', 'TK', 'IST'),
-- ('Ethiopian Airlines', 'ET', 'ADD');
