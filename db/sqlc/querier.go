// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.14.0

package db

import (
	"context"
)

type Querier interface {
	CreateAircraft(ctx context.Context, arg CreateAircraftParams) (Aircraft, error)
	CreateAirline(ctx context.Context, arg CreateAirlineParams) (Airline, error)
	//subdivision_code
	CreateAirport(ctx context.Context, arg CreateAirportParams) (Airport, error)
	CreateFlight(ctx context.Context, arg CreateFlightParams) (Flight, error)
	CreateSeat(ctx context.Context, arg CreateSeatParams) (Seat, error)
	DeleteAircraft(ctx context.Context, id int64) error
	DeleteAirline(ctx context.Context, id int64) error
	DeleteAirports(ctx context.Context, iataCode string) (Airport, error)
	// SELECT f.flight_id, f.flight_no, f.company_id, f.scheduled_departure, f.scheduled_departure_local, f.scheduled_arrival, f.scheduled_arrival_local, f.scheduled_duration, f.departure_airport, f.departure_airport_name, f.departure_city, f.arrival_airport, f.arrival_airport_name, f.arrival_city, f.status, f.aircraft_id, f.actual_departure, f.actual_departure_local, f.actual_arrival, f.actual_arrival_local, COALESCE(f.actual_duration ,0) as  actual_duration
	// FROM flights_v f
	// WHERE f.departure_airport = $1
	// AND f.arrival_airport = $2
	// AND f.scheduled_departure > now()
	// AND f.company_id = $3
	// ORDER BY f.scheduled_departure LIMIT 2
	//
	FlightAvailability(ctx context.Context, arg FlightAvailabilityParams) ([]FlightsV, error)
	GetAircraft(ctx context.Context, id int64) (Aircraft, error)
	// iata_code,
	// icao_code,
	// model,
	// range,
	// company_id
	GetAircraftSeatsWithIataCode(ctx context.Context, iataCode string) ([]GetAircraftSeatsWithIataCodeRow, error)
	GetAirline(ctx context.Context, id int64) (Airline, error)
	GetAirport(ctx context.Context, iataCode string) (Airport, error)
	GetSeats(ctx context.Context) ([]Seat, error)
	InsertNewSeatMap(ctx context.Context, arg []InsertNewSeatMapParams) (int64, error)
	ListAircraft(ctx context.Context, arg ListAircraftParams) ([]Aircraft, error)
	ListAirline(ctx context.Context, arg ListAirlineParams) ([]Airline, error)
	ListAirport(ctx context.Context, arg ListAirportParams) ([]ListAirportRow, error)
}

var _ Querier = (*Queries)(nil)
