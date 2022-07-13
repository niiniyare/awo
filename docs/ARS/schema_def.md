
1. ### **Schema Description**

The main entity is a booking `(bookings)`.

One booking can include several passengers, with a separate ticket `(tickets)` issued to each passenger. A ticket has a unique number and includes information about the passenger. As such, the passenger is not a separate entity. Both the passenger's name and identity document number can change over time, so it is impossible to uniquely identify all the tickets of a particular person; for simplicity, we can assume that all passengers are unique.

The ticket includes one or more flight segments `(ticket_flights)`. Several flight segments can be included into a single ticket if there are no non-stop flights between the points of departure and destination `(connecting flights)`, or if it is a round-trip ticket. Although there is no constraint in the schema, it is assumed that all tickets in the booking have the same flight segments.

Each flight `(flights)` goes from one airport `(airports)` to another. Flights with the same flight number have the same points of departure and destination, but differ in departure date.

At flight check-in, the passenger is issued a boarding pass `(boarding_passes)`, where the seat number is specified. The passenger can check in for the flight only if this flight is included into the ticket. The flight-seat combination must be unique to avoid issuing two boarding passes for the same seat.

The number of seats `(seats)` in the aircraft and their distribution between different travel classes depends on the model of the aircraft `(aircrafts)` performing the flight. It is assumed that every aircraft model has only one cabin configuration. Database schema does not check that seat numbers in boarding passes have the corresponding seats in the aircraft (such verification can be done using table triggers, or at the application level).

## **1.4.1. List of Relations**

`
              List of relations
  Schema  |      Name       | Type  |  Owner  
----------+-----------------+-------+---------
 bookings | aircrafts_data  | table | admin
 bookings | airports_data   | table | admin
 bookings | boarding_passes | table | admin
 bookings | bookings        | table | admin
 bookings | flights         | table | admin
 bookings | seats           | table | admin
 bookings | ticket_flights  | table | admin
 bookings | tickets         | table | admin
`
