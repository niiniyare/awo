// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.13.0
// source: airport.sql

package db

import (
	"context"
)

const createAirportList = `-- name: CreateAirportList :many
/*
INSERT INTO airports (
    iata_code
    name ,
    city ,
    coordinates
) VALUES (
    $1 , $2 , $3 , SRID=4326;POINT($4)
)
RETURNING * ;
*/
SELECT id, iata_code, icao_code, name, subdivision_code, city, coordinates FROM airports
WHERE iata_code = $1 LIMIT 1
`

func (q *Queries) CreateAirportList(ctx context.Context, iataCode string) ([]Airport, error) {
	rows, err := q.db.QueryContext(ctx, createAirportList, iataCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Airport{}
	for rows.Next() {
		var i Airport
		if err := rows.Scan(
			&i.ID,
			&i.IataCode,
			&i.IcaoCode,
			&i.Name,
			&i.SubdivisionCode,
			&i.City,
			&i.Coordinates,
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

const createAirports = `-- name: CreateAirports :one
INSERT INTO airports(
iata_code, 
icao_code, 
name, 
city, 
coordinates) VALUES
(  $1 , $2 , $3 , $4, POINT($5)
)
RETURNING id, iata_code, icao_code, name, subdivision_code, city, coordinates
`

type CreateAirportsParams struct {
	IataCode string      `json:"iata_code"`
	IcaoCode string      `json:"icao_code"`
	Name     string      `json:"name"`
	City     string      `json:"city"`
	Point    interface{} `json:"point"`
}

//subdivision_code,
func (q *Queries) CreateAirports(ctx context.Context, arg CreateAirportsParams) (Airport, error) {
	row := q.db.QueryRowContext(ctx, createAirports,
		arg.IataCode,
		arg.IcaoCode,
		arg.Name,
		arg.City,
		arg.Point,
	)
	var i Airport
	err := row.Scan(
		&i.ID,
		&i.IataCode,
		&i.IcaoCode,
		&i.Name,
		&i.SubdivisionCode,
		&i.City,
		&i.Coordinates,
	)
	return i, err
}

const deleteAirports = `-- name: DeleteAirports :exec
DELETE FROM airports
WHERE id = $1
RETURNING id, iata_code, icao_code, name, subdivision_code, city, coordinates
`

func (q *Queries) DeleteAirports(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, deleteAirports, id)
	return err
}

const listAirports = `-- name: ListAirports :many
SELECT id, iata_code,
name ,
city
FROM airports
ORDER BY id
`

type ListAirportsRow struct {
	ID       int64  `json:"id"`
	IataCode string `json:"iata_code"`
	Name     string `json:"name"`
	City     string `json:"city"`
}

func (q *Queries) ListAirports(ctx context.Context) ([]ListAirportsRow, error) {
	rows, err := q.db.QueryContext(ctx, listAirports)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListAirportsRow{}
	for rows.Next() {
		var i ListAirportsRow
		if err := rows.Scan(
			&i.ID,
			&i.IataCode,
			&i.Name,
			&i.City,
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
