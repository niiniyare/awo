package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueries_CreateAirline(t *testing.T) {
	type fields struct {
		db DBTX
	}
	type args struct {
		ctx context.Context
		arg CreateAirlineParams
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      Airline
		assertion assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		q := &Queries{
			db: tt.fields.db,
		}
		got, err := q.CreateAirline(tt.args.ctx, tt.args.arg)
		tt.assertion(t, err, fmt.Sprintf("%q. Queries.CreateAirline()", tt.name))
		assert.Equalf(t, tt.want, got, "%q. Queries.CreateAirline()", tt.name)
	}
}

func TestQueries_DeleteAirline(t *testing.T) {
	type fields struct {
		db DBTX
	}
	type args struct {
		ctx context.Context
		id  int64
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		assertion assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		q := &Queries{
			db: tt.fields.db,
		}
		tt.assertion(t, q.DeleteAirline(tt.args.ctx, tt.args.id), fmt.Sprintf("%q. Queries.DeleteAirline()", tt.name))
	}
}

func TestQueries_GetAirline(t *testing.T) {
	type fields struct {
		db DBTX
	}
	type args struct {
		ctx context.Context
		id  int64
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      Airline
		assertion assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		q := &Queries{
			db: tt.fields.db,
		}
		got, err := q.GetAirline(tt.args.ctx, tt.args.id)
		tt.assertion(t, err, fmt.Sprintf("%q. Queries.GetAirline()", tt.name))
		assert.Equalf(t, tt.want, got, "%q. Queries.GetAirline()", tt.name)
	}
}

func TestQueries_ListAirline(t *testing.T) {
	type fields struct {
		db DBTX
	}
	type args struct {
		ctx context.Context
		arg ListAirlineParams
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      []Airline
		assertion assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		q := &Queries{
			db: tt.fields.db,
		}
		got, err := q.ListAirline(tt.args.ctx, tt.args.arg)
		tt.assertion(t, err, fmt.Sprintf("%q. Queries.ListAirline()", tt.name))
		assert.Equalf(t, tt.want, got, "%q. Queries.ListAirline()", tt.name)
	}
}
