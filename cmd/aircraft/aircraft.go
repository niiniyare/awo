package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"github.com/niiniyare/awo/util"
)

func main() {
	PrintAircraft()

}

/* var aircraft = data.Aircraft{} */

func PrintAircraft() {
	no := 0
	data, err := ReadAircraftFromCsvFile("cmd/aircraft/aircrafts.csv")

	if err != nil {
		log.Fatal(err)

	}

	for _, dt := range data {
		if dt.Iata == "" || dt.Icao == "" {
			continue
		}
		if len(dt.Iata) > 0 && len(dt.Iata) < 4 && dt.Name != "" {

			fmt.Printf("INSERT INTO aircrafts(iata_code, icao_code,  model, range, company_id)")

			fmt.Printf("VALUES( '%v','%v', '%v', %v,%v);\n", dt.Iata, dt.Icao, dt.Name, util.RandomInt(100, 800), util.RandomInt(1, 2))

			no++

		}
	}
	print(no)
}

type Aircraft struct {
	Name string `json:"Name,omitempty"`
	Icao string `json:"ICAO,omitempty"`
	Iata string `json:"IATA,omitempty"`
	/* 	Capacity int64  `json:"Capacity,omitempty"` */
	Country string `json:"country,omitempty"`
}

func ReadAircraftFromCsvFile(filename string) ([]Aircraft, error) {
	aircraft := []Aircraft{}

	// Open CSV file
	fileContent, err := os.Open(filename)
	if err != nil {
		return aircraft, err
	}
	defer fileContent.Close()
	// Read File into a Variable
	lines, err := csv.NewReader(fileContent).ReadAll()
	if err != nil {
		return aircraft, err
	}
	for _, line := range lines {

		// capacity, err := strconv.ParseInt(line[3], 10, 64)
		// if err != nil {
		// 	return aircraft, err
		// }
		//
		dt := Aircraft{
			Name: line[0],
			Icao: line[1],
			Iata: line[2],
			/* 			Capacity: int64(capacity), */
			Country: line[4],
		}
		aircraft = append(aircraft, dt)
	}
	return aircraft, nil

}
