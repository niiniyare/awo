CREATE TABLE fare_rules (
  fare_id INTEGER PRIMARY KEY,
  fare_basis_code CHAR(8) NOT NULL,
  route VARCHAR(25) NOT NULL,
  origin VARCHAR(3) NOT NULL,
  destination VARCHAR(3) NOT NULL,
  effective_date DATE NOT NULL,
  expiration_date DATE NOT NULL,
 min_stay INTEGER NOT NULL,
  max_stay INTEGER NOT NULL,
   FOREIGN KEY (fare_id) REFERENCES fares (fare_id)
                    );
                      

CREATE TABLE conditions (
    condition_id INTEGER PRIMARY KEY,
     fare_id INTEGER NOT NULL,
    class_code CHAR(1) NOT NULL,
    max_passengers INTEGER NOT NULL,
    FOREIGN KEY (fare_id) REFERENCES fares (fare_id),
   FOREIGN KEY (class_code) REFERENCES class_of_service (class_code)
            );
              
)


CREATE VIEW routes AS
  SELECT f3.flight_no,
    -- f3.company_id,
    f3.departure_airport,
    dep.name AS departure_airport_name,
    dep.city AS departure_city,
    f3.arrival_airport,
    arr.name AS arrival_airport_name,
    arr.city AS arrival_city,
    f3.aircraft_code,
    f3.duration,
    f3.days_of_week,
    f.fare_basis_code,
    f.route,
    f.origin,
    f.destination,
    f.effective_date,
    f.expiration_date,
    f.min_stay,
    f.max_stay
  FROM (
    SELECT f2.flight_no,
      -- f2.company_id,
      f2.departure_airport,
      f2.arrival_airport,
      f2.aircraft_code,
      f2.duration,
      array_agg(f2.days_of_week) AS days_of_week
    FROM (
      SELECT f1.flight_no,
        -- f1.company_id,
        f1.departure_airport,
        f1.arrival_airport,
        f1.aircraft_code,
        f1.duration,
        f1.days_of_week,
        row_number() OVER (PARTITION BY f1.flight_no, f1.departure_airport, f1.arrival_airport ORDER BY f1.days_of_week) as rn
      FROM (
        SELECT f.flight_no,
          -- f.company_id,
          f.departure_airport,
          f.arrival_airport,
          f.aircraft_code,
          (f.scheduled_arrival - f.scheduled_departure) AS duration,
          (CASE
            WHEN (EXTRACT(DOW FROM f.scheduled_departure) = 0) THEN 'Sunday'::VARCHAR
            WHEN (EXTRACT(DOW FROM f.scheduled_departure) = 1) THEN 'Monday'::VARCHAR
            WHEN (EXTRACT(DOW FROM f.scheduled_departure) = 2) THEN 'Tuesday'::VARCHAR
            WHEN (EXTRACT(DOW FROM f.scheduled_departure) = 3) THEN 'Wednesday'::VARCHAR
            WHEN (EXTRACT(DOW FROM f.scheduled_departure) = 4) THEN 'Thursday'::VARCHAR
            WHEN (EXTRACT(DOW FROM f.scheduled_departure) = 5) THEN 'Friday'::VARCHAR
            WHEN (EXTRACT(DOW FROM f.scheduled_departure) = 6) THEN 'Saturday'::VARCHAR
          END) AS days_of_week
        FROM flights f
      ) f1
    ) f2
    WHERE f2.rn = f1.rn) as f3;


CREATE VIEW routes AS
SELECT f.flight_no,
  f.company_id,
  f.departure_airport,
  f.arrival_airport,
  f.aircraft_id,
  (f.scheduled_arrival - f.scheduled_departure) AS duration,
  array_agg(f.days_of_week) AS days_of_week,
  f.fare_id,
  fr.fare_basis_code,
  fr.route,
  fr.origin,
  fr.destination,
  fr.effective_date,
  fr.expiration_date,
  fr.min_stay,
  fr.max_stay,
  c.class_code,
  c.class_name
FROM flights f
JOIN fare_rules fr ON f.fare_id = fr.fare_id
JOIN class_of_service c ON f.class_code = c.class_code
GROUP BY f.flight_no, f.company_id, f.departure_airport, f.arrival_airport, f.aircraft_id, f.fare_id, fr.fare_basis_code, fr.route, fr.origin, fr.destination, fr.effective_date, fr.expiration_date, fr.min_stay, fr.max_stay, c.class_code, c.class_name;



