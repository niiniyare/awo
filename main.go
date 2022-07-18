package main

import (
	"fmt"
	
	"github.com/niiniyare/awo/util"
)

func main() {

	fmt.Println(util.RandomPoint())

}

/*now := time.Now()
	for a := 1; a < 10000; a++ {
		//	RandomOwner()
		fmt.Printf("%d:  %v \n", a, util.RandomString(6))

	}
	diff := now.Sub(time.Now())

	fmt.Println("Elapsed MiliSeconds: ", diff.Milliseconds())
}
*/
