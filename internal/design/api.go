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
	Server("api", func() {
		Description("calcsvr hosts the Calculator Service.")

		// List the services hosted by this server.

		Services("flight", "booking")

		// List the Hosts and their transport URLs.
		Host("production", func() {
			Description("Production host.")
			// URIs can be parameterized using {param} notation.
			URI("https://{version}.goa.design/calc")
			URI("grpcs://{version}.goa.design")

			// Variable describes a URI variable.
			Variable("version", String, "API version", func() {
				// URI parameters must have a default value and/or an
				// enum validation.
				Default("v1")
			})
		})

		Host("development", func() {
			Description("Development hosts.")
			// Transport specific URLs, supported schemes are:
			// 'http', 'https', 'grpc' and 'grpcs' with the respective default
			// ports: 80, 443, 8080, 8443.
			URI("http://localhost:80/calc")
			URI("grpc://localhost:8080")
		})

	})
})

// var _ = Service("flight", func() {
// 	Method("search", func() {
// 		Description("Search for available flights")
//
// 		Payload(FlightSearchRequestPrams)
//
// 		Result(CollectionOf(FlightResult))
//
// 		Error("NotFound")
// 		Error("BadRequest")
// 		HTTP(func() {
// 			GET("")
// 			Response(StatusOK)
//
// 		})
// 	})
//
// })
