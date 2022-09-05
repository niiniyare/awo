package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

const (
	scheduled string = "Scheduled"
	onTime           = "On Time"
)

func CreateRandomFlight(t *testing.T) Flight {

	t.Parallel()

	airline := CreateTestAirline(t)

	airport := CreateRandomAirport(t)

	airport1 := CreateRandomAirport(t)

	aircraft := createRandomAircraft(t)

	r := require.New(t)

	startDate := time.Date(2022, time.September, 25, 72, 01, 0, 0, time.UTC)

	dep := fk.DateRange(startDate, startDate.Add(time.Hour*time.Duration(util.RandomInt(1, 12))))

	arr := dep.AddDate(0, 0, int(util.RandomInt(1, 2)))

	fmt.Printf("ScheduledDeparture time: %v\n", dep.Format(time.UnixDate))

	fmt.Printf("ScheduledArrival  time: %v\n", arr.Format(time.UnixDate))

	fmt.Printf("Duration time: %v\n", dep.Sub(arr).Hours())

	arg :=
		CreateFlightParams{
			FlightNo:           util.RandomFlightNo(airline.IataCode),
			CompanyID:          airline.ID,
			ScheduledDeparture: dep,
			ScheduledArrival:   arr,
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
