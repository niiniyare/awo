// Coverage template
package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RandomSuite struct {
	suite.Suite
}

func (s *RandomSuite) TestRandomAlphaString0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString0", r)
		}
	}()

	n := 99

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString1", r)
		}
	}()

	n := 13

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString2", r)
		}
	}()

	n := 54

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString3", r)
		}
	}()

	n := -43

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString4", r)
		}
	}()

	n := -81

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString5", r)
		}
	}()

	n := 43

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString6", r)
		}
	}()

	n := -35

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString7", r)
		}
	}()

	n := 21

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString8", r)
		}
	}()

	n := -5

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString9", r)
		}
	}()

	n := -94

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString10", r)
		}
	}()

	n := 80

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString11", r)
		}
	}()

	n := 46

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString12", r)
		}
	}()

	n := -16

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString13", r)
		}
	}()

	n := -35

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString14", r)
		}
	}()

	n := 55

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString15", r)
		}
	}()

	n := -7

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString16", r)
		}
	}()

	n := -16

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomAlphaString17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomAlphaString17", r)
		}
	}()

	n := -79

	RandomAlphaString(n)

}

func (s *RandomSuite) TestRandomCurrency0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency0", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency1", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency2", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency3", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency4", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency5", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency6", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency7", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency8", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency9", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency10", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency11", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency12", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency13", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency14", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency15", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency16", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomCurrency17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomCurrency17", r)
		}
	}()

	RandomCurrency()

}

func (s *RandomSuite) TestRandomEmail0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail0", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail1", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail2", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail3", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail4", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail5", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail6", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail7", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail8", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail9", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail10", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail11", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail12", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail13", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail14", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail15", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail16", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomEmail17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomEmail17", r)
		}
	}()

	RandomEmail()

}

func (s *RandomSuite) TestRandomFlight0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight0", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight1", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight2", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight3", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight4", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight5", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight6", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight7", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight8", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight9", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight10", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight11", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight12", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight13", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight14", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight15", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight16", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlight17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlight17", r)
		}
	}()

	RandomFlight()

}

func (s *RandomSuite) TestRandomFlightNo0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo0", r)
		}
	}()

	s2 := "Clyde Armstrong"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo1", r)
		}
	}()

	s2 := "Amira Rolfson"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo2", r)
		}
	}()

	s2 := "Cortez Bartell"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo3", r)
		}
	}()

	s2 := "Arjun Hauck"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo4", r)
		}
	}()

	s2 := "Harley Langworth"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo5", r)
		}
	}()

	s2 := "Angie Crist"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo6", r)
		}
	}()

	s2 := "Britney Dickens"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo7", r)
		}
	}()

	s2 := "Sophie Ryan"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo8", r)
		}
	}()

	s2 := "Fatima Luettgen"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo9", r)
		}
	}()

	s2 := "Minnie Lesch"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo10", r)
		}
	}()

	s2 := "Benedict Schmitt"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo11", r)
		}
	}()

	s2 := "Marcelo Kreiger"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo12", r)
		}
	}()

	s2 := "Saul Fritsch"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo13", r)
		}
	}()

	s2 := "Dorris Powlowski"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo14", r)
		}
	}()

	s2 := "Sigmund Wintheiser"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo15", r)
		}
	}()

	s2 := "Joseph Gislason"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo16", r)
		}
	}()

	s2 := "Godfrey Mosciski"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFlightNo17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFlightNo17", r)
		}
	}()

	s2 := "Orrin Kutch"

	RandomFlightNo(s2)

}

func (s *RandomSuite) TestRandomFloat320() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat320", r)
		}
	}()

	min := float32(-78.406923)
	max := float32(-49.728327)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat321() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat321", r)
		}
	}()

	min := float32(99.198520)
	max := float32(81.328071)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat322() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat322", r)
		}
	}()

	min := float32(81.377477)
	max := float32(-44.829540)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat323() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat323", r)
		}
	}()

	min := float32(-54.351531)
	max := float32(99.604359)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat324() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat324", r)
		}
	}()

	min := float32(-60.845572)
	max := float32(3.525585)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat325() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat325", r)
		}
	}()

	min := float32(-63.865118)
	max := float32(-73.968842)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat326() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat326", r)
		}
	}()

	min := float32(36.464710)
	max := float32(-33.403518)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat327() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat327", r)
		}
	}()

	min := float32(35.091351)
	max := float32(35.126613)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat328() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat328", r)
		}
	}()

	min := float32(-25.122413)
	max := float32(-91.546183)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat329() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat329", r)
		}
	}()

	min := float32(47.202787)
	max := float32(42.363014)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat3210() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat3210", r)
		}
	}()

	min := float32(18.963757)
	max := float32(76.698930)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat3211() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat3211", r)
		}
	}()

	min := float32(13.901478)
	max := float32(-48.721190)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat3212() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat3212", r)
		}
	}()

	min := float32(-5.129823)
	max := float32(69.239736)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat3213() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat3213", r)
		}
	}()

	min := float32(99.852325)
	max := float32(-83.585273)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat3214() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat3214", r)
		}
	}()

	min := float32(-60.890063)
	max := float32(73.127115)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat3215() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat3215", r)
		}
	}()

	min := float32(65.526057)
	max := float32(30.325832)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat3216() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat3216", r)
		}
	}()

	min := float32(-44.481510)
	max := float32(95.568212)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat3217() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat3217", r)
		}
	}()

	min := float32(-0.744001)
	max := float32(98.749824)

	RandomFloat32(min, max)

}

func (s *RandomSuite) TestRandomFloat640() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat640", r)
		}
	}()

	min := -49.287864
	max := -70.906523

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat641() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat641", r)
		}
	}()

	min := 49.066583
	max := -14.746066

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat642() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat642", r)
		}
	}()

	min := -32.322445
	max := -73.689036

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat643() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat643", r)
		}
	}()

	min := 24.927108
	max := 82.658775

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat644() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat644", r)
		}
	}()

	min := 4.224496
	max := 37.816025

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat645() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat645", r)
		}
	}()

	min := 49.410342
	max := 32.465510

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat646() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat646", r)
		}
	}()

	min := -69.026052
	max := -97.231392

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat647() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat647", r)
		}
	}()

	min := -31.132021
	max := -16.180944

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat648() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat648", r)
		}
	}()

	min := 40.312442
	max := 75.683380

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat649() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat649", r)
		}
	}()

	min := -0.484458
	max := -37.331121

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat6410() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat6410", r)
		}
	}()

	min := -19.003980
	max := 17.741093

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat6411() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat6411", r)
		}
	}()

	min := -74.576282
	max := 38.939919

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat6412() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat6412", r)
		}
	}()

	min := -21.126396
	max := 71.867023

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat6413() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat6413", r)
		}
	}()

	min := 76.735517
	max := -42.180768

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat6414() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat6414", r)
		}
	}()

	min := -95.112141
	max := 45.904631

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat6415() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat6415", r)
		}
	}()

	min := 88.508678
	max := -15.462441

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat6416() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat6416", r)
		}
	}()

	min := -29.915744
	max := -80.481710

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomFloat6417() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomFloat6417", r)
		}
	}()

	min := -92.615214
	max := -47.256410

	RandomFloat64(min, max)

}

func (s *RandomSuite) TestRandomInt0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt0", r)
		}
	}()

	min := int64(19)
	max := int64(-92)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt1", r)
		}
	}()

	min := int64(35)
	max := int64(-77)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt2", r)
		}
	}()

	min := int64(-64)
	max := int64(-29)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt3", r)
		}
	}()

	min := int64(-94)
	max := int64(-4)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt4", r)
		}
	}()

	min := int64(80)
	max := int64(-61)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt5", r)
		}
	}()

	min := int64(60)
	max := int64(-74)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt6", r)
		}
	}()

	min := int64(-56)
	max := int64(95)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt7", r)
		}
	}()

	min := int64(4)
	max := int64(33)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt8", r)
		}
	}()

	min := int64(97)
	max := int64(92)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt9", r)
		}
	}()

	min := int64(5)
	max := int64(61)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt10", r)
		}
	}()

	min := int64(8)
	max := int64(-62)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt11", r)
		}
	}()

	min := int64(-60)
	max := int64(75)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt12", r)
		}
	}()

	min := int64(-60)
	max := int64(-15)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt13", r)
		}
	}()

	min := int64(-79)
	max := int64(52)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt14", r)
		}
	}()

	min := int64(-51)
	max := int64(73)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt15", r)
		}
	}()

	min := int64(-13)
	max := int64(-38)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt16", r)
		}
	}()

	min := int64(79)
	max := int64(-15)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomInt17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomInt17", r)
		}
	}()

	min := int64(50)
	max := int64(66)

	RandomInt(min, max)

}

func (s *RandomSuite) TestRandomMoney0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney0", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney1", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney2", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney3", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney4", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney5", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney6", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney7", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney8", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney9", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney10", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney11", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney12", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney13", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney14", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney15", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney16", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomMoney17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomMoney17", r)
		}
	}()

	RandomMoney()

}

func (s *RandomSuite) TestRandomOwner0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner0", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner1", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner2", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner3", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner4", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner5", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner6", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner7", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner8", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner9", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner10", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner11", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner12", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner13", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner14", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner15", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner16", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomOwner17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomOwner17", r)
		}
	}()

	RandomOwner()

}

func (s *RandomSuite) TestRandomPoint0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint0", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint1", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint2", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint3", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint4", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint5", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint6", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint7", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint8", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint9", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint10", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint11", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint12", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint13", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint14", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint15", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint16", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomPoint17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomPoint17", r)
		}
	}()

	RandomPoint()

}

func (s *RandomSuite) TestRandomString0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString0", r)
		}
	}()

	n := -92

	RandomString(n)

}

func (s *RandomSuite) TestRandomString1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString1", r)
		}
	}()

	n := -20

	RandomString(n)

}

func (s *RandomSuite) TestRandomString2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString2", r)
		}
	}()

	n := -79

	RandomString(n)

}

func (s *RandomSuite) TestRandomString3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString3", r)
		}
	}()

	n := 73

	RandomString(n)

}

func (s *RandomSuite) TestRandomString4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString4", r)
		}
	}()

	n := -2

	RandomString(n)

}

func (s *RandomSuite) TestRandomString5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString5", r)
		}
	}()

	n := 88

	RandomString(n)

}

func (s *RandomSuite) TestRandomString6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString6", r)
		}
	}()

	n := -88

	RandomString(n)

}

func (s *RandomSuite) TestRandomString7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString7", r)
		}
	}()

	n := 39

	RandomString(n)

}

func (s *RandomSuite) TestRandomString8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString8", r)
		}
	}()

	n := 94

	RandomString(n)

}

func (s *RandomSuite) TestRandomString9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString9", r)
		}
	}()

	n := -15

	RandomString(n)

}

func (s *RandomSuite) TestRandomString10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString10", r)
		}
	}()

	n := -54

	RandomString(n)

}

func (s *RandomSuite) TestRandomString11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString11", r)
		}
	}()

	n := -72

	RandomString(n)

}

func (s *RandomSuite) TestRandomString12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString12", r)
		}
	}()

	n := 56

	RandomString(n)

}

func (s *RandomSuite) TestRandomString13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString13", r)
		}
	}()

	n := 12

	RandomString(n)

}

func (s *RandomSuite) TestRandomString14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString14", r)
		}
	}()

	n := -63

	RandomString(n)

}

func (s *RandomSuite) TestRandomString15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString15", r)
		}
	}()

	n := 75

	RandomString(n)

}

func (s *RandomSuite) TestRandomString16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString16", r)
		}
	}()

	n := 98

	RandomString(n)

}

func (s *RandomSuite) TestRandomString17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestRandomString17", r)
		}
	}()

	n := 61

	RandomString(n)

}

func (s *RandomSuite) Testinit0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit0", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit1", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit2", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit3", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit4", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit5", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit6", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit7", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit8", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit9", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit10", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit11", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit12", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit13", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit14", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit15", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit16", r)
		}
	}()

	init()

}

func (s *RandomSuite) Testinit17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in Testinit17", r)
		}
	}()

	init()

}

func TestRandomSuite(t *testing.T) {
	suite.Run(t, new(RandomSuite))
}
