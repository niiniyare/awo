package amis

// ThemeConfig holds AMIS SDK initialization options derived from a theme name.
// The web shell uses these values to configure the AMIS SDK on page load.
//
// AMIS theming is a SDK-init concern, not a per-schema concern. The schema
// carries a "theme" key only so the web shell knows which CSS/JS bundle to
// load at runtime.
type ThemeConfig struct {
	// Theme is the AMIS SDK theme name (e.g. "default", "antd", "cxd").
	// Passed to amis.init({ theme }) in the web shell.
	Theme string

	// ClassPrefix is the CSS class prefix AMIS uses for this theme.
	// "cxd-" for default/dark, "antd-" for Ant Design, "ang-" for Angular.
	ClassPrefix string

	// DarkMode indicates the web shell should apply the dark CSS override layer
	// (via html.dark class + CSS custom property tokens). AMIS has no built-in
	// dark CSS; dark mode is implemented by scaling --colors-neutral-* vars.
	DarkMode bool
}

// ThemeConfigFor resolves the AMIS SDK theme configuration for a theme name.
// Unrecognised names fall back to the default theme.
//
// Supported theme names:
//   - "default" or ""  → cxd (AMIS default)
//   - "antd"           → Ant Design
//   - "ang"            → Angular Material
//   - "dark"           → cxd + dark mode CSS overlay
//   - "compact"        → cxd with compact density CSS
func ThemeConfigFor(theme string) ThemeConfig {
	switch theme {
	case "antd":
		return ThemeConfig{Theme: "antd", ClassPrefix: "antd-"}
	case "ang":
		return ThemeConfig{Theme: "ang", ClassPrefix: "ang-"}
	case "dark":
		return ThemeConfig{Theme: "cxd", ClassPrefix: "cxd-", DarkMode: true}
	case "compact":
		return ThemeConfig{Theme: "cxd", ClassPrefix: "cxd-"}
	default:
		return ThemeConfig{Theme: "cxd", ClassPrefix: "cxd-"}
	}
}

// applyTheme emits the "theme" key on the AMIS page schema root.
// The web shell reads schema.theme to determine which AMIS SDK variant to
// activate. Renderers call this on page-level nodes only.
//
// Note: the "dark" pseudo-theme does NOT emit theme:"dark" because AMIS has
// no built-in dark CSS. Instead, the web shell's html.dark class activates
// CSS custom property overrides. The theme key emitted is always the base
// AMIS theme name ("cxd", "antd", "ang").
func applyTheme(out map[string]any, theme string) {
	cfg := ThemeConfigFor(theme)
	if cfg.Theme != "" && cfg.Theme != "cxd" {
		// "cxd" is the AMIS default; omitting the key keeps output minimal.
		out["theme"] = cfg.Theme
	}
	if cfg.DarkMode {
		// Signal the web shell to activate the dark CSS overlay.
		// Convention: web shell checks schema.darkMode === true.
		out["darkMode"] = true
	}
}
