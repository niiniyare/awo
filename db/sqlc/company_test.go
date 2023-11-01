package db

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func CreateTestAirline(t *testing.T) *Airline {
	arg := CreateAirlineParams{
		CompanyName:       util.RandomString(7),
		IataCode:          strings.ToUpper(util.RandomString(2)),
		RegistaredCountry: strings.ToUpper(util.RandomString(2)),
	}
	airline, err := testStore.CreateAirline(context.Background(), arg)
	r := require.New(t)
	r.NoError(err)
	r.NotEmpty(airline)
	r.Equal(arg.CompanyName, airline.CompanyName)
	r.Equal(arg.IataCode, airline.IataCode)
	r.Equal(arg.RegistaredCountry, airline.RegistaredCountry)
	r.NotZero(airline.ID)

	r.NotZero(airline.CreatedAt)

	return &airline
}

func TestCreateAirline(t *testing.T) {
	CreateTestAirline(t)
}
func TestGetAirline(t *testing.T) {
	airline := CreateTestAirline(t)
	r := require.New(t)

	r.NotEmpty(airline)
	res, err := testStore.GetAirline(context.Background(), airline.ID)

	r.NoError(err)
	r.NotEmpty(res)
	r.Equal(airline.ID, res.ID)

	r.Equal(airline.CompanyName, res.CompanyName)

	r.Equal(airline.IataCode, res.IataCode)

	r.Equal(airline.RegistaredCountry, res.RegistaredCountry)

}

func TestListAirline(t *testing.T) {
	for i := 0; i < 10; i++ {
		CreateTestAirline(t)
	}
	r := require.New(t)
	arg := ListAirlineParams{
		Limit:  10,
		Offset: 0,
	}
	res, err := testStore.ListAirline(context.Background(), arg)

	r.NoError(err)
	r.NotEmpty(res)
	r.Len(res, 10)
	for _, airline := range res {
		r.NotEmpty(airline)
		r.NotZero(airline.ID)
		r.NotZero(airline.CreatedAt)
		r.NotEmpty(airline.CompanyName)
		r.NotEmpty(airline.IataCode)
		r.NotEmpty(airline.RegistaredCountry)

	}

}

func TestDeleteAirline(t *testing.T) {
	airline := CreateTestAirline(t)
	r := require.New(t)
	err := testStore.DeleteAirline(context.Background(), airline.ID)
	r.NoError(err)
	res, err := testStore.GetAirline(context.Background(), airline.ID)
	r.Error(err)
	r.Empty(res)

	arline1, err := testStore.GetAirline(context.Background(), airline.ID)
	r.Error(err)
	r.Empty(arline1)
	r.EqualError(err, sql.ErrNoRows.Error())

}

// func TestUpdateAirline(t *testing.T) {
// 	t.Skip("not implemented")
//
// }
