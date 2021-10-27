package handler

import (
	"context"

	log "github.com/micro/micro/v3/service/logger"

	flightinfo "flightinfo/pb"
)

type Flightinfo struct{}

// Call is a single request handler called via client.Call or the generated client code
func (e *Flightinfo) Call(ctx context.Context, req *flightinfo.Request, rsp *flightinfo.Response) error {
	log.Info("Received Flightinfo.Call request")
	rsp.Msg = "Hello " + req.Name
	return nil
}

// Stream is a server side stream handler called via client.Stream or the generated client code
func (e *Flightinfo) Stream(ctx context.Context, req *flightinfo.StreamingRequest, stream flightinfo.Flightinfo_StreamStream) error {
	log.Infof("Received Flightinfo.Stream request with count: %d", req.Count)

	for i := 0; i < int(req.Count); i++ {
		log.Infof("Responding: %d", i)
		if err := stream.Send(&flightinfo.StreamingResponse{
			Count: int64(i),
		}); err != nil {
			return err
		}
	}

	return nil
}

// PingPong is a bidirectional stream handler called via client.Stream or the generated client code
func (e *Flightinfo) PingPong(ctx context.Context, stream flightinfo.Flightinfo_PingPongStream) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		log.Infof("Got ping %v", req.Stroke)
		if err := stream.Send(&flightinfo.Pong{Stroke: req.Stroke}); err != nil {
			return err
		}
	}
}
