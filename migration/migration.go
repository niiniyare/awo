// Package migration provides the framework-native migration system for AWO.
//
// Each framework module embeds its SQL migration files and registers a [Source]
// via [Register] inside an init() function. The [BuildFS] function assembles
// all registered sources into a single, topologically-ordered virtual fs.FS
// that is compatible with the golang-migrate iofs source driver.
//
// # Module Registration Pattern
//
//	//go:embed *.sql
//	var sqlFS embed.FS
//
//	func init() {
//	    migration.Register(migration.Source{
//	        Module:    "iam",
//	        Priority:  20,
//	        DependsOn: []string{"bootstrap", "tenant"},
//	        FS:        sqlFS,
//	    })
//	}
//
// # Version Numbering
//
// Global migration versions are computed as: Priority*1000 + LocalStep.
// LocalStep is the 3-digit numeric prefix of each SQL filename
// (e.g. "001_create_iam_user.up.sql" → LocalStep=1).
//
// This scheme reserves 999 steps per module and supports up to 999 modules
// before numeric collision. Priority gaps allow future modules to be inserted
// in order without renumbering.
//
// # Dependency Ordering
//
// DependsOn declares which other Module names must be fully applied before
// this source begins. The engine topologically sorts all registered sources
// and panics on cycles or missing dependencies at startup.
package migration

import (
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Source describes all SQL migrations contributed by one framework module.
// It is registered once via [Register] inside the module's init() function.
type Source struct {
	// Module is the unique module identifier used in DependsOn declarations.
	// Use short lowercase names: "bootstrap", "tenant", "iam", "finance".
	Module string

	// Priority controls the relative order of modules when no explicit
	// DependsOn relationship exists. Lower priority runs first.
	// Standard tiers:
	//   0   — bootstrap (extensions, utilities)
	//   10  — tenant    (platform_tenant table, RLS infrastructure)
	//   20  — iam       (users, sessions, roles)
	//   30  — finance   (accounting, invoicing)
	//   40+ — custom application modules
	Priority int

	// DependsOn lists Module names that must be fully migrated before this
	// source begins. The engine enforces this at startup via topological sort.
	DependsOn []string

	// FS is the embedded filesystem containing the SQL files for this module.
	// Files must follow the naming convention:
	//   {NNN}_{description}.up.sql    (apply)
	//   {NNN}_{description}.down.sql  (rollback, optional)
	// where NNN is a 3-digit zero-padded integer (001–999).
	FS fs.FS
}

var globalRegistry []Source

// Register adds a module's migrations to the global registry.
// Must be called only from init() functions; never after bootstrap starts.
func Register(s Source) {
	if s.Module == "" {
		panic("migration.Register: Module is required")
	}
	if s.FS == nil {
		panic(fmt.Sprintf("migration.Register: FS is nil for module %q", s.Module))
	}
	globalRegistry = append(globalRegistry, s)
}

// All returns all registered sources in topologically-sorted order.
// Panics if a dependency cycle or missing dependency is detected.
func All() []Source {
	return TopoSortSources(globalRegistry)
}

// BuildFS constructs a single fs.FS containing all migration files from all
// registered sources, with global version numbers assigned per the scheme
// described in the package doc. The returned FS is compatible with the
// golang-migrate iofs source driver.
//
// Panics if any two sources produce a colliding global version number.
func BuildFS() fs.FS {
	return BuildFSFrom(globalRegistry)
}

// BuildFSFrom constructs a virtual fs.FS from an explicit list of sources,
// bypassing the global registry. Intended for use in tests and tooling.
func BuildFSFrom(sources []Source) fs.FS {
	sorted := TopoSortSources(sources)
	mem := make(memFS)
	// Track used global versions across all modules for collision detection.
	// Two modules with the same Priority and the same local step number produce
	// the same global version — that is a configuration error.
	usedVersions := make(map[int]string) // globalVer → "module:stepname"

	for _, src := range sorted {
		steps, err := readSteps(src)
		if err != nil {
			panic(fmt.Sprintf("migration: module %q: read steps: %v", src.Module, err))
		}
		for _, step := range steps {
			globalVer := src.Priority*1000 + step.localNum
			if prev, exists := usedVersions[globalVer]; exists {
				panic(fmt.Sprintf("migration: version collision at global version %d: %s vs %s:%s",
					globalVer, prev, src.Module, step.name))
			}
			usedVersions[globalVer] = src.Module + ":" + step.name
			upName := fmt.Sprintf("%06d_%s_%s.up.sql", globalVer, src.Module, step.name)
			mem[upName] = []byte(step.up)
			if step.down != "" {
				downName := fmt.Sprintf("%06d_%s_%s.down.sql", globalVer, src.Module, step.name)
				mem[downName] = []byte(step.down)
			}
		}
	}
	return mem
}

// TopoSortSources sorts sources by dependency order. Exported for testing.
func TopoSortSources(sources []Source) []Source {
	return topoSort(sources)
}

// ── In-memory fs.FS ──────────────────────────────────────────────────────────

// memFS is a simple in-memory fs.FS. Keys are file names (no directory prefix).
type memFS map[string][]byte

func (m memFS) Open(name string) (fs.File, error) {
	if name == "." {
		return &memDir{files: m}, nil
	}
	data, ok := m[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &memFile{name: name, data: data, r: strings.NewReader(string(data))}, nil
}

// memFile implements fs.File for a single in-memory file.
type memFile struct {
	name string
	data []byte
	r    *strings.Reader
}

func (f *memFile) Stat() (fs.FileInfo, error) {
	return &memFileInfo{name: f.name, size: int64(len(f.data))}, nil
}
func (f *memFile) Read(b []byte) (int, error) { return f.r.Read(b) }
func (f *memFile) Close() error               { return nil }

// memDir implements fs.File for the root directory listing.
type memDir struct {
	files   memFS
	entries []fs.DirEntry
	pos     int
}

func (d *memDir) Stat() (fs.FileInfo, error) {
	return &memFileInfo{name: ".", isDir: true}, nil
}
func (d *memDir) Read([]byte) (int, error) { return 0, io.EOF }
func (d *memDir) Close() error             { return nil }
func (d *memDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.entries == nil {
		for name, data := range d.files {
			d.entries = append(d.entries, &memDirEntry{name: name, size: int64(len(data))})
		}
		sort.Slice(d.entries, func(i, j int) bool {
			return d.entries[i].Name() < d.entries[j].Name()
		})
	}
	if n <= 0 {
		// Return all remaining entries and advance to end.
		result := d.entries[d.pos:]
		d.pos = len(d.entries)
		return result, nil
	}
	if d.pos >= len(d.entries) {
		return nil, io.EOF
	}
	end := d.pos + n
	if end > len(d.entries) {
		end = len(d.entries)
	}
	result := d.entries[d.pos:end]
	d.pos = end
	return result, nil
}

type memFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi *memFileInfo) Name() string { return fi.name }
func (fi *memFileInfo) Size() int64  { return fi.size }
func (fi *memFileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (fi *memFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *memFileInfo) IsDir() bool        { return fi.isDir }
func (fi *memFileInfo) Sys() any           { return nil }

type memDirEntry struct {
	name string
	size int64
}

func (e *memDirEntry) Name() string      { return e.name }
func (e *memDirEntry) IsDir() bool       { return false }
func (e *memDirEntry) Type() fs.FileMode { return 0 }
func (e *memDirEntry) Info() (fs.FileInfo, error) {
	return &memFileInfo{name: e.name, size: e.size}, nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// step is an internal representation of one migration file pair.
type step struct {
	localNum int    // 3-digit number from filename prefix
	name     string // description part of filename
	up       string
	down     string
}

// readSteps reads and parses SQL files from src.FS.
// Files not matching the naming convention are silently skipped.
func readSteps(src Source) ([]step, error) {
	entries, err := fs.ReadDir(src.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("readdir: %w", err)
	}

	byNum := make(map[int]*step)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		num, desc, direction, ok := parseSQLFilename(name)
		if !ok {
			continue
		}
		data, err := fs.ReadFile(src.FS, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		s := byNum[num]
		if s == nil {
			s = &step{localNum: num, name: desc}
			byNum[num] = s
		}
		switch direction {
		case "up":
			s.up = string(data)
		case "down":
			s.down = string(data)
		}
	}

	steps := make([]step, 0, len(byNum))
	for _, s := range byNum {
		if s.up == "" {
			return nil, fmt.Errorf("step %03d (%s) has no .up.sql file", s.localNum, s.name)
		}
		steps = append(steps, *s)
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].localNum < steps[j].localNum })
	return steps, nil
}

// parseSQLFilename parses "NNN_description.up.sql" → (NNN, description, "up", true).
func parseSQLFilename(name string) (num int, desc, direction string, ok bool) {
	if !strings.HasSuffix(name, ".sql") {
		return
	}
	name = strings.TrimSuffix(name, ".sql")

	var dir string
	switch {
	case strings.HasSuffix(name, ".up"):
		dir = "up"
		name = strings.TrimSuffix(name, ".up")
	case strings.HasSuffix(name, ".down"):
		dir = "down"
		name = strings.TrimSuffix(name, ".down")
	default:
		return
	}

	idx := strings.Index(name, "_")
	if idx < 1 {
		return
	}
	n, err := strconv.Atoi(name[:idx])
	if err != nil || n < 1 || n > 999 {
		return
	}
	return n, name[idx+1:], dir, true
}

// topoSort returns sources sorted by dependency order.
// Panics on cycle or missing dependency.
func topoSort(sources []Source) []Source {
	byModule := make(map[string]Source, len(sources))
	for _, s := range sources {
		byModule[s.Module] = s
	}

	for _, s := range sources {
		for _, dep := range s.DependsOn {
			if _, ok := byModule[dep]; !ok {
				panic(fmt.Sprintf("migration: module %q depends on %q which is not registered", s.Module, dep))
			}
		}
	}

	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var result []Source

	var visit func(module string)
	visit = func(module string) {
		if visited[module] {
			return
		}
		if inStack[module] {
			panic(fmt.Sprintf("migration: dependency cycle detected involving module %q", module))
		}
		inStack[module] = true
		s := byModule[module]
		deps := append([]string(nil), s.DependsOn...)
		sort.Strings(deps)
		for _, dep := range deps {
			visit(dep)
		}
		inStack[module] = false
		visited[module] = true
		result = append(result, s)
	}

	sorted := append([]Source(nil), sources...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority < sorted[j].Priority
		}
		return sorted[i].Module < sorted[j].Module
	})

	for _, s := range sorted {
		visit(s.Module)
	}
	return result
}
