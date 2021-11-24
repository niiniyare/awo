package repository

import (
	"context"

	"github.com/Abdirahman0022/airline/domain/entity"
)

type SaveAirlineRepo interface {
	SaveAirline(ctx context.Context, obj *entity.Airline) error
}
