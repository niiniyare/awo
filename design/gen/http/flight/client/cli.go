// Code generated by goa v3.10.2, DO NOT EDIT.
//
// flight HTTP client CLI support package
//
// Command:
// $ goa gen github.com/niiniyare/awo/design -o design

package client

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	flight "github.com/niiniyare/awo/design/gen/flight"
	goa "goa.design/goa/v3/pkg"
)

// BuildSearchPayload builds the payload for the flight search endpoint from
// CLI flags.
func BuildSearchPayload(flightSearchBody string) (*flight.FlightSearchRequestPrams, error) {
	var err error
	var body SearchRequestBody
	{
		err = json.Unmarshal([]byte(flightSearchBody), &body)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON for body, \nerror: %s, \nexample of valid JSON:\n%s", err, "'{\n      \"departure_date\": \"Magni enim ad.\",\n      \"destination\": \"bln\",\n      \"one_way\": false,\n      \"origin\": \"ju0\",\n      \"return_date\": \"Enim vitae corrupti illum vitae.\"\n   }'")
		}
		if utf8.RuneCountInString(body.Origin) < 3 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.origin", body.Origin, utf8.RuneCountInString(body.Origin), 3, true))
		}
		if utf8.RuneCountInString(body.Origin) > 3 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.origin", body.Origin, utf8.RuneCountInString(body.Origin), 3, false))
		}
		if utf8.RuneCountInString(body.Destination) < 3 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.destination", body.Destination, utf8.RuneCountInString(body.Destination), 3, true))
		}
		if utf8.RuneCountInString(body.Destination) > 3 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.destination", body.Destination, utf8.RuneCountInString(body.Destination), 3, false))
		}
		if err != nil {
			return nil, err
		}
	}
	v := &flight.FlightSearchRequestPrams{
		Origin:        body.Origin,
		Destination:   body.Destination,
		DepartureDate: body.DepartureDate,
		ReturnDate:    body.ReturnDate,
		OneWay:        body.OneWay,
	}

	return v, nil
}