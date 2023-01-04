// Code generated by goa v3.10.2, DO NOT EDIT.
//
// flight views
//
// Command:
// $ goa gen github.com/niiniyare/awo/design -o design

package views

import (
	goa "goa.design/goa/v3/pkg"
)

// FlightresultCollection is the viewed result type that is projected based on
// a view.
type FlightresultCollection struct {
	// Type to project
	Projected FlightresultCollectionView
	// View to render
	View string
}

// FlightresultCollectionView is a type that runs validations on a projected
// type.
type FlightresultCollectionView []*FlightresultView

// FlightresultView is a type that runs validations on a projected type.
type FlightresultView struct {
	// Unique Flight Booking Identifier
	ID *int
	// Origin Airport Code
	Origin *string
	// Destination Airport Code
	Destination *string
	// Departure Date (YYYY-MM-DD)
	DepartureDate *string
	// Return Date (YYYY-MM-DD)
	ReturnDate *string
	// Status of the booking (pending, confirmed, cancelled)
	Status *string
}

var (
	// FlightresultCollectionMap is a map indexing the attribute names of
	// FlightresultCollection by view name.
	FlightresultCollectionMap = map[string][]string{
		"default": {
			"id",
			"origin",
			"destination",
			"departure_date",
			"return_date",
		},
	}
	// FlightresultMap is a map indexing the attribute names of Flightresult by
	// view name.
	FlightresultMap = map[string][]string{
		"default": {
			"id",
			"origin",
			"destination",
			"departure_date",
			"return_date",
		},
	}
)

// ValidateFlightresultCollection runs the validations defined on the viewed
// result type FlightresultCollection.
func ValidateFlightresultCollection(result FlightresultCollection) (err error) {
	switch result.View {
	case "default", "":
		err = ValidateFlightresultCollectionView(result.Projected)
	default:
		err = goa.InvalidEnumValueError("view", result.View, []interface{}{"default"})
	}
	return
}

// ValidateFlightresultCollectionView runs the validations defined on
// FlightresultCollectionView using the "default" view.
func ValidateFlightresultCollectionView(result FlightresultCollectionView) (err error) {
	for _, item := range result {
		if err2 := ValidateFlightresultView(item); err2 != nil {
			err = goa.MergeErrors(err, err2)
		}
	}
	return
}

// ValidateFlightresultView runs the validations defined on FlightresultView
// using the "default" view.
func ValidateFlightresultView(result *FlightresultView) (err error) {
	if result.ID == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("id", "result"))
	}
	if result.Origin == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("origin", "result"))
	}
	if result.Destination == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("destination", "result"))
	}
	if result.DepartureDate == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("departure_date", "result"))
	}
	if result.ReturnDate == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("return_date", "result"))
	}
	return
}