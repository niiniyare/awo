package main

import "fmt"

func main() {
	turnedAUnicode := '\u0256' // -> latin small letter turned a unicode 
	fmt.Printf("Turned A symbol: %c\n", turnedAUnicode) 
	valueOnly := fmt.Sprintf("%c", turnedAUnicode) // If you only want the resulting string. 
	fmt.Println("Turned A value: ", valueOnly)
}
