package main

import (
	"crypto/rand"
	"fmt"
)

func main() {
	RandomCrypto, _ := rand.Prime(rand.Reader, 8)
	fmt.Println(RandomCrypto)
}
