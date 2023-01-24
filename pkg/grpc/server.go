package grpc_Server

import (
	db "github.com/niiniyare/awo/db/sqlc"
	"github.com/niiniyare/awo/pkg/api/v1/pb"
	"github.com/niiniyare/awo/util"
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
