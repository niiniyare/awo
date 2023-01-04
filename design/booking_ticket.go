package flightapi

import (
	"context"
	"log"

	bookingticket "github.com/niiniyare/awo/design/gen/booking_ticket"
)

// booking_ticket service example implementation.
// The example methods log the requests and return zero values.
type bookingTicketsrvc struct {
	logger *log.Logger
}

// NewBookingTicket returns the booking_ticket service implementation.
func NewBookingTicket(logger *log.Logger) bookingticket.Service {
	return &bookingTicketsrvc{logger}
}

// Book a flight using IATA NDC standard
func (s *bookingTicketsrvc) Book(ctx context.Context, p *bookingticket.BookingFlightPrams) (res *bookingticket.FlightBooking, err error) {
	res = &bookingticket.FlightBooking{}
	s.logger.Print("bookingTicket.book")
	return
}
