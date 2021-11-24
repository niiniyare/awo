package createairline

import (
	"context"

	"github.com/Abdirahman0022/airline/domain/entity"
)

//go:generate mockery --name Outport -output mocks/

type createAirlineInteractor struct {
	outport Outport
}

// NewUsecase is constructor for create default implementation of usecase CreateAirline
func NewUsecase(outputPort Outport) Inport {
	return &createAirlineInteractor{
		outport: outputPort,
	}
}

// Execute the usecase CreateAirline
func (r *createAirlineInteractor) Execute(ctx context.Context, req InportRequest) (*InportResponse, error) {

	res := &InportResponse{
	  
	}

	// code your usecase definition here ...

	airlineObj, err := entity.NewAirline(entity.AirlineRequest{})
	if err != nil {
		return nil, err
	}

	err = r.outport.SaveAirline(ctx, airlineObj)
	if err != nil {
		return nil, err
	}

	return res, nil
}
