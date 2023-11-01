// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.14.0
// source: airport.sql

package db

import (
	"context"
	"database/sql"
)

const createAirport = `-- name: CreateAirport :one
INSERT INTO airports(
iata_code, 
icao_code, 
name, 
elevation,
city,
country,
state,
lat,
lon,
timezone
) VALUES
(  $1 , $2 , $3 , $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, iata_code, icao_code, name, country, state, city, elevation, lat, lon, timezone
`

type CreateAirportParams struct {
	IataCode  string         `json:"iata_code"`
	IcaoCode  string         `json:"icao_code"`
	Name      string         `json:"name"`
	Elevation sql.NullString `json:"elevation"`
	City      string         `json:"city"`
	Country   string         `json:"country"`
	State     string         `json:"state"`
	Lat       string         `json:"lat"`
	Lon       string         `json:"lon"`
	Timezone  string         `json:"timezone"`
}

// subdivision_code
func (q *Queries) CreateAirport(ctx context.Context, arg CreateAirportParams) (Airport, error) {
	row := q.db.QueryRowContext(ctx, createAirport,
		arg.IataCode,
		arg.IcaoCode,
		arg.Name,
		arg.Elevation,
		arg.City,
		arg.Country,
		arg.State,
		arg.Lat,
		arg.Lon,
		arg.Timezone,
	)
	var i Airport
	err := row.Scan(
		&i.ID,
		&i.IataCode,
		&i.IcaoCode,
		&i.Name,
		&i.Country,
		&i.State,
		&i.City,
		&i.Elevation,
		&i.Lat,
		&i.Lon,
		&i.Timezone,
	)
	return i, err
}

const deleteAirports = `-- name: DeleteAirports :one
DELETE FROM airports
WHERE iata_code = $1
RETURNING id, iata_code, icao_code, name, country, state, city, elevation, lat, lon, timezone
`

func (q *Queries) DeleteAirports(ctx context.Context, iataCode string) (Airport, error) {
	row := q.db.QueryRowContext(ctx, deleteAirports, iataCode)
	var i Airport
	err := row.Scan(
		&i.ID,
		&i.IataCode,
		&i.IcaoCode,
		&i.Name,
		&i.Country,
		&i.State,
		&i.City,
		&i.Elevation,
		&i.Lat,
		&i.Lon,
		&i.Timezone,
	)
	return i, err
}

const getAirport = `-- name: GetAirport :one
SELECT id, iata_code, icao_code, name, country, state, city, elevation, lat, lon, timezone FROM airports
WHERE iata_code = $1
`

func (q *Queries) GetAirport(ctx context.Context, iataCode string) (Airport, error) {
	row := q.db.QueryRowContext(ctx, getAirport, iataCode)
	var i Airport
	err := row.Scan(
		&i.ID,
		&i.IataCode,
		&i.IcaoCode,
		&i.Name,
		&i.Country,
		&i.State,
		&i.City,
		&i.Elevation,
		&i.Lat,
		&i.Lon,
		&i.Timezone,
	)
	return i, err
}

const listAirport = `-- name: ListAirport :many
SELECT 
iata_code, 
icao_code, 
name, 
elevation,
city,
country,
state,
lat,
lon,
timezone
FROM airports
OFFSET $1
LIMIT $2
`

type ListAirportParams struct {
	Offset int32 `json:"offset"`
	Limit  int32 `json:"limit"`
}

type ListAirportRow struct {
	IataCode  string         `json:"iata_code"`
	IcaoCode  string         `json:"icao_code"`
	Name      string         `json:"name"`
	Elevation sql.NullString `json:"elevation"`
	City      string         `json:"city"`
	Country   string         `json:"country"`
	State     string         `json:"state"`
	Lat       string         `json:"lat"`
	Lon       string         `json:"lon"`
	Timezone  string         `json:"timezone"`
}

func (q *Queries) ListAirport(ctx context.Context, arg ListAirportParams) ([]ListAirportRow, error) {
	rows, err := q.db.QueryContext(ctx, listAirport, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListAirportRow{}
	for rows.Next() {
		var i ListAirportRow
		if err := rows.Scan(
			&i.IataCode,
			&i.IcaoCode,
			&i.Name,
			&i.Elevation,
			&i.City,
			&i.Country,
			&i.State,
			&i.Lat,
			&i.Lon,
			&i.Timezone,
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
