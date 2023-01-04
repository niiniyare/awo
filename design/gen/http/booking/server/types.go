// Code generated by goa v3.10.2, DO NOT EDIT.
//
// booking HTTP server types
//
// Command:
// $ goa gen github.com/niiniyare/awo/design -o design

package server

import (
	"unicode/utf8"

	booking "github.com/niiniyare/awo/design/gen/booking"
	goa "goa.design/goa/v3/pkg"
)

// BookRequestBody is the type of the "booking" service "book" endpoint HTTP
// request body.
type BookRequestBody struct {
	// Origin airport code
	Origin *string `form:"origin,omitempty" json:"origin,omitempty" xml:"origin,omitempty"`
	// Destination airport code
	Destination *string `form:"destination,omitempty" json:"destination,omitempty" xml:"destination,omitempty"`
	// Departure date and time
	DepartureDate *string `form:"departure_date,omitempty" json:"departure_date,omitempty" xml:"departure_date,omitempty"`
	// Return date and time (for round trip searches)
	ReturnDate *string `form:"return_date,omitempty" json:"return_date,omitempty" xml:"return_date,omitempty"`
	// True for one-way searches, false for round trip searches
	OneWay *bool `form:"one_way,omitempty" json:"one_way,omitempty" xml:"one_way,omitempty"`
}

// BookResponseBody is the type of the "booking" service "book" endpoint HTTP
// response body.
type BookResponseBody struct {
	// Unique Flight Booking Identifier
	ID int `form:"id" json:"id" xml:"id"`
	// Origin Airport Code
	Origin string `form:"origin" json:"origin" xml:"origin"`
	// Destination Airport Code
	Destination string `form:"destination" json:"destination" xml:"destination"`
	// Departure Date (YYYY-MM-DD)
	DepartureDate string `form:"departure_date" json:"departure_date" xml:"departure_date"`
	// Return Date (YYYY-MM-DD)
	ReturnDate string `form:"return_date" json:"return_date" xml:"return_date"`
	// Status of the booking (pending, confirmed, cancelled)
	Status string `form:"status" json:"status" xml:"status"`
}

// NewBookResponseBody builds the HTTP response body from the result of the
// "book" endpoint of the "booking" service.
func NewBookResponseBody(res *booking.FlightBooking) *BookResponseBody {
	body := &BookResponseBody{
		ID:            res.ID,
		Origin:        res.Origin,
		Destination:   res.Destination,
		DepartureDate: res.DepartureDate,
		ReturnDate:    res.ReturnDate,
		Status:        res.Status,
	}
	return body
}

// NewBookingFlightPrams builds a booking service book endpoint payload.
func NewBookingFlightPrams(body *BookRequestBody) *booking.BookingFlightPrams {
	v := &booking.BookingFlightPrams{
		Origin:        *body.Origin,
		Destination:   *body.Destination,
		DepartureDate: *body.DepartureDate,
		ReturnDate:    body.ReturnDate,
		OneWay:        *body.OneWay,
	}

	return v
}

// ValidateBookRequestBody runs the validations defined on BookRequestBody
func ValidateBookRequestBody(body *BookRequestBody) (err error) {
	if body.Origin == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("origin", "body"))
	}
	if body.Destination == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("destination", "body"))
	}
	if body.DepartureDate == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("departure_date", "body"))
	}
	if body.OneWay == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("one_way", "body"))
	}
	if body.Origin != nil {
		if utf8.RuneCountInString(*body.Origin) < 3 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.origin", *body.Origin, utf8.RuneCountInString(*body.Origin), 3, true))
		}
	}
	if body.Origin != nil {
		if utf8.RuneCountInString(*body.Origin) > 3 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.origin", *body.Origin, utf8.RuneCountInString(*body.Origin), 3, false))
		}
	}
	if body.Destination != nil {
		if utf8.RuneCountInString(*body.Destination) < 3 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.destination", *body.Destination, utf8.RuneCountInString(*body.Destination), 3, true))
		}
	}
	if body.Destination != nil {
		if utf8.RuneCountInString(*body.Destination) > 3 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.destination", *body.Destination, utf8.RuneCountInString(*body.Destination), 3, false))
		}
	}
	return
}
