// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.14.0
// source: aircraft.sql

package db

import (
	"context"
	"reflect"
	"testing"
)

func TestQueries_CreateAircraft(t *testing.T) {
	type fields struct {
		db DBTX
	}
	type args struct {
		ctx context.Context
		arg CreateAircraftParams
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    Aircraft
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &Queries{
				db: tt.fields.db,
			}
			got, err := q.CreateAircraft(tt.args.ctx, tt.args.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Queries.CreateAircraft() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Queries.CreateAircraft() = %v, want %v", got, tt.want)
			}
		})
	}
}