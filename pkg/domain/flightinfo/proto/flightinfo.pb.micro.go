// Code generated by protoc-gen-micro. DO NOT EDIT.
// source: proto/flightinfo.proto

package flightinfo

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	math "math"
)

import (
	context "context"
	api "github.com/micro/micro/v3/service/api"
	client "github.com/micro/micro/v3/service/client"
	server "github.com/micro/micro/v3/service/server"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// Reference imports to suppress errors if they are not otherwise used.
var _ api.Endpoint
var _ context.Context
var _ client.Option
var _ server.Option

// Api Endpoints for FlightService service

func NewFlightServiceEndpoints() []*api.Endpoint {
	return []*api.Endpoint{}
}

// Client API for FlightService service

type FlightService interface {
	CreateFlight(ctx context.Context, in *Flight, opts ...client.CallOption) (*Flight, error)
	SearchFlight(ctx context.Context, in *FlightRequest, opts ...client.CallOption) (*FlightResponse, error)
}

type flightService struct {
	c    client.Client
	name string
}

func NewFlightService(name string, c client.Client) FlightService {
	return &flightService{
		c:    c,
		name: name,
	}
}

func (c *flightService) CreateFlight(ctx context.Context, in *Flight, opts ...client.CallOption) (*Flight, error) {
	req := c.c.NewRequest(c.name, "FlightService.CreateFlight", in)
	out := new(Flight)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *flightService) SearchFlight(ctx context.Context, in *FlightRequest, opts ...client.CallOption) (*FlightResponse, error) {
	req := c.c.NewRequest(c.name, "FlightService.SearchFlight", in)
	out := new(FlightResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for FlightService service

type FlightServiceHandler interface {
	CreateFlight(context.Context, *Flight, *Flight) error
	SearchFlight(context.Context, *FlightRequest, *FlightResponse) error
}

func RegisterFlightServiceHandler(s server.Server, hdlr FlightServiceHandler, opts ...server.HandlerOption) error {
	type flightService interface {
		CreateFlight(ctx context.Context, in *Flight, out *Flight) error
		SearchFlight(ctx context.Context, in *FlightRequest, out *FlightResponse) error
	}
	type FlightService struct {
		flightService
	}
	h := &flightServiceHandler{hdlr}
	return s.Handle(s.NewHandler(&FlightService{h}, opts...))
}

type flightServiceHandler struct {
	FlightServiceHandler
}

func (h *flightServiceHandler) CreateFlight(ctx context.Context, in *Flight, out *Flight) error {
	return h.FlightServiceHandler.CreateFlight(ctx, in, out)
}

func (h *flightServiceHandler) SearchFlight(ctx context.Context, in *FlightRequest, out *FlightResponse) error {
	return h.FlightServiceHandler.SearchFlight(ctx, in, out)
}