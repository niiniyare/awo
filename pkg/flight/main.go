package main

import (
	"flight/handler"
	pb "flight/proto"

	"github.com/micro/micro/v3/service"
	"github.com/micro/micro/v3/service/logger"
)

func main() {
	// Create service
	srv := service.New(
		service.Name("flight"),
		service.Version("latest"),
	)

	// Register handler
	pb.RegisterFlightHandler(srv.Server(), new(handler.Flight))

	// Run service
	if err := srv.Run(); err != nil {
		logger.Fatal(err)
	}
}
