package db

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func CreateRandomAirport(t *testing.T) Airport {

	r := require.New(t)

	lt := util.RandomFloat64(-90.0, 90)
	ln := util.RandomFloat64(-180, 180)

	lat := new(pgtype.Numeric)
	err := lat.Set(lt)
	r.NoError(err)

	lon := new(pgtype.Numeric)
	err = lon.Set(ln)
	r.NoError(err)
	arg := CreateAirportParams{
		IataCode: strings.ToUpper(util.RandomString(3)),
		IcaoCode: strings.ToUpper(util.RandomString(4)),
		Name:     strings.ToUpper(util.RandomString(10)),
		City:     strings.ToUpper(util.RandomString(4)),
		Country:  strings.ToUpper(util.RandomString(3)),
		State:    strings.ToUpper(util.RandomString(4)),

		Timezone: "Africa/Mogadishu",

		Lat: *lat,
		Lon: *lon,
	}

	airport, err := testQueries.CreateAirport(context.Background(), arg)
	r.NoError(err)
	r.NotEmpty(airport)

	return airport

}

func TestCreateAirport(t *testing.T) {
	CreateRandomAirport(t)
}

func TestGetAirport(t *testing.T) {
	r := require.New(t)
	airport1 := CreateRandomAirport(t)
	r.NotEmpty(airport1)

	airport2, err := testQueries.GetAirport(context.Background(), airport1.IataCode)

	r.NoError(err)
	r.NotEmpty(airport2)

	r.Equal(airport1.IataCode, airport2.IataCode)
	r.Equal(airport1.ID, airport2.ID)
	r.Equal(airport1.IcaoCode, airport2.IcaoCode)
	r.Equal(airport1.Name, airport2.Name)
	r.Equal(airport1.City, airport2.City)

}

func TestDeleteAirport(t *testing.T) {
	r := require.New(t)
	airport1 := CreateRandomAirport(t)
	r.NotEmpty(airport1)

	err := testQueries.DeleteAirports(context.Background(), airport1.ID)

	r.NoError(err)
	airport2, err2 := testQueries.GetAirport(context.Background(), airport1.IataCode)

	fmt.Errorf("%w\n", err2)
	r.Error(err2)
	r.EqualError(err2, pgx.ErrNoRows.Error())
	r.Empty(airport2)

}

func TestListAirportAirport(t *testing.T) {
	r := require.New(t)

	for i := 0; i < 10; i++ {

		airport0 := CreateRandomAirport(t)
		r.NotEmpty(airport0)

	}

	airport, err := testQueries.ListAirport(context.Background())

	r.NoError(err)
	r.NotEmpty(airport)
	r.NotNil(airport)
	r.GreaterOrEqual(len(airport), 0)

}

// func TestListAirport(t *testing.T) {
//
//
//
// }
// func (c CreateAirportParams) SetCoordinates(lon, lat float64) error {
// 	return c.Coordinates.Set(fmt.Sprint("%v,%v", lon, lat))
// }
