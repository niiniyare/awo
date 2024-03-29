// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.14.0
// source: seat.sql

package db

import (
	"context"
)

const createSeat = `-- name: CreateSeat :one
INSERT INTO seats ( 
aircraft_id, 
seat_no, 
fare_conditions)
VALUES($1, $2, $3)
RETURNING aircraft_id, seat_no, fare_conditions
`

type CreateSeatParams struct {
	AircraftID     int64  `json:"aircraft_id"`
	SeatNo         string `json:"seat_no"`
	FareConditions string `json:"fare_conditions"`
}

func (q *Queries) CreateSeat(ctx context.Context, arg CreateSeatParams) (Seat, error) {
	row := q.db.QueryRowContext(ctx, createSeat, arg.AircraftID, arg.SeatNo, arg.FareConditions)
	var i Seat
	err := row.Scan(&i.AircraftID, &i.SeatNo, &i.FareConditions)
	return i, err
}

const getAircraftSeatsWithIataCode = `-- name: GetAircraftSeatsWithIataCode :many


SELECT   a.iata_code,
         a.icao_code,
         a.model,
         a.range,
         s.seat_no,
         s.fare_conditions
FROM     aircrafts a
         JOIN seats s ON a.aircraft_code = s.aircraft_code
WHERE    a.iata_code = $1
ORDER BY s.seat_no
`

type GetAircraftSeatsWithIataCodeRow struct {
	IataCode       string `json:"iata_code"`
	IcaoCode       string `json:"icao_code"`
	Model          string `json:"model"`
	Range          int32  `json:"range"`
	SeatNo         string `json:"seat_no"`
	FareConditions string `json:"fare_conditions"`
}

// iata_code,
// icao_code,
// model,
// range,
// company_id
func (q *Queries) GetAircraftSeatsWithIataCode(ctx context.Context, iataCode string) ([]GetAircraftSeatsWithIataCodeRow, error) {
	rows, err := q.db.QueryContext(ctx, getAircraftSeatsWithIataCode, iataCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []GetAircraftSeatsWithIataCodeRow{}
	for rows.Next() {
		var i GetAircraftSeatsWithIataCodeRow
		if err := rows.Scan(
			&i.IataCode,
			&i.IcaoCode,
			&i.Model,
			&i.Range,
			&i.SeatNo,
			&i.FareConditions,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getSeats = `-- name: GetSeats :many
SELECT aircraft_id, seat_no, fare_conditions FROM seats
`

func (q *Queries) GetSeats(ctx context.Context) ([]Seat, error) {
	rows, err := q.db.QueryContext(ctx, getSeats)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Seat{}
	for rows.Next() {
		var i Seat
		if err := rows.Scan(&i.AircraftID, &i.SeatNo, &i.FareConditions); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
