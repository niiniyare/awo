-- name: CreateAirport :one
INSERT INTO airports(
iata_code, 
icao_code, 
name, 
elevation,
--subdivision_code
city,
country,
state,
lat,
lon,
timezone
) VALUES
(  $1 , $2 , $3 , $4, $5, $6, $7, $8, $9, $10
)
RETURNING * ;
-- name: GetAirport :one
SELECT * FROM airports
WHERE iata_code = $1;

-- name: ListAirport :many
SELECT id, iata_code, name ,city
FROM airports
OFFSET $1
LIMIT $2;

-- name: DeleteAirports :one
DELETE FROM airports
WHERE iata_code = $1
RETURNING *;

-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('ALU','HCMA','Alula Airport',6,'Alula','SO','Bari',50.748000,11.958200,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('BIB','HCMB','Baidoa Airport',1820,'Baidoa','SO','Bay',43.628601,3.102220,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('CXN','HCMC','Candala Airport',9,'Candala','SO','Bari',49.917000,11.500000,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('BSY','HCMD','Bardera Airport',4200,'','SO','Gedo',42.307778,2.336111,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('HCM','HCME','Eil Airport',812,'Eil','SO','Nugaal',49.799999,7.917000,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('BSA','HCMF','Bosaso Airport',3,'Bosaso','SO','Bari',49.149399,11.275300,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('GSR','HCMG','Gardo Airport',2632,'Gardo','SO','Bari',49.083000,9.517000,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('HGA','HCMH','Egal International Airport',4423,'Hargeisa','SO','Woqooyi-Galbeed',44.088799,9.518170,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('BBO','HCMI','Berbera Airport',30,'Berbera','SO','Woqooyi-Galbeed',44.941101,10.389200,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('KMU','HCMK','Kisimayu Airport',49,'','SO','Lower-Juba',42.459202,-0.377353,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('MGQ','HCMM','Aden Adde International Airport',29,'Mogadishu','SO','Banaadir',45.304699,2.014440,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('CMO','HCMO','Obbia Airport',65,'Obbia','SO','Mudug',48.516666,5.366667,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('GLK','HCMR','Galcaio Airport',975,'Galcaio','SO','Mudug',47.454700,6.780830,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('CMS','HCMS','Scusciuban Airport',1121,'Scusciuban','SO','Bari',50.233002,10.300000,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('ERA','HCMU','Erigavo Airport',5720,'Erigavo','SO','Sanaag',47.387981,10.642051,'Africa/Mogadishu');
-- INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)VALUES('BUO','HCMV','Burao Airport',3400,'Burao','SO','Togdheer',45.554900,9.527500,'Africa/Mogadishu');
