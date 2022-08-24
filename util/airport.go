package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type AirportList struct {
	Airport SampaleAirports `json:"airport,omitempty"`
}
//go:generate ffjson $GOFILE
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
/*
func ReadFromJsonFile(path string) (airport []AirportList, err error) {

	// f, err := os.Open(path)
	// if err != nil {
	// 	fmt.Errorf("cannot open file: %w", err)
	// 	return data, nil
	// }
	//
	var file []byte
	file, err = ioutil.ReadFile(path)

	if err != nil {
		fmt.Errorf("cannot ReadFile file: %w", err)
		return
	}

	err = json.Unmarshal(file, &airport)
	if err != nil {
		fmt.Errorf("cannot Unmarshal to from json: %w", err)
		return

	}

	return
}
*/