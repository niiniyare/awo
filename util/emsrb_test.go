// Coverage template
package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type EmsrbSuite struct {
	suite.Suite
}

func (s *EmsrbSuite) TestcalcEMSRb0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb0", r)
		}
	}()

	fares := []float64{-0.499931}
	demands := []float64{11.246564, 1.043466, 33.181973, -40.185454, -67.984670}
	sigmas := []float64{82.375927}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb1", r)
		}
	}()

	fares := []float64{44.051786, 4.748240, 22.430165, 30.515804, -75.561075, -45.482918, -98.683279, 26.230939, -52.775602}
	demands := []float64{15.236814, 41.159194, -40.012214, 64.279253, 10.586368, -25.741305, 54.074171}
	sigmas := []float64{89.399788, -60.126178, -9.432042, -28.110552, 58.501857, -69.369440, 24.482275, -63.032509, -64.575331}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb2", r)
		}
	}()

	fares := []float64{-87.844447, 51.203116}
	demands := []float64{}
	sigmas := []float64{-86.856513, 50.306934, -54.459490, -14.416028}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb3", r)
		}
	}()

	fares := []float64{-9.642466}
	demands := []float64{-61.215907, 26.985851, 81.527990}
	sigmas := []float64{-82.896173, -0.824194, 11.286066, 4.917652, 62.853539, -98.097333, -42.636438, 42.336994, 8.923823}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb4", r)
		}
	}()

	fares := []float64{-19.347645, 81.038138, 17.392024, 30.519484, 60.192349, 81.748875}
	demands := []float64{-38.227248, -29.410501, 21.828412}
	sigmas := []float64{28.484842, 24.768745, 81.301092, 44.916236}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb5", r)
		}
	}()

	fares := []float64{48.853787}
	demands := []float64{19.060985, -0.526022, -89.410745, -20.901631, -56.338682, 9.900555, 99.684452, 3.644224}
	sigmas := []float64{5.180592, -96.561942, -3.704931, -68.632409, 88.872396}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb6", r)
		}
	}()

	fares := []float64{96.160644, 14.122941, 2.250398, 50.973178, 71.704676, -52.969996}
	demands := []float64{-61.615908}
	sigmas := []float64{-87.552405, -11.997743, -21.181512}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb7", r)
		}
	}()

	fares := []float64{66.391817, -17.109457, 61.637289, -46.649456, -55.789748, 25.797073}
	demands := []float64{23.764670, 59.928199, 2.015812, 39.296713, -73.291111}
	sigmas := []float64{19.481256, 68.823105, 8.703842, -97.941746}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb8", r)
		}
	}()

	fares := []float64{4.849151, -95.943126, 93.532864}
	demands := []float64{}
	sigmas := []float64{}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb9", r)
		}
	}()

	fares := []float64{2.159763, -88.135247, -57.874835, -74.023396, -60.021016}
	demands := []float64{-79.734841, -83.220746, 48.450451, 72.868236, 47.517627, -40.126066}
	sigmas := []float64{23.360751, -57.912622, -17.100275, -3.854226, -46.935078, -18.731018}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb10", r)
		}
	}()

	fares := []float64{-1.610077, 58.587790, 77.472199, -25.184813, 6.126440, 30.489428}
	demands := []float64{}
	sigmas := []float64{-12.593870, -47.502149, 90.135191, 80.762962, -2.441409, 84.232165}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb11", r)
		}
	}()

	fares := []float64{-50.614607, 43.934773, -18.004296}
	demands := []float64{-84.234924}
	sigmas := []float64{-17.245002, -17.086226, -72.325736}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb12", r)
		}
	}()

	fares := []float64{-27.313883, -28.932046}
	demands := []float64{-46.771149, 24.916427, 74.168618, 31.055960}
	sigmas := []float64{87.263356, 53.878194, 58.103932, 74.009916, 45.603860, -35.180476}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb13", r)
		}
	}()

	fares := []float64{}
	demands := []float64{-53.003227, 84.698619, 39.475270, 91.043633, -88.915297, 35.610747, 53.163729}
	sigmas := []float64{6.340692, 85.371017}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb14", r)
		}
	}()

	fares := []float64{90.752210, 75.718796}
	demands := []float64{43.256605, -16.834894, 78.104166, -35.357484, 66.622793, -3.284928, -23.728082, 97.237085}
	sigmas := []float64{-53.181503, -40.566743, 60.791108, -16.091936, 38.882553}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb15", r)
		}
	}()

	fares := []float64{59.324741, 51.342557, -37.543608, -88.617357, 40.791215, -17.733167, -88.947297}
	demands := []float64{-20.862653, -68.082934, -4.407495, 98.490688, -86.452336, -47.375691, 29.548170, 21.252013, -13.743585}
	sigmas := []float64{-96.288317, 0.199758, -96.097739, 2.767254, 3.678549, 24.174343, 4.806703}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb16", r)
		}
	}()

	fares := []float64{66.148513, -61.675992, 9.300938}
	demands := []float64{88.200346, -80.997054, 48.809188, 6.188151, -55.994055, 8.014540, -84.797107, -35.297098}
	sigmas := []float64{-13.665123, -4.130196, -24.485407, 9.968780}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestcalcEMSRb17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestcalcEMSRb17", r)
		}
	}()

	fares := []float64{31.168140, -46.019251, 71.502682, -5.499027, -92.973862, 86.564343, 10.297952, -43.082708, 16.227034}
	demands := []float64{90.201751, -13.739563, -54.207241}
	sigmas := []float64{-49.512879, 89.579793, 62.010499, -13.521982, -35.253686, -60.213404, -48.150321}

	calcEMSRb(fares, demands, sigmas)

}

func (s *EmsrbSuite) TestnormPPF0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF0", r)
		}
	}()

	p := 32.955181

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF1", r)
		}
	}()

	p := -19.275408

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF2", r)
		}
	}()

	p := -46.398865

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF3", r)
		}
	}()

	p := 78.874302

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF4", r)
		}
	}()

	p := 29.368545

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF5", r)
		}
	}()

	p := 41.124192

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF6", r)
		}
	}()

	p := 67.524884

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF7", r)
		}
	}()

	p := -39.602840

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF8", r)
		}
	}()

	p := -47.078741

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF9", r)
		}
	}()

	p := -38.084419

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF10", r)
		}
	}()

	p := -95.629475

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF11", r)
		}
	}()

	p := 16.645829

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF12", r)
		}
	}()

	p := -40.390717

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF13", r)
		}
	}()

	p := 17.906146

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF14", r)
		}
	}()

	p := -44.813770

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF15", r)
		}
	}()

	p := 26.081317

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF16", r)
		}
	}()

	p := 9.436446

	normPPF(p)

}

func (s *EmsrbSuite) TestnormPPF17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestnormPPF17", r)
		}
	}()

	p := -24.737001

	normPPF(p)

}

func TestEmsrbSuite(t *testing.T) {
	suite.Run(t, new(EmsrbSuite))
}
