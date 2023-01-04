package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = API("flight", func() {
	Title("Flight Service")
	Description("Service for creating, deleteing, updating and searching for available flights")
	Version("0.1")
	// Terms of Service for the API
	TermsOfService("Terms of use")
	Contact(func() {
		Name("API Support")
		Email("support@example.com")
		URL("https://example.com/api-support")
	})
	// License details for the API
	// License(func() {
	// 	Name("API License")
	// 	URL("https://example.com/api-license")
	// })
	// Documentation page for the API
	// Docs(func() {
	// 	Description("API Documentation")
	// 	URL("https://example.com/api-docs")
	// })
	Server("http://localhost:8080")
})

var _ = Service("flight", func() {
	Method("search", func() {
		Description("Search for available flights")

		Payload(FlightSearchPrams)

		Result(CollectionOf(FlightResult))

		Error("NotFound")
		Error("BadRequest")
		HTTP(func() {
			GET("")
			Response(StatusOK)

		})
	})

})
