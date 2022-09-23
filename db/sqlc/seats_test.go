package db

import (
	"context"
	"testing"

	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func CreateRondomSeats(t *testing.T) Seat {

	r := require.New(t)

	aircraft := createRandomAircraft(t)

	r.NotEmpty(aircraft)

	arg := CreateSeatParams{
		AircraftID:     aircraft.ID,
		SeatNo:         util.RandomAlphaString(2),
		FareConditions: "Economy'",
	}
	seat, err := testQueries.CreateSeat(context.Background(), arg)

	r.NoError(err)
	r.NotEmpty(seat)

	r.Equal(aircraft.ID, seat.AircraftID)

	return seat

}

func TestGetSeat(t *testing.T) {
	r := require.New(t)

	seat1 := CreateRondomSeats(t)

	r.NotEmpty(seat1)

	seat, err := testQueries.GetSeats(context.Background())

	r.NoError(err)

	r.NotNil(seat)

}
