package http

import (
	"context"
	"encoding/json"
	"errors"
	endpoint "github.com/Abdirahman0022/airline_company/pkg/endpoint"
	http1 "github.com/go-kit/kit/transport/http"
	"net/http"
)

// makeCreateAirlineCompanyHandler creates the handler logic
func makeCreateAirlineCompanyHandler(m *http.ServeMux, endpoints endpoint.Endpoints, options []http1.ServerOption) {
	m.Handle("/create-airline-company", http1.NewServer(endpoints.CreateAirlineCompanyEndpoint, decodeCreateAirlineCompanyRequest, encodeCreateAirlineCompanyResponse, options...))
}

// decodeCreateAirlineCompanyRequest is a transport/http.DecodeRequestFunc that decodes a
// JSON-encoded request from the HTTP request body.
func decodeCreateAirlineCompanyRequest(_ context.Context, r *http.Request) (interface{}, error) {
	req := endpoint.CreateAirlineCompanyRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
}

// encodeCreateAirlineCompanyResponse is a transport/http.EncodeResponseFunc that encodes
// the response as JSON to the response writer
func encodeCreateAirlineCompanyResponse(ctx context.Context, w http.ResponseWriter, response interface{}) (err error) {
	if f, ok := response.(endpoint.Failure); ok && f.Failed() != nil {
		ErrorEncoder(ctx, f.Failed(), w)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(response)
	return
}

// makeDeleteAirlineCompanyHandler creates the handler logic
func makeDeleteAirlineCompanyHandler(m *http.ServeMux, endpoints endpoint.Endpoints, options []http1.ServerOption) {
	m.Handle("/delete-airline-company", http1.NewServer(endpoints.DeleteAirlineCompanyEndpoint, decodeDeleteAirlineCompanyRequest, encodeDeleteAirlineCompanyResponse, options...))
}

// decodeDeleteAirlineCompanyRequest is a transport/http.DecodeRequestFunc that decodes a
// JSON-encoded request from the HTTP request body.
func decodeDeleteAirlineCompanyRequest(_ context.Context, r *http.Request) (interface{}, error) {
	req := endpoint.DeleteAirlineCompanyRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
}

// encodeDeleteAirlineCompanyResponse is a transport/http.EncodeResponseFunc that encodes
// the response as JSON to the response writer
func encodeDeleteAirlineCompanyResponse(ctx context.Context, w http.ResponseWriter, response interface{}) (err error) {
	if f, ok := response.(endpoint.Failure); ok && f.Failed() != nil {
		ErrorEncoder(ctx, f.Failed(), w)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(response)
	return
}

// makeGetAirlineCompanyHandler creates the handler logic
func makeGetAirlineCompanyHandler(m *http.ServeMux, endpoints endpoint.Endpoints, options []http1.ServerOption) {
	m.Handle("/get-airline-company", http1.NewServer(endpoints.GetAirlineCompanyEndpoint, decodeGetAirlineCompanyRequest, encodeGetAirlineCompanyResponse, options...))
}

// decodeGetAirlineCompanyRequest is a transport/http.DecodeRequestFunc that decodes a
// JSON-encoded request from the HTTP request body.
func decodeGetAirlineCompanyRequest(_ context.Context, r *http.Request) (interface{}, error) {
	req := endpoint.GetAirlineCompanyRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
}

// encodeGetAirlineCompanyResponse is a transport/http.EncodeResponseFunc that encodes
// the response as JSON to the response writer
func encodeGetAirlineCompanyResponse(ctx context.Context, w http.ResponseWriter, response interface{}) (err error) {
	if f, ok := response.(endpoint.Failure); ok && f.Failed() != nil {
		ErrorEncoder(ctx, f.Failed(), w)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(response)
	return
}

// makeListAirlineCompanyHandler creates the handler logic
func makeListAirlineCompanyHandler(m *http.ServeMux, endpoints endpoint.Endpoints, options []http1.ServerOption) {
	m.Handle("/list-airline-company", http1.NewServer(endpoints.ListAirlineCompanyEndpoint, decodeListAirlineCompanyRequest, encodeListAirlineCompanyResponse, options...))
}

// decodeListAirlineCompanyRequest is a transport/http.DecodeRequestFunc that decodes a
// JSON-encoded request from the HTTP request body.
func decodeListAirlineCompanyRequest(_ context.Context, r *http.Request) (interface{}, error) {
	req := endpoint.ListAirlineCompanyRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
}

// encodeListAirlineCompanyResponse is a transport/http.EncodeResponseFunc that encodes
// the response as JSON to the response writer
func encodeListAirlineCompanyResponse(ctx context.Context, w http.ResponseWriter, response interface{}) (err error) {
	if f, ok := response.(endpoint.Failure); ok && f.Failed() != nil {
		ErrorEncoder(ctx, f.Failed(), w)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(response)
	return
}
func ErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	w.WriteHeader(err2code(err))
	json.NewEncoder(w).Encode(errorWrapper{Error: err.Error()})
}
func ErrorDecoder(r *http.Response) error {
	var w errorWrapper
	if err := json.NewDecoder(r.Body).Decode(&w); err != nil {
		return err
	}
	return errors.New(w.Error)
}

// This is used to set the http status, see an example here :
// https://github.com/go-kit/kit/blob/master/examples/addsvc/pkg/addtransport/http.go#L133
func err2code(err error) int {
	return http.StatusInternalServerError
}

type errorWrapper struct {
	Error string `json:"error"`
}
