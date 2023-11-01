package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	// "github.com/jackc/pgx/v5"
	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

const (
	scheduled = "Scheduled"
	onTime    = "On Time"
)

func CreateRandomFlight(t *testing.T) Flight {

	t.Parallel()

	airline := CreateTestAirline(t)

	airport := CreateRandomAirport(t)

	airport1 := CreateRandomAirport(t)

	aircraft := createRandomAircraft(t)

	r := require.New(t)

	// startDate := time.Date(2022, time.September, 25, 72, 01, 0, 0, time.UTC)
	// gf.DateTimer
	arr := time.Now().Add(time.Duration(util.RandomInt(1, 5) * int64(time.Hour)))
	dep := time.Now()
	// log.Printf("ScheduledDeparture time: %v\n", dep.Format(time.UnixDate))

	// log.Printf("ScheduledArrival  time: %v\n", arr.Format(time.UnixDate))

	// log.Printf("Duration time: %v\n", dep.Sub(arr).Hours())

	actualtime := sql.NullTime{Time: arr, Valid: false}
	aclerr := actualtime.Scan(arr)
	r.NoError(aclerr)
	actualDeparture := sql.NullTime{Time: dep, Valid: false}
	aclerr = actualDeparture.Scan(dep)
	r.NoError(aclerr)
	// arg := CreateFlightParams{
	// 	FlightNo:  util.RandomFlightNo(airline.IataCode),
	// 	CompanyID: airline.ID,
	// 	ScheduledDeparture:
	//    ScheduledArrival: pgtype.Timestamptz{
	// 		Time:             time.Time{},
	// 		InfinityModifier: 0,
	// 		Valid:            false,
	// 	},
	// 	DepartureAirport: airport.IataCode,
	// 	ArrivalAirport:   airport1.IataCode,
	// 	Status:           onTime,
	// 	AircraftID:       aircraft.ID,
	// 	ActualDeparture: pgtype.Timestamptz{
	// 		Time:             time.Time{},
	// 		InfinityModifier: 0,
	// 		Valid:            false,
	// 	},
	// 	ActualArrival: pgtype.Timestamptz{
	// 		Time:             time.Time{},
	// 		InfinityModifier: 0,
	// 		Valid:            false,
	// 	},
	// }
	arrAir := airport1.IataCode
	depAir := airport.IataCode
	arg := CreateFlightParams{
		FlightNo:           util.RandomFlightNo(airline.IataCode),
		CompanyID:          airline.ID,
		ScheduledDeparture: dep,
		ScheduledArrival:   arr,
		DepartureAirport:   arrAir,
		ArrivalAirport:     depAir,
		Status:             onTime,
		AircraftID:         aircraft.ID,
		ActualDeparture:    sql.NullTime{
			Time:  dep,
			Valid: true,
		},
		ActualArrival: sql.NullTime{
			Time:  arr,
			Valid: true,
		},
	}
	flight, err := testStore.CreateFlight(context.Background(), arg)
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
	// log.Printf("%+v\n", flight)
	tf, err := testStore.FlightAvailability(context.Background(), arg)
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
// 	result, err := testStore.FlightAvailability(context.Background(), args)
// 	r.NoError(err)
// 	r.NotEmpty(result)
// }
