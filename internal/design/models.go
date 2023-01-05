package design

import (
	. "goa.design/goa/v3/dsl"
)

var Aircraft = ResultType("aircraft", func() {
	Attribute("id", Int64, "aircraft system scope Unique Id")
	Attribute("iata_code", String, "an internation Unique code issued by Iata")
	Attribute("icao_code", String, "an internation Unique code issued by Icao")
	Attribute("model", String, "Aircraft model")
	Attribute("range", Int64, "Maximal flying distance, km")
	Attribute("company_id", Int64, "airline company that operating this aircraft")
	Attribute("created_at", String, func() {
		Format(FormatDateTime)
	})

	Required("id", "iata_code", "icao_code", "model", "range", "company_id")

	View("default", func() {
		Attribute("id")
		Attribute("iata_code")
		Attribute("icao_code")
		Attribute("model")
		Attribute("range")
		Attribute("company_id")
		Attribute("created_at")
	})
})

var Airline = ResultType("airline", func() {
	Attribute("id", Int64, "Unique for the airline company Id in a system Scope")
	Attribute("company_name", String, "")
	Attribute("iata_code", String, "")
	Attribute("main_airport", String, "")
	Attribute("created_at", String, func() {
		Format(FormatDateTime)
	})

	Required("id", "company_name", "iata_code", "main_airport")

	View("default", func() {
		Attribute("id")
		Attribute("iata_code")
		Attribute("company_name")
		Attribute("main_airport")
	})
})

var AircraftResultType = ResultType("application/vnd.aircraft+json", func() {
	Attributes(func() {
		Attribute("id", Int64, "ID of the aircraft")
		Attribute("iata_code", String, "Aircraft code, IATA")
		Attribute("icao_code", String, "Aircraft code, ICAO")
		Attribute("model", String, "Aircraft model")
		Attribute("range", Int32, "Maximal flying distance, km")
		Attribute("company_id", Int64, "ID of the company that owns the aircraft")
		Attribute("created_at", String, func() {

			Description("Time the aircraft was created")
			Format(FormatDateTime)
		})

	})
})

var AirlineResultType = ResultType("application/vnd.airline+json", func() {
	Attributes(func() {
		Attribute("id", Int64, "ID of the airline company")
		Attribute("company_name", String, "Name of the airline company")
		Attribute("iata_code", String, "IATA code of the airline company")
		Attribute("main_airport", String, "Main airport of the airline company")
		Attribute("created_at", String, func() {

			Description("Time the airline company was created")
			Format(FormatDateTime)
		})
	})
})

var AirportResultType = ResultType("application/vnd.airport+json", func() {
	Attributes(func() {
		Attribute("id", Int64, "ID of the airport")
		Attribute("iata_code", String, "IATA code of the airport")
		Attribute("icao_code", String, "ICAO code of the airport")
		Attribute("name", String, "Name of the airport")
		Attribute("country", String, "Country the airport is located in")
		Attribute("state", String, "State the airport is located in")
		Attribute("city", String, "City the airport is located in")
		Attribute("elevation", String, "Elevation of the airport")
		Attribute("lat", Float64, "Latitude of the airport")
		Attribute("lon", Float64, "Longitude of the airport")
		Attribute("timezone", String, "Time zone of the airport")
	})
})

var BookedSeatResultType = ResultType("application/vnd.bookedseat+json", func() {
	Attributes(func() {
		Attribute("id", Int64, "ID of the booked seat")
		Attribute("flight_id", Int32, "ID of the flight the seat is booked on")
		Attribute("cabin_class", String, "Cabin class of the booked seat")
		Attribute("seat_row", Int32, "Row of the booked seat")
		Attribute("seat_column", String, "Column of the booked seat")
	})
})

// var CabinClassResultType = ResultType("application/vnd.cabinclass+json", func() {
// 	Attributes(func() {
// 		Attribute("value", String, "Value of the cabin class")
// 		Attribute("description", String, "Description of the cabin class")
//
