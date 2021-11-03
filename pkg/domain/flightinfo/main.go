package main

import (
	"flightinfo/handler"
	pb "flightinfo/proto"

	"github.com/micro/micro/v3/service"
	"github.com/micro/micro/v3/service/logger"
)

func main() {
	// Create service
	srv := service.New(
		service.Name("flightinfo"),
		service.Version("latest"),
	)

	// Register handler
	pb.RegisterFlightinfoHandler(srv.Server(), new(handler.Flightinfo))

	// Run service
	if err := srv.Run(); err != nil {
		logger.Fatal(err)
	}
}
