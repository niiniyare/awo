// Package settings implements the Awo platform Settings module.
//
// Settings provide hierarchical, per-scope configuration: system defaults are
// overridden by tenant-level values, which are further overridden by branch-
// level values. This allows global defaults (e.g. default locale) while
// supporting per-tenant or per-location customisation without code changes.
//
// All settings are cached in Redis (5 minute TTL). Cache is invalidated
// immediately when a setting is written.
//
// The module owns one system entity:
//
//   - platform_setting — a single scoped key-value configuration entry
package settings

import "awo.so/awo/def"

func init() {
	def.Register(&SettingDefinition)
}
