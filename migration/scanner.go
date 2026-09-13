package migration

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// filenameRe matches migration filenames:
//
//	{14-digit-timestamp}_{description}.{up|down}.sql
var filenameRe = regexp.MustCompile(`^(\d{14})_([a-z0-9_]+)\.(up|down)\.sql$`)

// Pair is a matched up+down migration pair discovered during a directory scan.
type Pair struct {
	Version     uint64
	Description string
	UpFile      string
	DownFile    string
}

// Scan reads dir for *.up.sql / *.down.sql pairs and returns the corresponding
// Steps sorted by Version ascending. Both up and down Steps are returned;
// use BuildPlan to select the relevant subset.
//
// Files that do not match the naming convention are silently ignored.
// A missing down file is not an error — its Step will have empty SQL.
func Scan(dir string) ([]Step, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("migration.Scan: read dir %s: %w", dir, err)
	}

	type rawFile struct {
		version     uint64
		description string
		direction   Direction
		path        string
	}

	var raws []rawFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := filenameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			continue
		}
		stepDir := DirectionUp
		if m[3] == "down" {
			stepDir = DirectionDown
		}
		raws = append(raws, rawFile{
			version:     v,
			description: m[2],
			direction:   stepDir,
			path:        filepath.Join(dir, e.Name()),
		})
	}

	// Build steps from raw files.
	steps := make([]Step, 0, len(raws))
	for _, r := range raws {
		content, err := os.ReadFile(r.path)
		if err != nil {
			return nil, fmt.Errorf("migration.Scan: read %s: %w", r.path, err)
		}
		sql := string(content)
		sum := sha256.Sum256(content)
		steps = append(steps, Step{
			Version:     r.version,
			Description: r.description,
			Direction:   r.direction,
			SQL:         sql,
			Checksum:    fmt.Sprintf("%x", sum),
		})
	}

	sort.Slice(steps, func(i, j int) bool {
		if steps[i].Version != steps[j].Version {
			return steps[i].Version < steps[j].Version
		}
		// Within same version, up before down.
		return steps[i].Direction == DirectionUp && steps[j].Direction == DirectionDown
	})

	return steps, nil
}
