package main

import (
	"database/sql"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	app_books "flight/internal/books"
	"flight/internal/server"
	pb_books "flight/proto/books"
)

func registerServer(logger *zap.Logger, db *sql.DB) server.RegisterServer {
	return func(grpcServer *grpc.Server) {
		pb_books.RegisterBooksServiceServer(grpcServer, app_books.NewService(logger, app_books.New(db)))

	}
}

func registerHandlers() []server.RegisterHandler {
	var handlers []server.RegisterHandler

	handlers = append(handlers, pb_books.RegisterBooksServiceHandler)

	return handlers
}