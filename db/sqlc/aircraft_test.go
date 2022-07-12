package db

import (
	"context"
	"strings"
	"testing"

	"github.com/niiniyare/awo/util"
	"github.com/stretchr/testify/require"
)

func createTestAircraft(t *testing.T) Aircraft {
	//airline := createTestAirline(t) // create airline first
	r := require.New(t)
	//r.NotNil(airline)
	//	r.Equal(airline.Code, strings.ToUpper(airline.Code))

	args := CreateAircraftParams{
		Code:      strings.ToUpper(util.RandomString(3)),
		Model:     util.RandomString(8),
		Range:     int32(util.RandomInt(100, 200)),
		CompanyID: 3,
	}
	// Test the Aircraft struct
	aircraft, err := testQueries.CreateAircraft(context.Background(), args)
	//	r.NoError(err)
	r.NoError(err)
	r.NotEmpty(aircraft)
	r.Equal(args.Code, aircraft.Code)
	return aircraft
}

func TestCreateAircraft(t *testing.T) {
	aircraft := createTestAircraft(t)
	r := require.New(t)
	r.NotNil(aircraft)
	r.Equal(aircraft.Code, strings.ToUpper(aircraft.Code))

}
