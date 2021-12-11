package handler

import (
	"context"

	log "github.com/micro/micro/v3/service/logger"

	flight "flight/proto"
)

type Flight struct{}

// Call is a single request handler called via client.Call or the generated client code
func (e *Flight) Call(ctx context.Context, req *flight.Request, rsp *flight.Response) error {
	log.Info("Received Flight.Call request")
	rsp.Msg = "Hello " + req.Name
	return nil
}

// Stream is a server side stream handler called via client.Stream or the generated client code
func (e *Flight) Stream(ctx context.Context, req *flight.StreamingRequest, stream flight.Flight_StreamStream) error {
	log.Infof("Received Flight.Stream request with count: %d", req.Count)

	for i := 0; i < int(req.Count); i++ {
		log.Infof("Responding: %d", i)
		if err := stream.Send(&flight.StreamingResponse{
			Count: int64(i),
		}); err != nil {
			return err
		}
	}

	return nil
}

// PingPong is a bidirectional stream handler called via client.Stream or the generated client code
func (e *Flight) PingPong(ctx context.Context, stream flight.Flight_PingPongStream) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		log.Infof("Got ping %v", req.Stroke)
		if err := stream.Send(&flight.Pong{Stroke: req.Stroke}); err != nil {
			return err
		}
	}
}
