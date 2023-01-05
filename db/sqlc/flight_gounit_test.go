package db

import (
	"context"
	"reflect"
	"testing"
)

func TestQueries_CreateFlight(t *testing.T) {
	type args struct {
		ctx context.Context
		arg CreateFlightParams
	}
	tests := []struct {
		name    string
		init    func(t *testing.T) *Queries
		inspect func(r *Queries, t *testing.T) //inspects receiver after test run

		args func(t *testing.T) args

		want1      Flight
		wantErr    bool
		inspectErr func(err error, t *testing.T) //use for more precise error evaluation after test
	}{
		//TODO: Add test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			receiver := tt.init(t)
			got1, err := receiver.CreateFlight(tArgs.ctx, tArgs.arg)

			if tt.inspect != nil {
				tt.inspect(receiver, t)
			}

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Queries.CreateFlight got1 = %v, want1: %v", got1, tt.want1)
			}

			if (err != nil) != tt.wantErr {
				t.Fatalf("Queries.CreateFlight error = %v, wantErr: %t", err, tt.wantErr)
			}

			if tt.inspectErr != nil {
				tt.inspectErr(err, t)
			}
		})
	}
}

func TestQueries_FlightAvailability(t *testing.T) {
	type args struct {
		ctx context.Context
		arg FlightAvailabilityParams
	}
	tests := []struct {
		name    string
		init    func(t *testing.T) *Queries
		inspect func(r *Queries, t *testing.T) //inspects receiver after test run

		args func(t *testing.T) args

		want1      []FlightsV
		wantErr    bool
		inspectErr func(err error, t *testing.T) //use for more precise error evaluation after test
	}{
		//TODO: Add test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			receiver := tt.init(t)
			got1, err := receiver.FlightAvailability(tArgs.ctx, tArgs.arg)

			if tt.inspect != nil {
				tt.inspect(receiver, t)
			}

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Queries.FlightAvailability got1 = %v, want1: %v", got1, tt.want1)
			}

			if (err != nil) != tt.wantErr {
				t.Fatalf("Queries.FlightAvailability error = %v, wantErr: %t", err, tt.wantErr)
			}

			if tt.inspectErr != nil {
				tt.inspectErr(err, t)
			}
		})
	}
}
