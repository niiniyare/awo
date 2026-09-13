package migration

import (
	"fmt"
	"sort"
)

// BuildPlan constructs an execution Plan from the provided steps.
//
//   - currentVersion: the version currently applied in the database (0 = fresh).
//   - direction: DirectionUp or DirectionDown.
//   - target: the desired target version. 0 means "latest" for DirectionUp,
//     or "version 0 / fully rolled back" for DirectionDown.
//
// For DirectionUp, Steps are executed in ascending version order and only
// steps with Version > currentVersion are included.
//
// For DirectionDown, Steps are executed in descending version order and only
// steps with Version <= currentVersion down to target are included.
func BuildPlan(steps []Step, currentVersion uint64, direction Direction, target uint64) (Plan, error) {
	// Filter to the requested direction.
	var dirSteps []Step
	for _, s := range steps {
		if s.Direction == direction {
			dirSteps = append(dirSteps, s)
		}
	}

	if direction == DirectionUp {
		// Sort ascending.
		sort.Slice(dirSteps, func(i, j int) bool { return dirSteps[i].Version < dirSteps[j].Version })

		var selected []Step
		for _, s := range dirSteps {
			if s.Version <= currentVersion {
				continue
			}
			if target != 0 && s.Version > target {
				break
			}
			selected = append(selected, s)
		}

		to := currentVersion
		if len(selected) > 0 {
			to = selected[len(selected)-1].Version
		}

		return Plan{Steps: selected, From: currentVersion, To: to}, nil
	}

	// DirectionDown.
	// Sort descending.
	sort.Slice(dirSteps, func(i, j int) bool { return dirSteps[i].Version > dirSteps[j].Version })

	var selected []Step
	for _, s := range dirSteps {
		if s.Version > currentVersion {
			continue
		}
		if s.Version <= target {
			break
		}
		selected = append(selected, s)
	}

	to := currentVersion
	if len(selected) > 0 {
		to = selected[len(selected)-1].Version
	}
	if len(selected) == 0 {
		to = currentVersion
	} else {
		// After rolling down, the DB is at the version just below the last step rolled back.
		to = target
	}

	if target > currentVersion {
		return Plan{}, fmt.Errorf("migration.BuildPlan: target %d is above currentVersion %d for down direction", target, currentVersion)
	}

	return Plan{Steps: selected, From: currentVersion, To: to}, nil
}
