package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = Service("flight", func() {
	Method("search", func() {
		Description("Search for available flights")

		Payload(FlightSearchRequestPrams)

		Result(CollectionOf(FlightSearchResponse))

		Error("NotFound")
		Error("BadRequest")
		HTTP(func() {
			GET("")
			Response(StatusOK)

		})
	})

})
