package db

import (
	"context"
	"database/sql"
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

	actualtime := sql.NullTime{Time: arr}
	aclerr := actualtime.Scan(arr)
	r.NoError(aclerr)
	actualDeparture := sql.NullTime{Time: arr}
	aclerr = actualDeparture.Scan(dep)
	r.NoError(aclerr)
	arg := CreateFlightParams{
		FlightNo:           util.RandomFlightNo(airline.IataCode),
		CompanyID:          airline.ID,
		ScheduledDeparture: dep,
		ScheduledArrival:   arr,
		DepartureAirport:   airport.IataCode,
		ArrivalAirport:     airport1.IataCode,
		Status:             onTime,
		AircraftID:         aircraft.ID,
		ActualDeparture:    actualDeparture,
		ActualArrival:      actualtime,
	}
	flight, err := testQueries.CreateFlight(context.Background(), arg)
	r.NoError(err)
	r.NotEmpty(flight)

	return flight

}

func TestCreateFlight(t *testing.T) {
	CreateRandomFlight(t)
}

func TestGetflight(t *testing.T) {
	r := require.New(t)

	flight := CreateRandomFlight(t)
	r.NotEmpty(flight)
	arg := FlightAvailabilityParams{
		DepartureAirport: flight.DepartureAirport,
		ArrivalAirport:   flight.ArrivalAirport,
		CompanyID:        flight.CompanyID,
	}
	// fmt.Printf("%+v\n", flight)
	tf, err := testQueries.FlightAvailability(context.Background(), arg)
	// fmt.Errorf("FlightAvailability: %w", err)
	r.NoError(err)

	for _, f := range tf {

		r.NotEmpty(f)

		r.Equal(f.ArrivalAirport, flight.ArrivalAirport)
		r.Equal(f.DepartureAirport, flight.DepartureAirport)
		r.Equal(f.FlightNo, flight.FlightNo)
		r.Equal(f.FlightID, flight.FlightID)

	}

	r.NotNil(tf)
}

// func TestAvailabilityFlight(t *testing.T) {
// 	r := require.New(t)
// 	f1 := CreateRandomFlight(t)
//
// 	args := FlightAvailabilityParams{
// 		DepartureAirport: f1.DepartureAirport,
// 		ArrivalAirport:   f1.ArrivalAirport,
// 		CompanyID:        f1.CompanyID,
// 		Limit:            2,
// 	}
// 	result, err := testQueries.FlightAvailability(context.Background(), args)
// 	r.NoError(err)
// 	r.NotEmpty(result)
// }
