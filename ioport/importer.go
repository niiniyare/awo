// Package ioport provides import and export capabilities for entity records.
//
// Import feeds records into the entity pipeline (validation + hooks run for
// each record). Export serializes query results to CSV, JSON, or JSONL.
//
// Import runs entirely through EntityBatchMutator — implemented by
// *api/service.EntityService.CreateBatch (Phase 2 Step 5,
// PHASE2_ARCHITECTURE_PLAN.md Step 5, ADR-025 §14) — never through a raw
// repository INSERT. Every imported row receives the same validation,
// defaults, before/after hooks, mandatory audit record, and durable outbox
// event as a record created via the ordinary POST /entities endpoint. ioport
// depends only on the narrow EntityBatchMutator interface below, not on
// api/service itself, so its own dependency graph stays at the
// driver/compiler layer — the concrete EntityService is wired in by whatever
// constructs the call (an HTTP handler or CLI command), not by ioport.
//
// Export serializes Query results and remains on the plain
// driver.EntityRepository read path — it performs no mutation and needs no
// pipeline access.
//
// # Import
//
//	result, err := ioport.Import(ctx, schema, mutator, reader, ioport.ImportOptions{
//	    Format: ioport.FormatCSV,
//	    Actor:  actor,
//	})
//
// # Export
//
//	err := ioport.Export(ctx, schema, repo, writer, ioport.ExportOptions{
//	    Format: ioport.FormatCSV,
//	    Filter: filter.Eq("status", "active"),
//	})
package ioport

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
)

// EntityBatchMutator is the canonical mutation-pipeline dependency Import
// uses to persist records in bounded, flush-sized batches (ADR-025 §14).
// Implemented by *api/service.EntityService.CreateBatch. rows are raw field
// data maps (the same shape driver.CreateInput.Data already carries);
// skipInvalid mirrors ImportOptions.SkipErrors — when true, a row that fails
// validation is excluded from the flush (recorded in the result's Skipped
// list) while the rest of the flush still proceeds; when false, the first
// invalid row aborts the call with no transaction opened.
type EntityBatchMutator interface {
	CreateBatch(ctx context.Context, rows []map[string]any, actor *def.Actor, skipInvalid bool) (*def.BatchCreateResult, error)
}

// Format identifies the serialization format for import and export.
type Format string

const (
	// FormatCSV is RFC 4180 CSV with a header row.
	FormatCSV Format = "csv"

	// FormatJSON is a JSON array of objects, one object per record.
	FormatJSON Format = "json"

	// FormatJSONL is newline-delimited JSON (one object per line, no array wrapper).
	FormatJSONL Format = "jsonl"
)

// ImportOptions configures an import operation.
type ImportOptions struct {
	// Format is the input serialization format. Default: FormatCSV.
	Format Format

	// Actor is the authenticated principal performing the import.
	// Passed to the repository's CreateInput.Actor for audit logging.
	Actor *def.Actor

	// BatchSize is the number of records per BulkCreate call. Default: 100.
	// Must be > 0.
	BatchSize int

	// SkipErrors when true logs row errors and continues. When false (default),
	// the first error aborts the import.
	SkipErrors bool
}

// ImportResult summarises the outcome of an import operation.
type ImportResult struct {
	// Total is the number of input rows processed (including skipped errors).
	Total int

	// Created is the number of records successfully created.
	Created int

	// Errors is the list of per-row errors encountered during import.
	// Non-empty only when ImportOptions.SkipErrors is true.
	Errors []ImportError
}

// ImportError records a single row-level error during import.
type ImportError struct {
	// Row is the 1-based input row number.
	Row int

	// Err is the error encountered for that row.
	Err error
}

func (e ImportError) Error() string {
	return fmt.Sprintf("row %d: %v", e.Row, e.Err)
}

// Import reads records from r in the specified format and creates them via
// mutator's canonical batch-create pipeline (EntityBatchMutator —
// implemented by *api/service.EntityService.CreateBatch). All framework
// validation, defaults, before/after hooks, mandatory audit, and durable
// outbox events run for each record, per flush (default 100 rows, ADR-025
// §14) — never bypassed. Returns an ImportResult and a non-nil error only
// when ImportOptions.SkipErrors is false and a row or flush fails; when true,
// row/flush failures are recorded in ImportResult.Errors and the import
// continues with the next row/flush.
func Import(
	ctx context.Context,
	schema *compiler.EntitySchema,
	mutator EntityBatchMutator,
	r io.Reader,
	opts ImportOptions,
) (*ImportResult, error) {
	if opts.Format == "" {
		opts.Format = FormatCSV
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}

	switch opts.Format {
	case FormatCSV:
		return importCSV(ctx, schema, mutator, r, opts)
	case FormatJSON:
		return importJSON(ctx, schema, mutator, r, opts)
	case FormatJSONL:
		return importJSONL(ctx, schema, mutator, r, opts)
	default:
		return nil, fmt.Errorf("ioport: unknown import format %q", opts.Format)
	}
}

// flushBatch persists batchData (whose entries correspond 1:1, in order, to
// the 1-based input row numbers in batchRowNums) via mutator.CreateBatch,
// merging the outcome into result in place.
//
// Returns a non-nil error only when the caller must abort the entire import
// immediately (ImportOptions.SkipErrors is false and either a row failed
// validation or the flush's shared transaction failed) — mirroring the
// existing fail-fast convention already used for CSV/JSON parse errors.
// When SkipErrors is true, every failure this function encounters is instead
// appended to result.Errors and nil is returned, so the caller proceeds to
// the next row/flush.
func flushBatch(ctx context.Context, mutator EntityBatchMutator, opts ImportOptions, batchData []map[string]any, batchRowNums []int, result *ImportResult) error {
	if len(batchData) == 0 {
		return nil
	}

	res, err := mutator.CreateBatch(ctx, batchData, opts.Actor, opts.SkipErrors)
	if res != nil {
		result.Created += len(res.Created)
		for _, sk := range res.Skipped {
			row := 0
			if sk.Index >= 0 && sk.Index < len(batchRowNums) {
				row = batchRowNums[sk.Index]
			}
			ie := ImportError{Row: row, Err: sk.Err}
			if !opts.SkipErrors {
				return ie
			}
			result.Errors = append(result.Errors, ie)
		}
	}
	if err != nil {
		lastRow := 0
		if len(batchRowNums) > 0 {
			lastRow = batchRowNums[len(batchRowNums)-1]
		}
		ie := ImportError{Row: lastRow, Err: err}
		if !opts.SkipErrors {
			return ie
		}
		result.Errors = append(result.Errors, ie)
	}
	return nil
}

func importCSV(
	ctx context.Context,
	schema *compiler.EntitySchema,
	mutator EntityBatchMutator,
	r io.Reader,
	opts ImportOptions,
) (*ImportResult, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	headers, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("ioport: csv: read header: %w", err)
	}

	result := &ImportResult{}
	var batchData []map[string]any
	var batchRowNums []int
	rowNum := 1 // header is row 0

	flush := func() error {
		err := flushBatch(ctx, mutator, opts, batchData, batchRowNums, result)
		batchData = batchData[:0]
		batchRowNums = batchRowNums[:0]
		return err
	}

	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			ie := ImportError{Row: rowNum, Err: err}
			if !opts.SkipErrors {
				return result, ie
			}
			result.Errors = append(result.Errors, ie)
			continue
		}
		result.Total++

		data, parseErr := csvRowToData(schema, headers, record)
		if parseErr != nil {
			ie := ImportError{Row: rowNum, Err: parseErr}
			if !opts.SkipErrors {
				return result, ie
			}
			result.Errors = append(result.Errors, ie)
			continue
		}

		batchData = append(batchData, data)
		batchRowNums = append(batchRowNums, rowNum)
		if len(batchData) >= opts.BatchSize {
			if flushErr := flush(); flushErr != nil {
				return result, flushErr
			}
		}
	}

	if err := flush(); err != nil {
		return result, err
	}
	return result, nil
}

func importJSON(
	ctx context.Context,
	schema *compiler.EntitySchema,
	mutator EntityBatchMutator,
	r io.Reader,
	opts ImportOptions,
) (*ImportResult, error) {
	var rows []map[string]any
	if err := json.NewDecoder(r).Decode(&rows); err != nil {
		return nil, fmt.Errorf("ioport: json: decode: %w", err)
	}

	result := &ImportResult{Total: len(rows)}
	var batchData []map[string]any
	var batchRowNums []int

	for i, row := range rows {
		rowNum := i + 1
		data := normalizeJSONRow(schema, row)
		batchData = append(batchData, data)
		batchRowNums = append(batchRowNums, rowNum)

		if len(batchData) >= opts.BatchSize || i == len(rows)-1 {
			if err := flushBatch(ctx, mutator, opts, batchData, batchRowNums, result); err != nil {
				return result, err
			}
			batchData = batchData[:0]
			batchRowNums = batchRowNums[:0]
		}
	}
	return result, nil
}

func importJSONL(
	ctx context.Context,
	schema *compiler.EntitySchema,
	mutator EntityBatchMutator,
	r io.Reader,
	opts ImportOptions,
) (*ImportResult, error) {
	dec := json.NewDecoder(r)
	result := &ImportResult{}
	var batchData []map[string]any
	var batchRowNums []int
	rowNum := 0

	flush := func() error {
		err := flushBatch(ctx, mutator, opts, batchData, batchRowNums, result)
		batchData = batchData[:0]
		batchRowNums = batchRowNums[:0]
		return err
	}

	for dec.More() {
		rowNum++
		result.Total++
		var row map[string]any
		if err := dec.Decode(&row); err != nil {
			ie := ImportError{Row: rowNum, Err: err}
			if !opts.SkipErrors {
				return result, ie
			}
			result.Errors = append(result.Errors, ie)
			continue
		}
		data := normalizeJSONRow(schema, row)
		batchData = append(batchData, data)
		batchRowNums = append(batchRowNums, rowNum)

		if len(batchData) >= opts.BatchSize {
			if flushErr := flush(); flushErr != nil {
				return result, flushErr
			}
		}
	}
	if err := flush(); err != nil {
		return result, err
	}
	return result, nil
}

// csvRowToData converts a CSV row into a field data map using the entity schema
// to guide type coercion. Unknown column headers are silently ignored.
func csvRowToData(schema *compiler.EntitySchema, headers, values []string) (map[string]any, error) {
	data := make(map[string]any, len(headers))
	for i, h := range headers {
		if i >= len(values) {
			break
		}
		h = strings.TrimSpace(h)
		v := strings.TrimSpace(values[i])
		if v == "" {
			continue
		}
		fd, ok := schema.FieldsByName[h]
		if !ok {
			continue // unknown column — skip
		}
		coerced, err := coerceCSVValue(fd.Type, v)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", h, err)
		}
		data[h] = coerced
	}
	return data, nil
}

// coerceCSVValue converts a CSV string to the Go type expected for a FieldType.
func coerceCSVValue(ft def.FieldType, s string) (any, error) {
	switch ft {
	case def.FieldTypeInt:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected integer, got %q", s)
		}
		return n, nil
	case def.FieldTypeFloat:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("expected float, got %q", s)
		}
		return f, nil
	case def.FieldTypeBool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("expected bool, got %q", s)
		}
		return b, nil
	default:
		// All other types (data, select, datetime, etc.) remain as strings.
		// The runtime/repo layer applies further type-specific parsing.
		return s, nil
	}
}

// normalizeJSONRow strips unknown keys and coerces JSON-decoded numeric values
// (float64) to the type expected by the entity schema.
func normalizeJSONRow(schema *compiler.EntitySchema, row map[string]any) map[string]any {
	data := make(map[string]any, len(row))
	for k, v := range row {
		if fd, ok := schema.FieldsByName[k]; ok {
			data[k] = coerceJSONValue(fd.Type, v)
		}
	}
	return data
}

// coerceJSONValue converts JSON-decoded values (where numbers are float64)
// to the Go type expected for a FieldType.
func coerceJSONValue(ft def.FieldType, v any) any {
	switch ft {
	case def.FieldTypeInt:
		if f, ok := v.(float64); ok {
			return int64(f)
		}
	}
	return v
}

// ExportOptions configures an export operation.
type ExportOptions struct {
	// Format is the output serialization format. Default: FormatCSV.
	Format Format

	// Filter restricts which records are exported. Nil exports all records.
	Filter *filter.Filter

	// Fields restricts which fields appear in the output. Nil exports all
	// non-sensitive fields.
	Fields []string

	// PageSize is the number of records fetched per page. Default: 500.
	PageSize int
}

// Export writes all matching records from repo to w in the specified format.
// Sensitive fields (FieldDef.Sensitive) are excluded unless explicitly listed
// in ExportOptions.Fields.
func Export(
	ctx context.Context,
	schema *compiler.EntitySchema,
	repo driver.EntityRepository[*def.EntityRecord],
	w io.Writer,
	opts ExportOptions,
) error {
	if opts.Format == "" {
		opts.Format = FormatCSV
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 500
	}

	// Determine column list (non-sensitive by default).
	columns := opts.Fields
	if len(columns) == 0 {
		for _, f := range schema.Fields {
			if !f.Sensitive {
				columns = append(columns, f.Name)
			}
		}
	}

	switch opts.Format {
	case FormatCSV:
		return exportCSV(ctx, schema, repo, w, opts, columns)
	case FormatJSON:
		return exportJSON(ctx, schema, repo, w, opts, columns)
	case FormatJSONL:
		return exportJSONL(ctx, schema, repo, w, opts, columns)
	default:
		return fmt.Errorf("ioport: unknown export format %q", opts.Format)
	}
}

func exportCSV(
	ctx context.Context,
	_ *compiler.EntitySchema,
	repo driver.EntityRepository[*def.EntityRecord],
	w io.Writer,
	opts ExportOptions,
	columns []string,
) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(columns); err != nil {
		return fmt.Errorf("ioport: csv: write header: %w", err)
	}

	page := 1
	for {
		records, _, err := repo.Query(ctx, opts.Filter,
			driver.WithPage(page, opts.PageSize))
		if err != nil {
			return fmt.Errorf("ioport: csv: query page %d: %w", page, err)
		}
		for _, rec := range records {
			row := make([]string, len(columns))
			for i, col := range columns {
				row[i] = fmt.Sprint(rec.Get(col))
			}
			if err := cw.Write(row); err != nil {
				return fmt.Errorf("ioport: csv: write row: %w", err)
			}
		}
		if len(records) < opts.PageSize {
			break
		}
		page++
	}
	cw.Flush()
	return cw.Error()
}

func exportJSON(
	ctx context.Context,
	_ *compiler.EntitySchema,
	repo driver.EntityRepository[*def.EntityRecord],
	w io.Writer,
	opts ExportOptions,
	columns []string,
) error {
	enc := json.NewEncoder(w)
	var all []map[string]any

	page := 1
	for {
		records, _, err := repo.Query(ctx, opts.Filter,
			driver.WithPage(page, opts.PageSize))
		if err != nil {
			return fmt.Errorf("ioport: json: query page %d: %w", page, err)
		}
		for _, rec := range records {
			row := make(map[string]any, len(columns))
			for _, col := range columns {
				row[col] = rec.Get(col)
			}
			all = append(all, row)
		}
		if len(records) < opts.PageSize {
			break
		}
		page++
	}
	return enc.Encode(all)
}

func exportJSONL(
	ctx context.Context,
	_ *compiler.EntitySchema,
	repo driver.EntityRepository[*def.EntityRecord],
	w io.Writer,
	opts ExportOptions,
	columns []string,
) error {
	enc := json.NewEncoder(w)
	page := 1
	for {
		records, _, err := repo.Query(ctx, opts.Filter,
			driver.WithPage(page, opts.PageSize))
		if err != nil {
			return fmt.Errorf("ioport: jsonl: query page %d: %w", page, err)
		}
		for _, rec := range records {
			row := make(map[string]any, len(columns))
			for _, col := range columns {
				row[col] = rec.Get(col)
			}
			if err := enc.Encode(row); err != nil {
				return fmt.Errorf("ioport: jsonl: encode row: %w", err)
			}
		}
		if len(records) < opts.PageSize {
			break
		}
		page++
	}
	return nil
}
