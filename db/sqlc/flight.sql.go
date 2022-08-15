// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.14.0
// source: flight.sql

package db

import (
	"context"
	"database/sql"
	"time"
)

const createFlight = `-- name: CreateFlight :one
INSERT INTO flights (
    flight_no,
    company_id ,
    scheduled_departure  ,
    scheduled_arrival  ,
    departure_airport ,
    arrival_airport ,
    status ,
    aircraft_id ,
    actual_departure ,
    actual_arrival 
    


)
VALUES (
$1, $2, $3, $4, $5, $6, $7, $8, $9,$10
)
RETURNING flight_id, flight_no, company_id, scheduled_departure, scheduled_arrival, departure_airport, arrival_airport, status, aircraft_id, actual_departure, actual_arrival
`

type CreateFlightParams struct {
	FlightNo           string       `json:"flight_no"`
	CompanyID          int64        `json:"company_id"`
	ScheduledDeparture time.Time    `json:"scheduled_departure"`
	ScheduledArrival   time.Time    `json:"scheduled_arrival"`
	DepartureAirport   string       `json:"departure_airport"`
	ArrivalAirport     string       `json:"arrival_airport"`
	Status             string       `json:"status"`
	AircraftID         int64        `json:"aircraft_id"`
	ActualDeparture    sql.NullTime `json:"actual_departure"`
	ActualArrival      sql.NullTime `json:"actual_arrival"`
}

func (q *Queries) CreateFlight(ctx context.Context, arg CreateFlightParams) (Flight, error) {
	row := q.db.QueryRow(ctx, createFlight,
		arg.FlightNo,
		arg.CompanyID,
		arg.ScheduledDeparture,
		arg.ScheduledArrival,
		arg.DepartureAirport,
		arg.ArrivalAirport,
		arg.Status,
		arg.AircraftID,
		arg.ActualDeparture,
		arg.ActualArrival,
	)
	var i Flight
	err := row.Scan(
		&i.FlightID,
		&i.FlightNo,
		&i.CompanyID,
		&i.ScheduledDeparture,
		&i.ScheduledArrival,
		&i.DepartureAirport,
		&i.ArrivalAirport,
		&i.Status,
		&i.AircraftID,
		&i.ActualDeparture,
		&i.ActualArrival,
	)
	return i, err
}

const getFlight = `-- name: GetFlight :many
SELECT f.flight_id, f.flight_no, f.company_id, f.scheduled_departure, f.scheduled_departure_local, f.scheduled_arrival, f.scheduled_arrival_local, f.scheduled_duration, f.departure_airport, f.departure_airport_name, f.departure_city, f.arrival_airport, f.arrival_airport_name, f.arrival_city, f.status, f.aircraft_id, f.actual_departure, f.actual_departure_local, f.actual_arrival, f.actual_arrival_local, f.actual_duration 
FROM flights_v f 
WHERE f.departure_airport = $1
AND f.arrival_airport = $2
AND f.scheduled_departure > now() 
ORDER BY f.scheduled_departure LIMIT 1
`

type GetFlightParams struct {
	DepartureAirport string `json:"departure_airport"`
	ArrivalAirport   string `json:"arrival_airport"`
}

func (q *Queries) GetFlight(ctx context.Context, arg GetFlightParams) ([]FlightsV, error) {
	rows, err := q.db.Query(ctx, getFlight, arg.DepartureAirport, arg.ArrivalAirport)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []FlightsV{}
	for rows.Next() {
		var i FlightsV
		if err := rows.Scan(
			&i.FlightID,
			&i.FlightNo,
			&i.CompanyID,
			&i.ScheduledDeparture,
			&i.ScheduledDepartureLocal,
			&i.ScheduledArrival,
			&i.ScheduledArrivalLocal,
			&i.ScheduledDuration,
			&i.DepartureAirport,
			&i.DepartureAirportName,
			&i.DepartureCity,
			&i.ArrivalAirport,
			&i.ArrivalAirportName,
			&i.ArrivalCity,
			&i.Status,
			&i.AircraftID,
			&i.ActualDeparture,
			&i.ActualDepartureLocal,
			&i.ActualArrival,
			&i.ActualArrivalLocal,
			&i.ActualDuration,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
