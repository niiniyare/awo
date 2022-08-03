package db

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func CreateRandomAirport(t *testing.T) Airport {

	arg := CreateAirportParams{
		IataCode: strings.ToUpper(util.RandomString(3)),
		IcaoCode: strings.ToUpper(util.RandomString(4)),
		Name:     strings.ToUpper(util.RandomString(10)),
		City:     strings.ToUpper(util.RandomString(4)),

		// Coordinates: pgtype.Point{
		// 	P: pgtype.Vec2{

		// 		X: util.RandomFloat64(-90, 90),

		// 		Y: util.RandomFloat64(-180, 180)},
		// 	Status: 1,
		// },
	}
	r := require.New(t)

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

	r.Error(err2)
	r.EqualError(err2, pgx.ErrNoRows.Error())
	r.Empty(airport2)

}

func TestUpdateAirport(t *testing.T) {
	r := require.New(t)
	airport1 := CreateRandomAirport(t)
	r.NotEmpty(airport1)

	

}
