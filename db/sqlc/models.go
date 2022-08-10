// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.14.0

package db

import (
	"database/sql"
	"time"
)

// Aircrafts (internal data)
type Aircraft struct {
	ID int64 `json:"id"`
	// Aircraft code, IATA
	Code string `json:"code"`
	// Aircraft model
	Model string `json:"model"`
	// Maximal flying distance, km
	Range     int32     `json:"range"`
	CompanyID int64     `json:"company_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Airline struct {
	ID          int64     `json:"id"`
	CompanyName string    `json:"company_name"`
	IataCode    string    `json:"iata_code"`
	MainAirport string    `json:"main_airport"`
	CreatedAt   time.Time `json:"created_at"`
}

type Airport struct {
	ID       int64  `json:"id"`
	IataCode string `json:"iata_code"`
	IcaoCode string `json:"icao_code"`
	Name     string `json:"name"`
	City     string `json:"city"`
	Timezone string `json:"timezone"`
}

type Flight struct {
	FlightID           int64        `json:"flight_id"`
	FlightNo           string       `json:"flight_no"`
	CompanyID          int64        `json:"company_id"`
	ScheduledDeparture time.Time    `json:"scheduled_departure"`
	ScheduledArrival   time.Time    `json:"scheduled_arrival"`
	DepartureAirport   string       `json:"departure_airport"`
	ArrivalAirport     string       `json:"arrival_airport"`
	Status             string       `json:"status"`
	AircraftID         int64        `json:"aircraft_id"`
	ActualDeparture    sql.NullTime `json:"actual_departure"`
	ActualArrival      sql.NullTime `json:"actual_arrival"`
}

type FlightsV struct {
	// Flight ID
	FlightID int64 `json:"flight_id"`
	// Flight number
	FlightNo  string `json:"flight_no"`
	CompanyID int64  `json:"company_id"`
	// Scheduled departure time
	ScheduledDeparture time.Time `json:"scheduled_departure"`
	// Scheduled departure time, local time at the point of departure
	ScheduledDepartureLocal interface{} `json:"scheduled_departure_local"`
	// Scheduled arrival time
	ScheduledArrival time.Time `json:"scheduled_arrival"`
	// Scheduled arrival time, local time at the point of destination
	ScheduledArrivalLocal interface{} `json:"scheduled_arrival_local"`
	// Scheduled flight duration
	ScheduledDuration int32 `json:"scheduled_duration"`
	// Deprature airport code
	DepartureAirport string `json:"departure_airport"`
	// Departure airport name
	DepartureAirportName string `json:"departure_airport_name"`
	// City of departure
	DepartureCity string `json:"departure_city"`
	// Arrival airport code
	ArrivalAirport string `json:"arrival_airport"`
	// Arrival airport name
	ArrivalAirportName string `json:"arrival_airport_name"`
	// City of arrival
	ArrivalCity string `json:"arrival_city"`
	// Flight status
	Status string `json:"status"`
	// Aircraft code, IATA
	AircraftID int64 `json:"aircraft_id"`
	// Actual departure time
	ActualDeparture sql.NullTime `json:"actual_departure"`
	// Actual departure time, local time at the point of departure
	ActualDepartureLocal interface{} `json:"actual_departure_local"`
	// Actual arrival time
	ActualArrival sql.NullTime `json:"actual_arrival"`
	// Actual arrival time, local time at the point of destination
	ActualArrivalLocal interface{} `json:"actual_arrival_local"`
	// Actual flight duration
	ActualDuration int32 `json:"actual_duration"`
}

type Route struct {
	// Flight number
	FlightNo  string `json:"flight_no"`
	CompanyID int64  `json:"company_id"`
	// Code of airport of departure
	DepartureAirport string `json:"departure_airport"`
	// Name of airport of departure
	DepartureAirportName string `json:"departure_airport_name"`
	// City of departure
	DepartureCity string `json:"departure_city"`
	// Code of airport of arrival
	ArrivalAirport string `json:"arrival_airport"`
	// Name of airport of arrival
	ArrivalAirportName string `json:"arrival_airport_name"`
	// City of arrival
	ArrivalCity string `json:"arrival_city"`
	// Aircraft code, IATA
	AircraftID int64 `json:"aircraft_id"`
	// Scheduled duration of flight
	Duration int32 `json:"duration"`
	// Days of week on which flights are scheduled
	DaysOfWeek interface{} `json:"days_of_week"`
}
