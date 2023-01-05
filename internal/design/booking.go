package design

import (
	"context"
	"log"

	booking "github.com/niiniyare/awo/design/gen/booking"
)

// booking service example implementation.
// The example methods log the requests and return zero values.
type bookingsrvc struct {
	logger *log.Logger
}

// NewBooking returns the booking service implementation.
func NewBooking(logger *log.Logger) booking.Service {
	return &bookingsrvc{logger}
}

// Book a flight using IATA NDC standard
func (s *bookingsrvc) Book(ctx context.Context, p *booking.BookingFlightPrams) (res *booking.FlightBooking, err error) {
	res = &booking.FlightBooking{}
	s.logger.Print("booking.book")
	return
}
