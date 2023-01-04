package design

import (
	// _ "goa.design/goa/v3/codegen/generator"
	. "goa.design/goa/v3/dsl"
)

var FlightBooking = Type("FlightBooking", func() {
	Attribute("id", Int, "Unique Flight Booking Identifier")
	Attribute("origin", String, "Origin Airport Code")
	Attribute("destination", String, "Destination Airport Code")
	Attribute("departure_date", String, "Departure Date (YYYY-MM-DD)")
	Attribute("return_date", String, "Return Date (YYYY-MM-DD)")
	Attribute("status", String, "Status of the booking (pending, confirmed, cancelled)")
	// Attribute("ancillaries", IATADNCAncillaryResponse, "IATA NDC Ancillary Response")
	Required("id", "origin", "destination", "departure_date", "return_date", "status")
})

var FlightAvailability = Type("FlightAvailability", func() {
	Attribute("origin", String, "Origin airport code")
	Attribute("destination", String, "Destination airport code")
	Attribute("departure_date", String, "Departure date and time")
	Attribute("return_date", String, "Return date and time (for round trip searches)")
	Attribute("price", Float64, "Price of the flight")
	Attribute("airline", String, "Airline operating the flight")
})
