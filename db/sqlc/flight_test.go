package db

import (
	"context"
"testing"

	gf "github.com/brianvoe/gofakeit"
	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func CreateRandomFlight(t *testing.T) Flight {
	t.Parallel()
	airline := CreateTestAirline(t)

	airport := CreateRandomAirport(t)

	airport1 := CreateRandomAirport(t)

	aircraft := createRandomAircraft(t)
	gf.Seed(0)

	r := require.New(t)
	// sdtime := time.Now().Sub(gf.Int32()*int32(time.Now().Day())
	arg :=
		CreateFlightParams{
			FlightNo:           util.RandomFlightNo(airline.IataCode),
			CompanyID:          airline.ID,
			ScheduledDeparture: gf.Date(),
			ScheduledArrival:   gf.Date(),
			DepartureAirport:   airport.IataCode,
			ArrivalAirport:     airport1.IataCode,
			Status:             "Scheduled",
			AircraftID:         aircraft.ID,
		}

	flight, err := testQueries.CreateFlight(context.Background(), arg)
	r.NoError(err)
	r.NotEmpty(flight)

	return flight

}

func TestCreateFlight(t *testing.T) {
	CreateRandomFlight(t)
}

//
// func TestGetflight(t *testing.T) {
//
// 	flight := CreateRandomFlight(t)
//
// 	arg := GetFlightParams{
// 		DepartureAirport: flight.DepartureAirport,
// 		ArrivalAirport:   flight.ArrivalAirport,
// 	}
// 	fmt.Printf("%v\n", flight)
// 	tf, err := testQueries.GetFlight(context.Background(), arg)
// 	for _, v := range tf {
// 		fmt.Println(v)
//
// 	}
// 	r := require.New(t)
//
// 	r.NoError(err)
// 	r.NotEmpty(tf)
// }
