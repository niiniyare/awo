// Coverage template
package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
}

func (s *ConfigSuite) TestLoadConfig0() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig0", r)
		}
	}()

	path := "Kaylie Wuckert"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig1() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig1", r)
		}
	}()

	path := "Mariane McLaughlin"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig2() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig2", r)
		}
	}()

	path := "Gerhard Beier"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig3() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig3", r)
		}
	}()

	path := "Tierra Robel"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig4() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig4", r)
		}
	}()

	path := "Lizzie Bartoletti"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig5() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig5", r)
		}
	}()

	path := "Emanuel Considine"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig6() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig6", r)
		}
	}()

	path := "Fredrick Stehr"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig7() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig7", r)
		}
	}()

	path := "Marley Hessel"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig8() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig8", r)
		}
	}()

	path := "Arvilla Stracke"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig9() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig9", r)
		}
	}()

	path := "Lessie Bartell"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig10() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig10", r)
		}
	}()

	path := "Barry Pfeffer"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig11() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig11", r)
		}
	}()

	path := "Abbey Crist"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig12() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig12", r)
		}
	}()

	path := "Kole Aufderhar"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig13() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig13", r)
		}
	}()

	path := "Winfield Gottlieb"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig14() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig14", r)
		}
	}()

	path := "Jackeline Feeney"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig15() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig15", r)
		}
	}()

	path := "Nannie McCullough"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig16() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig16", r)
		}
	}()

	path := "Kirstin Williamson"

	LoadConfig(path)

}

func (s *ConfigSuite) TestLoadConfig17() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in TestLoadConfig17", r)
		}
	}()

	path := "Mae Glover"

	LoadConfig(path)

}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
