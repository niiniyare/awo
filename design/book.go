package design

import (
	// _ "goa.design/goa/v3/codegen/generator"
	. "goa.design/goa/v3/dsl"
)

var _ = Service("booking", func() {

	Method("book", func() {
		Description("Book a flight using IATA NDC standard")
		Payload(BookingFlightPrams)
		Result(FlightBooking)
		HTTP(func() {
			POST("/book")

		})
		Error("NotFound")
		Error("BadRequest")
		HTTP(func() {
			GET("")
			Response(StatusOK)

		})

	})
})
