package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func CreateRondomSeats(t *testing.T) Seat {

	r := require.New(t)

	aircraft := createRandomAircraft(t)

	r.NotEmpty(aircraft)

	Seat := fmt.Sprintf("%v%d", util.RandomString(1), util.RandomInt(1, 5))

	arg := CreateSeatParams{
		AircraftID:     aircraft.ID,
		SeatNo:         Seat,
		FareConditions: "Economy",
	}
	seat, err := testStore.CreateSeat(context.Background(), arg)

	r.NoError(err)
	r.NotEmpty(seat)

	r.Equal(aircraft.ID, seat.AircraftID)

	return seat

}

func TestGetSeat(t *testing.T) {
	r := require.New(t)

	seat1 := CreateRondomSeats(t)

	r.NotEmpty(seat1)

	seat, err := testStore.GetSeats(context.Background())

	r.NoError(err)

	r.NotNil(seat)

}
