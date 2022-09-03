package util

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

var aircraftQueryInsert string = "INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)"
var irportQueryValues string = "VALUES( '%v','%v', '%v', '%v','%v','%v','%v','%2.9f', '%2.9f', '%v');"

type Aircraft struct {
	Name     string `json:"Name,omitempty"`
	Icao     string `json:"ICAO,omitempty"`
	Iata     string `json:"IATA,omitempty"`
	Capacity int    `json:"Capacity,omitempty"`
	Country  string `json:"country,omitempty"`
}

var aircraft Aircraft

func (a Aircraft) Print() {
	no := 0
	data, err := aircraft.ReadAircraftFromCsvFile("util/sample_data/aircrafts.csv")

	if err != nil {
		log.Fatal(err)

	}

	for _, dt := range data {
		if dt.Iata == "" || dt.Icao == "" {
			continue
		}
		name := strings.ReplaceAll(dt.Name, "'", "")
		city := strings.ReplaceAll(dt.City, "'", "")
		state := strings.ReplaceAll(dt.State, "'", "")

		if len(dt.Iata) > 0 && len(dt.Iata) < 4 && dt.City != "" && dt.Name != "" && dt.Tz != "" && dt.Iata != "" && dt.Icao != "" && dt.Country != "" && dt.Elevation != 0 && dt.Lat != 0.0 || dt.Lon != 0.0 {
			fmt.Printf("INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)")

			fmt.Printf("VALUES('%v','%v','%v',%d,'%v','%v','%v',%f,%f,'%v');\n", dt.Iata, dt.Icao, name, dt.Elevation, city, state, dt.Country, dt.Lon, dt.Lat, dt.Tz)

			no++

		}
	}
	print(no)
}

func (a Aircraft) ReadAircraftFromCsvFile(filename string) ([]Aircraft, error) {
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
		lat, _ := strconv.ParseFloat(line[7], 64)
		lon, _ := strconv.ParseFloat(line[8], 64)
		elevation, _ := strconv.ParseInt(line[6], 10, 32)

		dt := Aircraft{
			Icao:      line[0],
			Iata:      line[1],
			Name:      line[2],
			City:      line[3],
			State:     line[4],
			Country:   line[5],
			Elevation: int(elevation),
			Lat:       lat,
			Lon:       lon,
			Tz:        line[9],
		}
		aircraft = append(aircraft, dt)
	}
	return aircraft, nil

}
