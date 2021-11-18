package service

import (
	"context"
	"time"
)

type AirlineCompany struct {
	CompanyID   int64     `json:"company_id"`
	CompanyName string    `json:"company_name"`
	IataCode    string    `json:"iata_code"`
	MainAirport string    `json:"main_airport"`
	CreatedAt   time.Time `json:"created_at"`
}

// AirlineCompanyService describes the service.
type AirlineCompanyService interface {
	// Add your methods here
	CreateAirlineCompany(ctx context.Context, IataCode string, CompanyName string, MainAirport string) (AirlineCompany, error)
	DeleteAirlineCompany(ctx context.Context, companyID int64) error
	GetAirlineCompany(ctx context.Context, companyID int64) (AirlineCompany, error)
	ListAirlineCompany(ctx context.Context) ([]AirlineCompany, error)
}

type basicAirlineCompanyService struct{}

func (b *basicAirlineCompanyService) CreateAirlineCompany(ctx context.Context, IataCode string, CompanyName string, MainAirport string) (a0 AirlineCompany, e1 error) {
	// TODO implement the business logic of CreateAirlineCompany
	return a0, e1
}
func (b *basicAirlineCompanyService) DeleteAirlineCompany(ctx context.Context, companyID int64) (e0 error) {
	// TODO implement the business logic of DeleteAirlineCompany
	return e0
}
func (b *basicAirlineCompanyService) GetAirlineCompany(ctx context.Context, companyID int64) (a0 AirlineCompany, e1 error) {
	// TODO implement the business logic of GetAirlineCompany
	return a0, e1
}
func (b *basicAirlineCompanyService) ListAirlineCompany(ctx context.Context) (a0 []AirlineCompany, e1 error) {
	// TODO implement the business logic of ListAirlineCompany
	return a0, e1
}

// NewBasicAirlineCompanyService returns a naive, stateless implementation of AirlineCompanyService.
func NewBasicAirlineCompanyService() AirlineCompanyService {
	return &basicAirlineCompanyService{}
}

// New returns a AirlineCompanyService with all of the expected middleware wired in.
func New(middleware []Middleware) AirlineCompanyService {
	var svc AirlineCompanyService = NewBasicAirlineCompanyService()
	for _, m := range middleware {
		svc = m(svc)
	}
	return svc
}
