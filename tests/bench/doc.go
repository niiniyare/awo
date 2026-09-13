// Package bench contains benchmark tests for the Awo framework core operations.
// Benchmarks run against the fakestore (no DB required) to measure framework
// overhead: registry build, compilation, filter evaluation, CRUD operations.
//
// Run with:
//
//	go test -bench=. -benchmem ./awo/tests/bench/
package bench
