package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"awo.so/awo/def"
)

// Fingerprint computes a deterministic SHA-256 hash of the CompiledSchema.
// Only structurally significant data is hashed (names, types, constraints,
// permissions) — display-only fields (Label, Description) are excluded.
// Used for cache invalidation and hot-reload detection.
func Fingerprint(s *CompiledSchema) string {
	h := sha256.New()

	// Sort entities by QualifiedName for determinism (Entities slice is
	// insertion-ordered, but we need hash stability regardless of registration order).
	qnames := make([]string, 0, len(s.Entities))
	for _, es := range s.Entities {
		qnames = append(qnames, es.QualifiedName)
	}
	sort.Strings(qnames)

	for _, name := range qnames {
		es := s.ByName[name]

		fmt.Fprintf(h, "entity:%s:system:%v\n", name, es.IsSystem)

		// Fields — sorted by name.
		fields := make([]def.FieldDef, len(es.Fields))
		copy(fields, es.Fields)
		sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
		for _, f := range fields {
			fmt.Fprintf(h, "field:%s:%s:req=%v:uniq=%v:imm=%v:sens=%v:search=%v\n",
				f.Name, f.Type, f.Required, f.Unique, f.Immutable, f.Sensitive, f.Searchable)
			if len(f.Options) > 0 {
				opts := make([]string, len(f.Options))
				copy(opts, f.Options)
				sort.Strings(opts)
				fmt.Fprintf(h, "field:%s:options:%s\n", f.Name, strings.Join(opts, ","))
			}
			if f.LinkTarget != "" {
				fmt.Fprintf(h, "field:%s:link:%s\n", f.Name, f.LinkTarget)
			}
			if f.Series != "" {
				fmt.Fprintf(h, "field:%s:series:%s\n", f.Name, f.Series)
			}
		}

		// Edges — sorted by name.
		edges := make([]def.EdgeDef, len(es.Edges))
		copy(edges, es.Edges)
		sort.Slice(edges, func(i, j int) bool { return edges[i].Name < edges[j].Name })
		for _, e := range edges {
			fmt.Fprintf(h, "edge:%s:%s:%s:cascade=%v\n", e.Name, e.Target, e.Type, e.CascadeDelete)
		}

		// Actions — sorted by name.
		actions := make([]def.ActionDef, len(es.Actions))
		copy(actions, es.Actions)
		sort.Slice(actions, func(i, j int) bool { return actions[i].Name < actions[j].Name })
		for _, a := range actions {
			fmt.Fprintf(h, "action:%s:%s\n", a.Name, a.Method)
		}

		// Permissions.
		perms := es.Permissions
		writePerms := func(label string, subjects []string) {
			cp := make([]string, len(subjects))
			copy(cp, subjects)
			sort.Strings(cp)
			for _, sub := range cp {
				fmt.Fprintf(h, "perm:%s:%s:%s\n", name, label, sub)
			}
		}
		writePerms("create", perms.Create)
		writePerms("read", perms.Read)
		writePerms("write", perms.Write)
		writePerms("delete", perms.Delete)
		actionNames := make([]string, 0, len(perms.Actions))
		for actionName := range perms.Actions {
			actionNames = append(actionNames, actionName)
		}
		sort.Strings(actionNames)
		for _, actionName := range actionNames {
			writePerms(actionName, perms.Actions[actionName])
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}
