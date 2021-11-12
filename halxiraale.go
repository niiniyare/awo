package main

import (
	"fmt"
)

func main() {

	cali := 8

	caasha := 0

	cibaado := 0
	fmt.Println(cibaado, caasha, cali)


	if caasha == 0 {

		caasha = cali - 5
	}

	if caasha == 3 {

		cibaado = caasha

		caasha = 0

		cali = 5
	}

	fmt.Println(cibaado, caasha, cali)

	if caasha == 0 {

		caasha = cali - 2

		cibaado += caasha - 1

		cali = 2

		caasha = 1
	}
	fmt.Println(caasha, cibaado, cali)

	if cibaado == 5 {

		cali += cibaado

		cibaado = 0

		cibaado += caasha

		caasha = 0
	}
	if cali == 7 {

		caasha = cali - 4
	}

	fmt.Println(cibaado, cali, caasha)

	cibaado += caasha

	caasha = 0

	cali -= 3

	fmt.Println(cibaado, cali)

}
