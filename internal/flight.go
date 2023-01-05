package flightapi

import (
	"context"
	"log"

	flight "github.com/niiniyare/awo/internal/gen/flight"
)

// flight service example implementation.
// The example methods log the requests and return zero values.
type flightsrvc struct {
	logger *log.Logger
}

// NewFlight returns the flight service implementation.
func NewFlight(logger *log.Logger) flight.Service {
	return &flightsrvc{logger}
}

// Search for available flights
func (s *flightsrvc) Search(ctx context.Context, p *flight.FlightSearchRequestPrams) (res flight.FlightresultCollection, err error) {
	s.logger.Print("flight.search")
	return
}
