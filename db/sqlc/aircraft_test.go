package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func createRandomAircraft(t *testing.T) Aircraft {
	airline := CreateTestAirline(t)

	r := require.New(t)
	r.NotEmpty(airline)

	args := CreateAircraftParams{

		Code:      strings.ToUpper(util.RandomString(3)),
		Model:     "Boeing 737-800 Freighter",
		Range:     3700,
		CompanyID: airline.ID,
	}
	aircraft, err := testQueries.CreateAircraft(context.Background(), args)
	r.NoError(err)
	r.Equal(aircraft.Code, args.Code)
	r.Equal(aircraft.Model, args.Model)
	r.Equal(aircraft.Range, args.Range)
	r.Equal(aircraft.CompanyID, airline.ID)

	r.NotEmpty(aircraft)

	return aircraft
}

func TestCreateAircraft(t *testing.T) {
	aircraft := createRandomAircraft(t)

	r := require.New(t)
	r.NotEmpty(aircraft)

}

func TestGetAircraft(t *testing.T) {
	aircraft1 := createRandomAircraft(t)

	r := require.New(t)
	r.NotEmpty(aircraft1)

	aircraft2, err := testQueries.GetAircraft(context.Background(), aircraft1.ID)

	r.NoError(err)
	r.NotEmpty(aircraft2)

	r.Equal(aircraft2.ID, aircraft1.ID)
	r.Equal(aircraft2.Code, aircraft1.Code)

	r.Equal(aircraft2.Range, aircraft1.Range)
	r.Equal(aircraft2.CompanyID, aircraft1.CompanyID)
	r.WithinDuration(aircraft1.CreatedAt, time.Now(), time.Second)

}

func TestDeleteAircraft(t *testing.T) {
	aircraft := createRandomAircraft(t)

	r := require.New(t)

	r.NotEmpty(aircraft)

	err := testQueries.DeleteAircraft(context.Background(), aircraft.ID)

	r.NoError(err)

	aircraft1, errs := testQueries.GetAircraft(context.Background(), aircraft.ID)
	r.EqualError(errs, pgx.ErrNoRows.Error())
	r.Empty(aircraft1)

}

func TestListAircraft(t *testing.T) {
	r := require.New(t)
	for i := 0; i < 10; i++ {
		aircraft := createRandomAircraft(t)
		r.NotEmpty(aircraft)
	}

	arg := ListAircraftParams{
		Limit:  5,
		Offset: 5,
	}

	aircraft1, err := testQueries.ListAircraft(context.Background(), arg)
	r.NoError(err)
	r.NotNil(aircraft1)
	for _, v := range aircraft1 {
		r.NotEmpty(v)
	}

}
