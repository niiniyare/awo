package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

var file = "cmd/airport/airports.csv"

func main() {
	// airport.PrintSlice()
	airport.PrintSql()

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

var airport Airport

func (a Airport) PrintSql() {
	no := 0
	data, err := airport.ReadAAirportFromCsvFile(file)

	if err != nil {
		log.Fatal(err)

	}

	for _, dt := range data {
		if dt.Iata == "" || dt.Icao == "" {
			continue
		}
		name := strings.ReplaceAll(dt.Name, "'", "''")
		city := strings.ReplaceAll(dt.City, "'", "''")
		state := strings.ReplaceAll(dt.State, "'", "''")

		if len(dt.Iata) > 0 && len(dt.Iata) < 4 && dt.City != "" && dt.Name != "" && dt.Tz != "" && dt.Iata != "" && dt.Icao != "" && dt.Country != "" && dt.Elevation != 0 && dt.Lat != 0.0 || dt.Lon != 0.0 {
			fmt.Printf("INSERT INTO airports(iata_code, icao_code, name, elevation, city, country, state, lat, lon, timezone)")

			fmt.Printf("VALUES('%v','%v','%v',%d,'%v','%v','%v',%f,%f,'%v');\n", dt.Iata, dt.Icao, name, dt.Elevation, city, dt.Country, state, dt.Lon, dt.Lat, dt.Tz)

			no++

		}
	}
	print(no)
}

func (a Airport) ReadAAirportFromCsvFile(filename string) ([]Airport, error) {
	airport := []Airport{}

	// Open CSV file
	fileContent, err := os.Open(filename)
	if err != nil {
		return airport, err
	}
	defer fileContent.Close()
	// Read File into a Variable
	lines, err := csv.NewReader(fileContent).ReadAll()
	if err != nil {
		return airport, err
	}
	for _, line := range lines {
		lat, _ := strconv.ParseFloat(line[7], 64)
		lon, _ := strconv.ParseFloat(line[8], 64)
		elevation, _ := strconv.ParseInt(line[6], 10, 32)

		dt := Airport{
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
		airport = append(airport, dt)
	}
	return airport, nil

}

type Airline struct {
}

func (a Airline) ReadAAirlineFromCsvFile(filename string) ([][]string, error) {
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
func (a Airport) PrintSlice() {
	//	no := 0
	data, err := airport.ReadAAirportFromCsvFile(file)

	if err != nil {
		log.Fatal(err)

	}
	airportSample := []string{}
	fmt.Printf("package main\n\n var AirportSample = []string{\n")
	for i, dt := range data {
		if i == 0 {
			continue
		}
		if dt.Iata != "" {
			airportSample = append(airportSample, dt.Iata)
		}
	}

	for _, v := range airportSample {
		fmt.Printf("\t\"%v\",\n", v)

	}
	fmt.Print("}")
}
