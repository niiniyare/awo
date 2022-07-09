// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.13.0
// source: airport.sql

package db

import (
	"context"
)

const createAirportList = `-- name: CreateAirportList :many
INSERT INTO airports (
  airport_code , 
  airport_name ,
  city , 
  coordinates
) VALUES (
  $1 , $2 , $3 , POINT($4)
)
RETURNING airport_code, airport_name, country_code, city, coordinates, timezone, created_at
`

type CreateAirportListParams struct {
	AirportCode string      `json:"airport_code"`
	AirportName string      `json:"airport_name"`
	City        string      `json:"city"`
	Point       interface{} `json:"point"`
}

func (q *Queries) CreateAirportList(ctx context.Context, arg CreateAirportListParams) ([]Airport, error) {
	rows, err := q.db.QueryContext(ctx, createAirportList,
		arg.AirportCode,
		arg.AirportName,
		arg.City,
		arg.Point,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Airport{}
	for rows.Next() {
		var i Airport
		if err := rows.Scan(
			&i.AirportCode,
			&i.AirportName,
			&i.CountryCode,
			&i.City,
			&i.Coordinates,
			&i.Timezone,
			&i.CreatedAt,
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
INSERT INTO airports (
  airport_code , 
  airport_name ,
  city , 
  coordinates
) VALUES (
  $1 , $2 , $3 , $4
)
RETURNING airport_code, airport_name, country_code, city, coordinates, timezone, created_at
`

type CreateAirportsParams struct {
	AirportCode string      `json:"airport_code"`
	AirportName string      `json:"airport_name"`
	City        string      `json:"city"`
	Coordinates interface{} `json:"coordinates"`
}

func (q *Queries) CreateAirports(ctx context.Context, arg CreateAirportsParams) (Airport, error) {
	row := q.db.QueryRowContext(ctx, createAirports,
		arg.AirportCode,
		arg.AirportName,
		arg.City,
		arg.Coordinates,
	)
	var i Airport
	err := row.Scan(
		&i.AirportCode,
		&i.AirportName,
		&i.CountryCode,
		&i.City,
		&i.Coordinates,
		&i.Timezone,
		&i.CreatedAt,
	)
	return i, err
}

const deleteAirports = `-- name: DeleteAirports :exec
DELETE FROM airports
WHERE airport_code  = $1
RETURNING airport_code, airport_name, country_code, city, coordinates, timezone, created_at
`

func (q *Queries) DeleteAirports(ctx context.Context, airportCode string) error {
	_, err := q.db.ExecContext(ctx, deleteAirports, airportCode)
	return err
}

const getAirports = `-- name: GetAirports :one
SELECT airport_code, airport_name, country_code, city, coordinates, timezone, created_at FROM airports
WHERE airport_code  = $1 LIMIT 1
`

func (q *Queries) GetAirports(ctx context.Context, airportCode string) (Airport, error) {
	row := q.db.QueryRowContext(ctx, getAirports, airportCode)
	var i Airport
	err := row.Scan(
		&i.AirportCode,
		&i.AirportName,
		&i.CountryCode,
		&i.City,
		&i.Coordinates,
		&i.Timezone,
		&i.CreatedAt,
	)
	return i, err
}

const listAirports = `-- name: ListAirports :many
SELECT airport_code,
  airport_name ,
  city
FROM airports
ORDER BY airport_code
`

type ListAirportsRow struct {
	AirportCode string `json:"airport_code"`
	AirportName string `json:"airport_name"`
	City        string `json:"city"`
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
		if err := rows.Scan(&i.AirportCode, &i.AirportName, &i.City); err != nil {
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
