// Code generated by goa v3.10.2, DO NOT EDIT.
//
// flight HTTP server encoders and decoders
//
// Command:
// $ goa gen github.com/niiniyare/awo/design -o design

package server

import (
	"context"
	"io"
	"net/http"

	flightviews "github.com/niiniyare/awo/design/gen/flight/views"
	goahttp "goa.design/goa/v3/http"
	goa "goa.design/goa/v3/pkg"
)

// EncodeSearchResponse returns an encoder for responses returned by the flight
// search endpoint.
func EncodeSearchResponse(encoder func(context.Context, http.ResponseWriter) goahttp.Encoder) func(context.Context, http.ResponseWriter, interface{}) error {
	return func(ctx context.Context, w http.ResponseWriter, v interface{}) error {
		res := v.(flightviews.FlightresultCollection)
		enc := encoder(ctx, w)
		body := NewFlightresultResponseCollection(res.Projected)
		w.WriteHeader(http.StatusOK)
		return enc.Encode(body)
	}
}

// DecodeSearchRequest returns a decoder for requests sent to the flight search
// endpoint.
func DecodeSearchRequest(mux goahttp.Muxer, decoder func(*http.Request) goahttp.Decoder) func(*http.Request) (interface{}, error) {
	return func(r *http.Request) (interface{}, error) {
		var (
			body SearchRequestBody
			err  error
		)
		err = decoder(r).Decode(&body)
		if err != nil {
			if err == io.EOF {
				return nil, goa.MissingPayloadError()
			}
			return nil, goa.DecodePayloadError(err.Error())
		}
		err = ValidateSearchRequestBody(&body)
		if err != nil {
			return nil, err
		}
		payload := NewSearchFlightSearchRequestPrams(&body)

		return payload, nil
	}
}

// marshalFlightviewsFlightresultViewToFlightresultResponse builds a value of
// type *FlightresultResponse from a value of type
// *flightviews.FlightresultView.
func marshalFlightviewsFlightresultViewToFlightresultResponse(v *flightviews.FlightresultView) *FlightresultResponse {
	res := &FlightresultResponse{
		ID:            *v.ID,
		Origin:        *v.Origin,
		Destination:   *v.Destination,
		DepartureDate: *v.DepartureDate,
		ReturnDate:    *v.ReturnDate,
	}

	return res
}