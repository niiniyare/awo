package main

import (
	"database/sql"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	app_db "flight/db"
	"flight/internal/server"
	pb_db "flight/proto/db"
)

func registerServer(logger *zap.Logger, db *sql.DB) server.RegisterServer {
	return func(grpcServer *grpc.Server) {
		pb_db.RegisterDbServiceServer(grpcServer, app_db.NewService(logger, app_db.New(db)))

	}
}

func registerHandlers() []server.RegisterHandler {
	var handlers []server.RegisterHandler

	handlers = append(handlers, pb_db.RegisterDbServiceHandler)

	return handlers
}
