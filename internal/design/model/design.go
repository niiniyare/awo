package design

import . "goa.design/model/dsl"

var _ = Design("Flight Reservation System", "This is a model of a flight reservation system.", func() {
	// Elements
	var System = SoftwareSystem("Flight Reservation System", "A flight reservation system that allows users to search and book flights.")
	var User = Person("User", "A user of the flight reservation system.")
	var Airline = SoftwareSystem("Airline", "An airline company that offers flights.")
	var Airport = SoftwareSystem("Airport", "An airport that serves as a departure or arrival point for flights.")

	// Relationships
	User.Uses(System, "Uses")
	System.Uses(Airline, "Sends booking requests to")
	System.Uses(Airport, "Gets flight information from")

	// Views
	SystemContextView(System, "SystemContext", "An example of a System Context diagram.", func() {
		AddAll()
		AutoLayout(RankLeftRight)
	})
	Styles(func() {
		ElementStyle("system", func() {
			Background("#1168bd")
			Color("#ffffff")
		})
		ElementStyle("person", func() {
			Shape(ShapePerson)
		})
	})
})
