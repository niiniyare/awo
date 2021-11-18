package endpoint

import (
	"context"

	service "github.com/Abdirahman0022/airline_company/pkg/service"
	endpoint "github.com/go-kit/kit/endpoint"
)

// CreateAirlineCompanyRequest collects the request parameters for the CreateAirlineCompany method.
type CreateAirlineCompanyRequest struct {
	IataCode    string `json:"iata_code"`
	CompanyName string `json:"company_name"`
	MainAirport string `json:"main_airport"`
}

// CreateAirlineCompanyResponse collects the response parameters for the CreateAirlineCompany method.
type CreateAirlineCompanyResponse struct {
	A0 service.AirlineCompany `json:"a0"`
	E1 error                  `json:"e1"`
}

// MakeCreateAirlineCompanyEndpoint returns an endpoint that invokes CreateAirlineCompany on the service.
func MakeCreateAirlineCompanyEndpoint(s service.AirlineCompanyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(CreateAirlineCompanyRequest)
		a0, e1 := s.CreateAirlineCompany(ctx, req.IataCode, req.CompanyName, req.MainAirport)
		return CreateAirlineCompanyResponse{
			A0: a0,
			E1: e1,
		}, nil
	}
}

// Failed implements Failer.
func (r CreateAirlineCompanyResponse) Failed() error {
	return r.E1
}

// DeleteAirlineCompanyRequest collects the request parameters for the DeleteAirlineCompany method.
type DeleteAirlineCompanyRequest struct {
	CompanyID int64 `json:"company_id"`
}

// DeleteAirlineCompanyResponse collects the response parameters for the DeleteAirlineCompany method.
type DeleteAirlineCompanyResponse struct {
	E0 error `json:"e0"`
}

// MakeDeleteAirlineCompanyEndpoint returns an endpoint that invokes DeleteAirlineCompany on the service.
func MakeDeleteAirlineCompanyEndpoint(s service.AirlineCompanyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(DeleteAirlineCompanyRequest)
		e0 := s.DeleteAirlineCompany(ctx, req.CompanyID)
		return DeleteAirlineCompanyResponse{E0: e0}, nil
	}
}

// Failed implements Failer.
func (r DeleteAirlineCompanyResponse) Failed() error {
	return r.E0
}

// GetAirlineCompanyRequest collects the request parameters for the GetAirlineCompany method.
type GetAirlineCompanyRequest struct {
	CompanyID int64 `json:"company_id"`
}

// GetAirlineCompanyResponse collects the response parameters for the GetAirlineCompany method.
type GetAirlineCompanyResponse struct {
	A0 service.AirlineCompany `json:"a0"`
	E1 error                  `json:"e1"`
}

// MakeGetAirlineCompanyEndpoint returns an endpoint that invokes GetAirlineCompany on the service.
func MakeGetAirlineCompanyEndpoint(s service.AirlineCompanyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(GetAirlineCompanyRequest)
		a0, e1 := s.GetAirlineCompany(ctx, req.CompanyID)
		return GetAirlineCompanyResponse{
			A0: a0,
			E1: e1,
		}, nil
	}
}

// Failed implements Failer.
func (r GetAirlineCompanyResponse) Failed() error {
	return r.E1
}

// ListAirlineCompanyRequest collects the request parameters for the ListAirlineCompany method.
type ListAirlineCompanyRequest struct{}

// ListAirlineCompanyResponse collects the response parameters for the ListAirlineCompany method.
type ListAirlineCompanyResponse struct {
	A0 []AirlineCompany `json:"a0"`
	E1 error            `json:"e1"`
}

// MakeListAirlineCompanyEndpoint returns an endpoint that invokes ListAirlineCompany on the service.
func MakeListAirlineCompanyEndpoint(s service.AirlineCompanyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		a0, e1 := s.ListAirlineCompany(ctx)
		return ListAirlineCompanyResponse{
			A0: a0,
			E1: e1,
		}, nil
	}
}

// Failed implements Failer.
func (r ListAirlineCompanyResponse) Failed() error {
	return r.E1
}

// Failure is an interface that should be implemented by response types.
// Response encoders can check if responses are Failer, and if so they've
// failed, and if so encode them using a separate write path based on the error.
type Failure interface {
	Failed() error
}

// CreateAirlineCompany implements Service. Primarily useful in a client.
func (e Endpoints) CreateAirlineCompany(ctx context.Context, IataCode string, CompanyName string, MainAirport string) (a0 service.AirlineCompany, e1 error) {
	request := CreateAirlineCompanyRequest{
		CompanyName: CompanyName,
		IataCode:    IataCode,
		MainAirport: MainAirport,
	}
	response, err := e.CreateAirlineCompanyEndpoint(ctx, request)
	if err != nil {
		return
	}
	return response.(CreateAirlineCompanyResponse).A0, response.(CreateAirlineCompanyResponse).E1
}

// DeleteAirlineCompany implements Service. Primarily useful in a client.
func (e Endpoints) DeleteAirlineCompany(ctx context.Context, companyID int64) (e0 error) {
	request := DeleteAirlineCompanyRequest{CompanyID: companyID}
	response, err := e.DeleteAirlineCompanyEndpoint(ctx, request)
	if err != nil {
		return
	}
	return response.(DeleteAirlineCompanyResponse).E0
}

// GetAirlineCompany implements Service. Primarily useful in a client.
func (e Endpoints) GetAirlineCompany(ctx context.Context, companyID int64) (a0 service.AirlineCompany, e1 error) {
	request := GetAirlineCompanyRequest{CompanyID: companyID}
	response, err := e.GetAirlineCompanyEndpoint(ctx, request)
	if err != nil {
		return
	}
	return response.(GetAirlineCompanyResponse).A0, response.(GetAirlineCompanyResponse).E1
}

// ListAirlineCompany implements Service. Primarily useful in a client.
func (e Endpoints) ListAirlineCompany(ctx context.Context) (a0 []AirlineCompany, e1 error) {
	request := ListAirlineCompanyRequest{}
	response, err := e.ListAirlineCompanyEndpoint(ctx, request)
	if err != nil {
		return
	}
	return response.(ListAirlineCompanyResponse).A0, response.(ListAirlineCompanyResponse).E1
}
