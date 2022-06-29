package main

import (
	"fmt"
	"time"
	"github.com/niiniyare/awo/util"
)

func main() {
  now:=time.Now()
	for a := 1; a < 10000; a++ {
		//	RandomOwner()

		fmt.Printf("%d:  %v \n", a, util.RandomString(6))

	}
diff:=now.Sub(time.Now())

fmt.Println("Elapsed MiliSeconds: ",diff.Milliseconds())
}
