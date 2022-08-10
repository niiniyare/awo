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
	//coordinates
	CreateAirport(ctx context.Context, arg CreateAirportParams) (Airport, error)
	DeleteAircraft(ctx context.Context, id int64) error
	DeleteAirline(ctx context.Context, id int64) error
	DeleteAirports(ctx context.Context, id int64) error
	GetAircraft(ctx context.Context, id int64) (Aircraft, error)
	GetAirline(ctx context.Context, id int64) (Airline, error)
	GetAirport(ctx context.Context, iataCode string) (Airport, error)
	ListAircraft(ctx context.Context, arg ListAircraftParams) ([]Aircraft, error)
	ListAirline(ctx context.Context, arg ListAirlineParams) ([]Airline, error)
	ListAirport(ctx context.Context) ([]ListAirportRow, error)
	createFlight(ctx context.Context, arg createFlightParams) (Flight, error)
}

var _ Querier = (*Queries)(nil)
