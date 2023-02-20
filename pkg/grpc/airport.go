package grpc_Server

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	db "github.com/niiniyare/awo/db/sqlc"
	"github.com/niiniyare/awo/pkg/api/v1/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// IataCode:
// IcaoCode:
// Name:
// Elevation:
// City    :
// Country :
// State   :
// Lat     :
// Lon     :
// Timezone:




func (s *GRPCServer) CreateAirport(ctx context.Context, req *pb.CreateAirportRequest) (*pb.CreateAirportResponse, error) {

  // elevation := 

	arg := db.CreateAirportParams{
IataCode: req.GetIataCode(),
IcaoCode: req.GetIcaoCode(),  
Name:    req.GetName() , 
Elevation: sql.NullString{
  String: fmt.Sprint("%v",req.GetElevation()), 
  Valid: true,
},
City    : req.GetCity(),
Country : req.GetCity(),
State   : req.GetState() ,
Lat     : fmt.Sprintf("%v",req.GetLat()),
Lon     : fmt.Sprintf("%v",req.GetLong()),
// Timezone: ,

// 		IataCode:    req.GetIataCode(),
	}
	airport, err := s.store.CreateAirport(ctx, arg)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to Create Airport err:"+err.Error())
	}
	res := &pb.CreateAirportResponse{
    Airport: &pb.Airport{
		Name:       airport.Name,
		IataCode:   airport.IataCode,
		IcaoCode:   airport.IcaoCode,
    City: airport.City,
    State: airport.State,
    CountryCode: airport.Country,
    // Elevation: airport.Elevation.Value(),

	},}
	return res, nil
}

func (s *GRPCServer) GetAirport(ctx context.Context, req *pb.GetAirportRequest) (*pb.GetAirportResponse, error) {
	airport, err := s.store.GetAirport(ctx, req.GetId())
	if err != nil {
		if err != pgx.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "Airport Company NotFound err: %s", err)

		}
		return nil, status.Errorf(codes.Internal, "Airport Company NotFound err: %s", err)

	}
	res := &pb.GetAirportResponse{
		Id:         airport.ID,
		Name:       airport.CompanyName,
		IataCode:   airport.IataCode,
		Created_At: timestamppb.New(airport.CreatedAt),
		Updated_At: timestamppb.New(airport.UpdatedAt),
	}

	return res, nil
}
func (s *GRPCServer) UpdateAirport(ctx context.Context, req *pb.UpdateAirportRequest) (*pb.UpdateAirportResponse, error) {

	return nil, status.Errorf(codes.Unimplemented, "method UpdateAirport not implemented")

}

func (s *GRPCServer) DeleteAirport(ctx context.Context, req *pb.DeleteAirportRequest) (*pb.DeleteAirportResponse, error) {

	return nil, status.Errorf(codes.Unimplemented, "method DeleteAirport not implemented")

}

func (s *GRPCServer) ListAirports(ctx context.Context, req *pb.ListAirportsReq) (*pb.ListAirportRes, error) {
	arg := db.ListAirportParams{
		Limit:  int32(req.GetLimit()),
		Offset: int32(req.Offset),
	}
	airports, err := s.store.ListAirport(ctx, arg)
	if err != nil {
		if err == pgx.ErrNoRows {

			return nil, status.Errorf(codes.NotFound, "ErrNoRows: %s", err)
		}
		return nil, status.Errorf(codes.Internal, "method ListAirports err: %s", err)
	}

	res := &pb.ListAirportResponse{}

	for _, airport := range airports {
		item := &pb.Airport{

			Name:       airport.Name,
			IataCode:   airport.IataCode,
			// Created_At: timestamppb.New(airport.CreatedAt),
			// Updated_At: timestamppb.New(airport.UpdatedAt),
		}

		res.Airports = append(res.Airports, item)

	}
	return res, nil
}
