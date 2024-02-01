// Coverage template
package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CoordinatesSuite struct {
	suite.Suite
}

func (s *CoordinatesSuite) TestDistance0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance0", r)
		}
	}()

	lat1 := 35.884369
	lng1 := 54.989389
	lat2 := 48.832243
	lng2 := 33.261875
	unit := "Turner Kuhlman"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance1", r)
		}
	}()

	lat1 := -82.309184
	lng1 := 47.504420
	lat2 := -4.480028
	lng2 := -42.846272
	unit := "Jermey Little"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance2", r)
		}
	}()

	lat1 := -92.675539
	lng1 := -9.696579
	lat2 := -49.045918
	lng2 := -15.477361
	unit := "Dejuan Mills"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance3", r)
		}
	}()

	lat1 := -7.541234
	lng1 := 58.581942
	lat2 := 19.888459
	lng2 := -29.576289
	unit := "Monroe Cremin"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance4", r)
		}
	}()

	lat1 := -28.311806
	lng1 := -67.605766
	lat2 := 87.795617
	lng2 := 22.718181
	unit := "Matt Abshire"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance5", r)
		}
	}()

	lat1 := -12.155960
	lng1 := 82.675393
	lat2 := -10.445627
	lng2 := 59.497434
	unit := "Clemmie Steuber"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance6", r)
		}
	}()

	lat1 := -20.734380
	lng1 := -15.466928
	lat2 := -94.968631
	lng2 := -94.011581
	unit := "Dock Hirthe"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance7", r)
		}
	}()

	lat1 := 57.084658
	lng1 := 74.699685
	lat2 := -86.082747
	lng2 := 33.393209
	unit := "Murphy Legros"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance8", r)
		}
	}()

	lat1 := -0.378030
	lng1 := 41.965352
	lat2 := -95.141131
	lng2 := -5.235203
	unit := "Tremaine Watsica"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance9", r)
		}
	}()

	lat1 := 30.471192
	lng1 := 62.192209
	lat2 := 61.745373
	lng2 := 68.062856
	unit := "Clementine Klein"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance10", r)
		}
	}()

	lat1 := 20.110957
	lng1 := -82.870540
	lat2 := -39.094330
	lng2 := -38.273144
	unit := "Lazaro Bruen"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance11", r)
		}
	}()

	lat1 := 33.358214
	lng1 := 28.583732
	lat2 := 22.734300
	lng2 := 55.998599
	unit := "Talon Cummerata"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance12", r)
		}
	}()

	lat1 := 82.623907
	lng1 := 74.858188
	lat2 := -4.406367
	lng2 := -29.381273
	unit := "Dianna Goyette"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance13", r)
		}
	}()

	lat1 := -30.946324
	lng1 := -65.778521
	lat2 := -55.824081
	lng2 := -88.968357
	unit := "Antonio Turner"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance14", r)
		}
	}()

	lat1 := -25.354883
	lng1 := -52.475129
	lat2 := -6.359744
	lng2 := -57.584609
	unit := "Kaleigh VonRueden"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance15", r)
		}
	}()

	lat1 := -96.867304
	lng1 := -28.863857
	lat2 := -90.578110
	lng2 := 41.185036
	unit := "Heather Romaguera"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance16", r)
		}
	}()

	lat1 := 3.278865
	lng1 := -28.541958
	lat2 := -64.081432
	lng2 := 88.584836
	unit := "Lenora Dibbert"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func (s *CoordinatesSuite) TestDistance17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestDistance17", r)
		}
	}()

	lat1 := -86.071863
	lng1 := 39.523388
	lat2 := -82.392972
	lng2 := -11.870874
	unit := "David Pollich"

	Distance(lat1, lng1, lat2, lng2, unit)

}

func TestCoordinatesSuite(t *testing.T) {
	suite.Run(t, new(CoordinatesSuite))
}
