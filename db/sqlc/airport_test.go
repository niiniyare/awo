package db

import (
	"context"
	"strings"
	"testing"

	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func TestCreateRandomAirport(t *testing.T) {

	arg := CreateAirportParams{
		IataCode: strings.ToUpper(util.RandomString(3)),
		IcaoCode: strings.ToUpper(util.RandomString(4)),
		Name:     strings.ToUpper(util.RandomString(10)),
		City:     strings.ToUpper(util.RandomString(4)),
		Point:    util.RandomPoint(),
	}
	r := require.New(t)

	airport, err := testQueries.CreateAirport(context.Background(), arg)
	r.NoError(err)
	r.NotEmpty(airport)

}
