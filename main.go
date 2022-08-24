package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

type AirportList struct {
	Airports Airport `json:"airport,omitempty"`
}

type Airport struct {
	Icao      string  `json:"icao,omitempty"`
	Iata      string  `json:"iata,omitempty"`
	Name      string  `json:"name,omitempty"`
	City      string  `json:"city,omitempty"`
	State     string  `json:"state,omitempty"`
	Country   string  `json:"country,omitempty"`
	Elevation int     `json:"elevation,omitempty"`
	Lat       float64 `json:"lat,omitempty"`
	Lon       float64 `json:"lon,omitempty"`
	Tz        string  `json:"tz,omitempty"`
}

func main() {
	csvData, err := ReadCsvFile("util/sample_data/airports.csv")
	if err != nil {
		fmt.Println(err.Error())
	}

	for i, line := range csvData {
		lat, _ := strconv.ParseFloat(line[7], 64)
		lon, _ := strconv.ParseFloat(line[8], 64)
		elevation, _ := strconv.ParseInt(line[6], 10, 32)
		data := Airport{
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
		fmt.Println(i, data.Icao+" "+data.Iata)
	}

}
func ReadCsvFile(filename string) ([][]string, error) {
	// Open CSV file
	fileContent, err := os.Open(filename)
	if err != nil {
		return [][]string{}, err
	}
	defer fileContent.Close()

	// Read File into a Variable
	lines, err := csv.NewReader(fileContent).ReadAll()
	if err != nil {
		return [][]string{}, err
	}
	return lines, nil
}
