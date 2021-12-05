/*
 * AirGateway NDC JSON API
 *
 * This API allows shopping and booking with IATA's New Distribution Capabilities (NDC) standard. It provides aggregated shopping capabilities (AirShopping), detailed offer description (OfferPrice), flight seat selection (SeatAvailability) and booking flight reservations (OrderCreate). Some fields in our API (when noticed) use the PADIS Standard v16.1. Find more information <a href=http://www.iata.org/whatwedo/workgroups/Pages/padis.aspx>here</a>
 *
 * API version: 1.1
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package swagger

type FlightPointDataRsArr struct {

	// Departure/Arrival airport IATA three letter code.
	AirportCode string `json:"airportCode"`

	// Departure/Arrival airport name.
	AirportName string `json:"airportName,omitempty"`

	// Country ID data
	CountryID string `json:"countryID,omitempty"`

	// Departure/Arrival date in format YYYY-MM-DD.
	Date string `json:"date,omitempty"`

	// Parent Location data
	ParentLocation string `json:"parentLocation,omitempty"`

	// Departure/Arrival terminal name.
	TerminalName string `json:"terminalName,omitempty"`

	// Preferred departure time in format HH:MM 24h.
	Time string `json:"time,omitempty"`
}