package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	gf "github.com/brianvoe/gofakeit/v6"
	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)


 const (
  scheduled = "Scheduled"
  onTime = "On Time"
)

func CreateRandomFlight(t *testing.T) Flight {
fk := gf.NewCrypto() 

		gf.SetGlobalFaker(fk)

	t.Parallel()
	airline := CreateTestAirline(t)

	airport := CreateRandomAirport(t)

	airport1 := CreateRandomAirport(t)

	aircraft := createRandomAircraft(t)
	
	r := require.New(t)
	now := time.Now()
	sdate := now.AddDate(0,int(util.RandomInt(0,5)),0)
	ddate := now.AddDate(0,int(util.RandomInt(6,10)),0)
		fmt.Println(fk.DateRange(sdate,ddate))
	arg :=
		CreateFlightParams{
			FlightNo:           util.RandomFlightNo(airline.IataCode),
			CompanyID:          airline.ID,
			ScheduledDeparture: fk.DateRange(sdate,ddate),
			ScheduledArrival:   fk.Date(),
			DepartureAirport:   airport.IataCode,
			ArrivalAirport:     airport1.IataCode,
			Status:             onTime,
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
