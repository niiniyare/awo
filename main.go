package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	s := "2022-03-23T07:00:00+01:00"
	loc, errs := time.LoadLocation("Africa/Mogadishu")
	if errs != nil {                                                         log.Fatal(errs)                                          }
	t, err := time.ParseInLocation(time.RFC3339, s, loc)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(t)
}
