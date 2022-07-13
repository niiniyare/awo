package db

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func createRandomAircraft(t *testing.T) Aircraft {
	airline := CreateTestAirline(t)

	r := require.New(t)
	r.NotEmpty(airline)

	args := CreateAircraftParams{
		Code:      "73U",
		Model:     "Boeing 737-800 Freighter",
		Range:     3700,
		CompanyID: airline.ID,
	}
	aircraft, err := testQueries.CreateAircraft(context.Background(), args)
	r.NoError(err)

	r.NotEmpty(aircraft)

	return aircraft
}

func TestCreateAircraft(t *testing.T) {
	aircraft := createRandomAircraft(t)
	r := require.New(t)
	r.NotNil(aircraft)
	r.Equal(aircraft.Code, strings.ToUpper(aircraft.Code))

}
