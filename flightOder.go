package main

type FlightOffersResponse struct {
	Data Data `json:"data,omitempty"`
}
type Departure struct {
	IataCode string `json:"iataCode,omitempty"`
	Terminal string `json:"terminal,omitempty"`
	At       string `json:"at,omitempty"`
}
type Arrival struct {
	IataCode string `json:"iataCode,omitempty"`
	Terminal string `json:"terminal,omitempty"`
	At       string `json:"at,omitempty"`
}
type Aircraft struct {
	Code string `json:"code,omitempty"`
}
type Operating struct {
	CarrierCode string `json:"carrierCode,omitempty"`
}

type Segments struct {
	Departure     Departure `json:"departure,omitempty"`
	Arrival       Arrival   `json:"arrival,omitempty"`
	CarrierCode   string    `json:"carrierCode,omitempty"`
	Number        string    `json:"number,omitempty"`
	Aircraft      Aircraft  `json:"aircraft,omitempty"`
	Operating     Operating `json:"operating,omitempty"`
	Duration      string    `json:"duration,omitempty"`
	ID            string    `json:"id,omitempty"`
	NumberOfStops int       `json:"numberOfStops,omitempty"`
	// Arrival0      Arrival   `json:"arrival,omitempty"`
	// Departure0    Departure `json:"departure,omitempty"`
}
type Itineraries struct {
	Segments []Segments `json:"segments,omitempty"`
}
type Fees struct {
	Amount string `json:"amount,omitempty"`
	Type   string `json:"type,omitempty"`
}
type Price struct {
	Currency        string `json:"currency,omitempty"`
	Total           string `json:"total,omitempty"`
	Base            string `json:"base,omitempty"`
	Fees            []Fees `json:"fees,omitempty"`
	GrandTotal      string `json:"grandTotal,omitempty"`
	BillingCurrency string `json:"billingCurrency,omitempty"`
}
type PricingOptions struct {
	FareType                []string `json:"fareType,omitempty"`
	IncludedCheckedBagsOnly bool     `json:"includedCheckedBagsOnly,omitempty"`
}
type Taxes struct {
	Amount string `json:"amount,omitempty"`
	Code   string `json:"code,omitempty"`
}

// type Price struct {
// 	Currency string  `json:"currency,omitempty"`
// 	Total    string  `json:"total,omitempty"`
// 	Base     string  `json:"base,omitempty"`
// 	Taxes    []Taxes `json:"taxes,omitempty"`
// }

type IncludedCheckedBags struct {
	Quantity int `json:"quantity,omitempty"`
}
type FareDetailsBySegment struct {
	SegmentID           string              `json:"segmentId,omitempty"`
	Cabin               string              `json:"cabin,omitempty"`
	FareBasis           string              `json:"fareBasis,omitempty"`
	BrandedFare         string              `json:"brandedFare,omitempty"`
	Class               string              `json:"class,omitempty"`
	IncludedCheckedBags IncludedCheckedBags `json:"includedCheckedBags,omitempty"`
}
type TravelerPricings struct {
	TravelerID           string                 `json:"travelerId,omitempty"`
	FareOption           string                 `json:"fareOption,omitempty"`
	TravelerType         string                 `json:"travelerType,omitempty"`
	Price                Price                  `json:"price,omitempty"`
	FareDetailsBySegment []FareDetailsBySegment `json:"fareDetailsBySegment,omitempty"`
}
type FlightOffers struct {
	Type                     string             `json:"type,omitempty"`
	ID                       string             `json:"id,omitempty"`
	Source                   string             `json:"source,omitempty"`
	InstantTicketingRequired bool               `json:"instantTicketingRequired,omitempty"`
	NonHomogeneous           bool               `json:"nonHomogeneous,omitempty"`
	PaymentCardRequired      bool               `json:"paymentCardRequired,omitempty"`
	LastTicketingDate        string             `json:"lastTicketingDate,omitempty"`
	Itineraries              []Itineraries      `json:"itineraries,omitempty"`
	Price                    Price              `json:"price,omitempty"`
	PricingOptions           PricingOptions     `json:"pricingOptions,omitempty"`
	ValidatingAirlineCodes   []string           `json:"validatingAirlineCodes,omitempty"`
	TravelerPricings         []TravelerPricings `json:"travelerPricings,omitempty"`
}
type Name struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
}
type Phones struct {
	DeviceType         string `json:"deviceType,omitempty"`
	CountryCallingCode string `json:"countryCallingCode,omitempty"`
	Number             string `json:"number,omitempty"`
}
type Contact struct {
	EmailAddress string   `json:"emailAddress,omitempty"`
	Phones       []Phones `json:"phones,omitempty"`
}
type Documents struct {
	DocumentType     string `json:"documentType,omitempty"`
	BirthPlace       string `json:"birthPlace,omitempty"`
	IssuanceLocation string `json:"issuanceLocation,omitempty"`
	IssuanceDate     string `json:"issuanceDate,omitempty"`
	Number           string `json:"number,omitempty"`
	ExpiryDate       string `json:"expiryDate,omitempty"`
	IssuanceCountry  string `json:"issuanceCountry,omitempty"`
	ValidityCountry  string `json:"validityCountry,omitempty"`
	Nationality      string `json:"nationality,omitempty"`
	Holder           bool   `json:"holder,omitempty"`
}
type Travelers struct {
	ID          string      `json:"id,omitempty"`
	DateOfBirth string      `json:"dateOfBirth,omitempty"`
	Name        Name        `json:"name,omitempty"`
	Gender      string      `json:"gender,omitempty"`
	Contact     Contact     `json:"contact,omitempty"`
	Documents   []Documents `json:"documents,omitempty"`
}
type General struct {
	SubType string `json:"subType,omitempty"`
	Text    string `json:"text,omitempty"`
}
type Remarks struct {
	General []General `json:"general,omitempty"`
}
type TicketingAgreement struct {
	Option string `json:"option,omitempty"`
	Delay  string `json:"delay,omitempty"`
}
type AddresseeName struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
}
type Address struct {
	Lines       []string `json:"lines,omitempty"`
	PostalCode  string   `json:"postalCode,omitempty"`
	CityName    string   `json:"cityName,omitempty"`
	CountryCode string   `json:"countryCode,omitempty"`
}
type Contacts struct {
	AddresseeName AddresseeName `json:"addresseeName,omitempty"`
	CompanyName   string        `json:"companyName,omitempty"`
	Purpose       string        `json:"purpose,omitempty"`
	Phones        []Phones      `json:"phones,omitempty"`
	EmailAddress  string        `json:"emailAddress,omitempty"`
	Address       Address       `json:"address,omitempty"`
}
type Data struct {
	Type               string             `json:"type,omitempty"`
	FlightOffers       []FlightOffers     `json:"flightOffers,omitempty"`
	Travelers          []Travelers        `json:"travelers,omitempty"`
	Remarks            Remarks            `json:"remarks,omitempty"`
	TicketingAgreement TicketingAgreement `json:"ticketingAgreement,omitempty"`
	Contacts           []Contacts         `json:"contacts,omitempty"`
}
