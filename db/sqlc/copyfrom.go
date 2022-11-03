// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.14.0
// source: copyfrom.go

package db

import (
	"context"
)

// iteratorForInsertNewSeatMap implements pgx.CopyFromSource.
type iteratorForInsertNewSeatMap struct {
	rows                 []InsertNewSeatMapParams
	skippedFirstNextCall bool
}

func (r *iteratorForInsertNewSeatMap) Next() bool {
	if len(r.rows) == 0 {
		return false
	}
	if !r.skippedFirstNextCall {
		r.skippedFirstNextCall = true
		return true
	}
	r.rows = r.rows[1:]
	return len(r.rows) > 0
}

func (r iteratorForInsertNewSeatMap) Values() ([]interface{}, error) {
	return []interface{}{
		r.rows[0].AircraftID,
		r.rows[0].CabinClass,
		r.rows[0].StartRow,
		r.rows[0].EndRow,
		r.rows[0].ColumnLayout,
	}, nil
}

func (r iteratorForInsertNewSeatMap) Err() error {
	return nil
}

func (q *Queries) InsertNewSeatMap(ctx context.Context, arg []InsertNewSeatMapParams) (int64, error) {
	return q.db.CopyFrom(ctx, []string{"seat_map"}, []string{"aircraft_id", "cabin_class", "start_row", "end_row", "column_layout"}, &iteratorForInsertNewSeatMap{rows: arg})
}
