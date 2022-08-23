package util

type AircraftList struct {
  Aircrafts       []Aircraft   `json:"aircrafts,omitempty"`
}
type Aircraft struct {
	AircraftCode string `json:"aircraft_code,omitempty"`
	Model        string `json:"model,omitempty"`
	Range        string `json:"range,omitempty"`
}
