// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.13.0

package db

import (
	"time"
)

// Aircrafts (internal data)
type AircraftsDatum struct {
	// Aircraft code, IATA
	AircraftCode string `json:"aircraft_code"`
	// Aircraft model
	Model string `json:"model"`
	// Maximal flying distance, km
	Range     int32     `json:"range"`
	CompanyID int64     `json:"company_id"`
	CreatedAt time.Time `json:"created_at"`
}

type AirlineCompany struct {
	CompanyID   int64     `json:"company_id"`
	CompanyName string    `json:"company_name"`
	IataCode    string    `json:"iata_code"`
	MainAirport string    `json:"main_airport"`
	CreatedAt   time.Time `json:"created_at"`
}

// Airports (internal data)
type AirportsDatum struct {
	// Airport code
	AirportCode string `json:"airport_code"`
	// Airport name
	AirportName string `json:"airport_name"`
	// Country
	CountryCode string `json:"country_code"`
	// City
	City string `json:"city"`
	// Airport coordinates (longitude and latitude)
	Coordinates interface{} `json:"coordinates"`
	// Airport time zone
	Timezone string `json:"timezone"`
	// time airport record Created
	CreatedAt time.Time `json:"created_at"`
}
