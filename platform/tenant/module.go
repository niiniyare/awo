// Package tenant implements the Awo platform Tenant module.
//
// The tenant module manages the top-level infrastructure isolation boundary:
// every request is scoped to exactly one tenant. It owns one system entity:
//
//   - platform_tenant — the tenant record itself (lifecycle state machine)
//
// Organisational hierarchy (org units, branches, divisions, departments, teams)
// lives in the platform/organization module — awo.so/awo/platform/organization.
//
// Registration happens in init() so that the definition is available to the
// compiler before any request is served.
package tenant

import "awo.so/awo/def"

func init() {
	def.Register(&Definition)
}
