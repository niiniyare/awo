package grpc_Server

import (
	"context"

	db "github.com/niiniyare/awo/db/sqlc"
	"github.com/niiniyare/awo/pkg/api/v1/pb"
	"github.com/niiniyare/awo/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GRPCServer struct {
	pb.UnimplementedAirlinesServiceServer

	config util.Config
	store  db.Store
}

func NewGGRPCServer(config util.Config, store db.Store) (*GRPCServer, error) {
	server := &GRPCServer{
		config: config,
		store:  store,
	}
	return server, nil
}

func (s *GRPCServer) CreateAirline(ctx context.Context, req *pb.CreateAirlineRequest) (*pb.CreateAirlineResponse, error) {
	arg := db.CreateAirlineParams{
		CompanyName: req.GetName(),
		IataCode:    req.GetIataCode(),
		MainAirport: req.GetHub_Airport(),
	}
	airline, err := s.store.CreateAirline(ctx, arg)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to Create Airline err:"+err.Error())
	}
	res := &pb.CreateAirlineResponse{
		Id:          airline.ID,
		Name:        airline.CompanyName,
		IataCode:    airline.IataCode,
		IcaoCode:    airline.IcaoCode.String,
		Callsign:    airline.Callsign.String,
		Hub_Airport: airline.MainAirport,
		Created_At:  timestamppb.New(airline.CreatedAt),
		Updated_At:  timestamppb.New(airline.UpdatedAt),
	}
	return res, nil
}
