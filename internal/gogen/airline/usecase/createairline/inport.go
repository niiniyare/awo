package createairline

import (
	"context"
	"time"
)

// Inport of CreateAirline
type Inport interface {
	Execute(ctx context.Context, req InportRequest) (*InportResponse, error)
}

// InportRequest is request payload to run the usecase CreateAirline
type InportRequest struct {
	CompanyName string `` //
	IataCode    string `` //
}

// InportResponse is response payload after running the usecase CreateAirline
type InportResponse struct {
	ID          int64     `` //
	CompanyName string    `` //
	IataCode    string    `` //
	CreatedAt   time.Time `` //
}
