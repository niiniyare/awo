package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type AirportList struct {
	Airport SampaleAirports `json:"airport,omitempty"`
}
type SampaleAirports struct {
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

func ReadFromJsonFile(path string) (SampaleAirports, error) {

	data := SampaleAirports{}
	f, err := os.Open(path)
	if err != nil {
		fmt.Errorf("cannot open file: %w", err)
		return data, nil
	}

	file, err = ioutil.ReadAll()

	if err != nil {
		fmt.Errorf("cannot ReadFile file: %w", err)
		return data, nil
	}

	err = json.Unmarshal([]byte(file), &data)
	if err != nil {
		fmt.Errorf("cannot Unmarshal to from json: %w", err)
		return data, nil

	}

	return data, nil
}
