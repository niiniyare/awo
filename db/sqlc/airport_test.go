package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func CreateRandomAirport(t *testing.T) Airport {

	r := require.New(t)

	lt := util.RandomFloat64(-90.0, 90.99)
	ln := util.RandomFloat64(-180.0, 180.99)

	lat := new(pgtype.Numeric)
	err := lat.Set(lt)
	r.NoError(err)

	lon := new(pgtype.Numeric)
	err = lon.Set(ln)
	r.NoError(err)

	arg := CreateAirportParams{
		IataCode:  "",
		IcaoCode:  "",
		Name:      "",
		Elevation: pgtype.Text{},
		City:      "",
		Country:   "",
		State:     "",
		Lat:       pgtype.Numeric{},
		Lon:       pgtype.Numeric{},
		Timezone:  "",
	}

	airport, err := testStore.CreateAirport(context.Background(), arg)
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

	airport2, err := testStore.GetAirport(context.Background(), airport1.IataCode)

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

	airport2, err := testStore.DeleteAirports(context.Background(), airport1.IataCode)

	r.NoError(err)
	airport2, err2 := testStore.GetAirport(context.Background(), airport1.IataCode)
	r.EqualError(err2, sql.ErrNoRows.Error())
	r.Empty(airport2)
	// r.Equal(airport1.ID, airport2.ID)
}

func TestListAirportAirport(t *testing.T) {
	r := require.New(t)

	for i := 0; i < 10; i++ {

		airport0 := CreateRandomAirport(t)
		r.NotEmpty(airport0)

	}

	airport, err := testStore.ListAirport(context.Background(), ListAirportParams{
		Limit:  5,
		Offset: 5,
	})

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
