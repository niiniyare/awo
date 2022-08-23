package util

type AirportList struct {
	Airports []Airport `json:"airports,omitempty"`
}
type Airport struct {
	AirportCode string `json:"airport_code,omitempty"`
	AirportName string `json:"airport_name,omitempty"`
	City        string `json:"city,omitempty"`
	Coordinates string `json:"coordinates,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
}
