// Coverage template
package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CurrencySuite struct {
	suite.Suite
}

func (s *CurrencySuite) TestIsSupportedCurrency0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency0", r)
		}
	}()

	currency := "Adriana Kris"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency1", r)
		}
	}()

	currency := "Vergie Hoeger"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency2", r)
		}
	}()

	currency := "Dorothea Kuvalis"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency3", r)
		}
	}()

	currency := "Noelia Thompson"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency4", r)
		}
	}()

	currency := "Zane Price"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency5", r)
		}
	}()

	currency := "Guiseppe Feil"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency6", r)
		}
	}()

	currency := "Jamal Schumm"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency7", r)
		}
	}()

	currency := "Izabella McDermott"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency8", r)
		}
	}()

	currency := "Cathryn Mills"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency9", r)
		}
	}()

	currency := "Adelia Block"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency10", r)
		}
	}()

	currency := "Velma Kuvalis"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency11", r)
		}
	}()

	currency := "Wilson Bahringer"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency12", r)
		}
	}()

	currency := "Hilton Bechtelar"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency13", r)
		}
	}()

	currency := "Jerrod Vandervort"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency14", r)
		}
	}()

	currency := "Gloria Kunze"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency15", r)
		}
	}()

	currency := "Cornell Renner"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency16", r)
		}
	}()

	currency := "Vivian Sauer"

	IsSupportedCurrency(currency)

}

func (s *CurrencySuite) TestIsSupportedCurrency17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestIsSupportedCurrency17", r)
		}
	}()

	currency := "Audrey Okuneva"

	IsSupportedCurrency(currency)

}

func TestCurrencySuite(t *testing.T) {
	suite.Run(t, new(CurrencySuite))
}
