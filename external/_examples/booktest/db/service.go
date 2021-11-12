package db

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "flight/proto/db"
)

type Service struct {
	pb.UnimplementedDbServiceServer
	logger *zap.Logger
	db     *Queries
}

func NewService(logger *zap.Logger, db *Queries) *Service {
	return &Service{logger: logger, db: db}
}

func (s *Service) CreateAircraft(ctx context.Context, in *pb.CreateAircraftParams) (out *pb.AircraftsDatum, err error) {
	var arg CreateAircraftParams
	arg.AircraftCode = in.GetAircraftCode()
	arg.Model = pgtype(in.GetModel())
	arg.Range = in.GetRange()
	arg.CompanyID = in.GetCompanyID()

	result, err := s.db.CreateAircraft(ctx, arg)
	if err != nil {
		return
	}

	out = new(pb.AircraftsDatum)
	out.AircraftCode = result.AircraftCode
	out.Model = JSONB(result.Model)
	out.Range = result.Range
	out.CompanyID = result.CompanyID

	return
}

func (s *Service) CreateAirlineCompany(ctx context.Context, in *pb.CreateAirlineCompanyParams) (out *pb.AirlineCompany, err error) {
	var arg CreateAirlineCompanyParams
	arg.CompanyName = in.GetCompanyName()
	arg.IataCode = in.GetIataCode()
	arg.MainAirport = in.GetMainAirport()

	result, err := s.db.CreateAirlineCompany(ctx, arg)
	if err != nil {
		return
	}

	out = new(pb.AirlineCompany)
	out.CompanyID = result.CompanyID
	out.CompanyName = result.CompanyName
	out.IataCode = result.IataCode
	out.MainAirport = result.MainAirport

	return
}

func (s *Service) CreateAirportList(ctx context.Context, in *pb.CreateAirportListParams) (out *pb.CreateAirportListResponse, err error) {
	var arg CreateAirportListParams
	arg.AirportCode = in.GetAirportCode()
	arg.AirportName = in.GetAirportName()
	arg.City = in.GetCity()
	arg.Coordinates = pgtype(in.GetCoordinates())

	result, err := s.db.CreateAirportList(ctx, arg)
	if err != nil {
		return
	}

	out = new(pb.CreateAirportListResponse)
	for _, r := range result {
		var item pb.AirportsDatum
		item.AirportCode = r.AirportCode
		item.AirportName = r.AirportName
		item.City = r.City
		item.Coordinates = Numeric(r.Coordinates)
		item.Timezone = r.Timezone
		out.Value = append(out.Value, &item)
	}

	return
}

func (s *Service) CreateAirports(ctx context.Context, in *pb.CreateAirportsParams) (out *pb.AirportsDatum, err error) {
	var arg CreateAirportsParams
	arg.AirportCode = in.GetAirportCode()
	arg.AirportName = in.GetAirportName()
	arg.City = in.GetCity()
	arg.Coordinates = pgtype(in.GetCoordinates())

	result, err := s.db.CreateAirports(ctx, arg)
	if err != nil {
		return
	}

	out = new(pb.AirportsDatum)
	out.AirportCode = result.AirportCode
	out.AirportName = result.AirportName
	out.City = result.City
	out.Coordinates = Numeric(result.Coordinates)
	out.Timezone = result.Timezone

	return
}

func (s *Service) DeleteAircraft(ctx context.Context, in *pb.DeleteAircraftParams) (out *emptypb.Empty, err error) {
	aircraftCode := in.GetAircraftCode()

	err = s.db.DeleteAircraft(ctx, aircraftCode)
	if err != nil {
		return
	}

	out = new(emptypb.Empty)

	return
}

func (s *Service) DeleteAirlineCompany(ctx context.Context, in *pb.DeleteAirlineCompanyParams) (out *emptypb.Empty, err error) {
	companyID := in.GetCompanyID()

	err = s.db.DeleteAirlineCompany(ctx, companyID)
	if err != nil {
		return
	}

	out = new(emptypb.Empty)

	return
}

func (s *Service) DeleteAirports(ctx context.Context, in *pb.DeleteAirportsParams) (out *emptypb.Empty, err error) {
	airportCode := in.GetAirportCode()

	err = s.db.DeleteAirports(ctx, airportCode)
	if err != nil {
		return
	}

	out = new(emptypb.Empty)

	return
}

func (s *Service) GetAircraft(ctx context.Context, in *pb.GetAircraftParams) (out *pb.AircraftsDatum, err error) {
	aircraftCode := in.GetAircraftCode()

	result, err := s.db.GetAircraft(ctx, aircraftCode)
	if err != nil {
		return
	}

	out = new(pb.AircraftsDatum)
	out.AircraftCode = result.AircraftCode
	out.Model = JSONB(result.Model)
	out.Range = result.Range
	out.CompanyID = result.CompanyID

	return
}

func (s *Service) GetAirlineCompany(ctx context.Context, in *pb.GetAirlineCompanyParams) (out *pb.AirlineCompany, err error) {
	companyID := in.GetCompanyID()

	result, err := s.db.GetAirlineCompany(ctx, companyID)
	if err != nil {
		return
	}

	out = new(pb.AirlineCompany)
	out.CompanyID = result.CompanyID
	out.CompanyName = result.CompanyName
	out.IataCode = result.IataCode
	out.MainAirport = result.MainAirport

	return
}

func (s *Service) GetAirports(ctx context.Context, in *pb.GetAirportsParams) (out *pb.AirportsDatum, err error) {
	airportCode := in.GetAirportCode()

	result, err := s.db.GetAirports(ctx, airportCode)
	if err != nil {
		return
	}

	out = new(pb.AirportsDatum)
	out.AirportCode = result.AirportCode
	out.AirportName = result.AirportName
	out.City = result.City
	out.Coordinates = Numeric(result.Coordinates)
	out.Timezone = result.Timezone

	return
}

func (s *Service) ListAircraft(ctx context.Context, in *emptypb.Empty) (out *pb.ListAircraftResponse, err error) {

	result, err := s.db.ListAircraft(ctx)
	if err != nil {
		return
	}

	out = new(pb.ListAircraftResponse)
	for _, r := range result {
		var item pb.AircraftsDatum
		item.AircraftCode = r.AircraftCode
		item.Model = JSONB(r.Model)
		item.Range = r.Range
		item.CompanyID = r.CompanyID
		out.Value = append(out.Value, &item)
	}

	return
}

func (s *Service) ListAirlineCompany(ctx context.Context, in *emptypb.Empty) (out *pb.ListAirlineCompanyResponse, err error) {

	result, err := s.db.ListAirlineCompany(ctx)
	if err != nil {
		return
	}

	out = new(pb.ListAirlineCompanyResponse)
	for _, r := range result {
		var item pb.AirlineCompany
		item.CompanyID = r.CompanyID
		item.CompanyName = r.CompanyName
		item.IataCode = r.IataCode
		item.MainAirport = r.MainAirport
		out.Value = append(out.Value, &item)
	}

	return
}

func (s *Service) ListAirports(ctx context.Context, in *emptypb.Empty) (out *pb.ListAirportsResponse, err error) {

	result, err := s.db.ListAirports(ctx)
	if err != nil {
		return
	}

	out = new(pb.ListAirportsResponse)
	for _, r := range result {
		var item pb.ListAirportsRow
		item.AirportCode = r.AirportCode
		item.AirportName = r.AirportName
		item.City = r.City
		out.Value = append(out.Value, &item)
	}

	return
}
