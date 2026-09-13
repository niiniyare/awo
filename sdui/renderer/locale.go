package renderer

// LocaleFormats holds the date/number format conventions for a BCP 47 locale.
// Populated from a static table — no external dependencies, no file I/O.
type LocaleFormats struct {
	// DateFormat is the day/month/year display pattern (e.g. "DD/MM/YYYY").
	DateFormat string

	// DateTimeFormat is the combined date-time pattern (e.g. "DD/MM/YYYY HH:mm").
	DateTimeFormat string

	// DecimalSeparator is "." or ",".
	DecimalSeparator string

	// ThousandSeparator is "," or "." or " " (narrow no-break space).
	ThousandSeparator string

	// RTL is true when the locale uses a right-to-left script.
	RTL bool
}

// LocaleFormatsFor returns format conventions for the given BCP 47 locale tag.
// The tag is matched first exactly, then by language subtag only.
// Falls back to "en-US" when the tag is empty or unrecognised.
//
// Examples:
//
//	LocaleFormatsFor("ar-SA")  →  {DateFormat:"DD/MM/YYYY", RTL:true, ...}
//	LocaleFormatsFor("de-DE")  →  {DecimalSeparator:",", ThousandSeparator:"."}
//	LocaleFormatsFor("")       →  en-US defaults
func LocaleFormatsFor(locale string) LocaleFormats {
	if f, ok := localeTable[locale]; ok {
		return f
	}
	// Language-only fallback (e.g. "en" → "en-US").
	if len(locale) >= 2 {
		lang := locale[:2]
		if f, ok := langFallback[lang]; ok {
			return f
		}
	}
	return localeTable["en-US"]
}

// ApplyLocale populates the locale-derived fields of ctx from the given locale
// tag. Other fields in ctx (Theme, GridSystem) are left unchanged.
// Returns ctx so callers can chain: ctx = renderer.ApplyLocale(ctx, locale).
func ApplyLocale(ctx RendererContext, locale string) RendererContext {
	f := LocaleFormatsFor(locale)
	ctx.DateFormat = f.DateFormat
	ctx.DateTimeFormat = f.DateTimeFormat
	ctx.DecimalSeparator = f.DecimalSeparator
	ctx.ThousandSeparator = f.ThousandSeparator
	ctx.RTL = f.RTL
	return ctx
}

// ── Locale table ──────────────────────────────────────────────────────────────
//
// Covers locales most likely to appear in an ERP context.
// Extend as needed; the table is the single source of truth.

var localeTable = map[string]LocaleFormats{
	// ── English ───────────────────────────────────────────────────────────────
	"en-US": {
		DateFormat:        "MM/DD/YYYY",
		DateTimeFormat:    "MM/DD/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	"en-GB": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	"en-AU": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	"en-CA": {
		DateFormat:        "YYYY-MM-DD",
		DateTimeFormat:    "YYYY-MM-DD HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	// ── German ────────────────────────────────────────────────────────────────
	"de-DE": {
		DateFormat:        "DD.MM.YYYY",
		DateTimeFormat:    "DD.MM.YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	"de-AT": {
		DateFormat:        "DD.MM.YYYY",
		DateTimeFormat:    "DD.MM.YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	"de-CH": {
		DateFormat:        "DD.MM.YYYY",
		DateTimeFormat:    "DD.MM.YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: "'",
	},
	// ── French ────────────────────────────────────────────────────────────────
	"fr-FR": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: "\u202f", // narrow no-break space
	},
	"fr-BE": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	"fr-CH": {
		DateFormat:        "DD.MM.YYYY",
		DateTimeFormat:    "DD.MM.YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: "'",
	},
	// ── Spanish ───────────────────────────────────────────────────────────────
	"es-ES": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	"es-MX": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	"es-AR": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	// ── Portuguese ────────────────────────────────────────────────────────────
	"pt-BR": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	"pt-PT": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	// ── Italian ───────────────────────────────────────────────────────────────
	"it-IT": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	// ── Dutch ─────────────────────────────────────────────────────────────────
	"nl-NL": {
		DateFormat:        "DD-MM-YYYY",
		DateTimeFormat:    "DD-MM-YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	// ── Polish ────────────────────────────────────────────────────────────────
	"pl-PL": {
		DateFormat:        "DD.MM.YYYY",
		DateTimeFormat:    "DD.MM.YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: "\u202f",
	},
	// ── Russian ───────────────────────────────────────────────────────────────
	"ru-RU": {
		DateFormat:        "DD.MM.YYYY",
		DateTimeFormat:    "DD.MM.YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: "\u202f",
	},
	// ── Turkish ───────────────────────────────────────────────────────────────
	"tr-TR": {
		DateFormat:        "DD.MM.YYYY",
		DateTimeFormat:    "DD.MM.YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	// ── Japanese ──────────────────────────────────────────────────────────────
	"ja-JP": {
		DateFormat:        "YYYY/MM/DD",
		DateTimeFormat:    "YYYY/MM/DD HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	// ── Chinese ───────────────────────────────────────────────────────────────
	"zh-CN": {
		DateFormat:        "YYYY-MM-DD",
		DateTimeFormat:    "YYYY-MM-DD HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	"zh-TW": {
		DateFormat:        "YYYY/MM/DD",
		DateTimeFormat:    "YYYY/MM/DD HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	// ── Korean ────────────────────────────────────────────────────────────────
	"ko-KR": {
		DateFormat:        "YYYY.MM.DD",
		DateTimeFormat:    "YYYY.MM.DD HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	// ── Arabic ────────────────────────────────────────────────────────────────
	"ar-SA": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
		RTL:               true,
	},
	"ar-AE": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
		RTL:               true,
	},
	"ar-EG": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
		RTL:               true,
	},
	// ── Hebrew ────────────────────────────────────────────────────────────────
	"he-IL": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
		RTL:               true,
	},
	// ── Persian ───────────────────────────────────────────────────────────────
	"fa-IR": {
		DateFormat:        "YYYY/MM/DD",
		DateTimeFormat:    "YYYY/MM/DD HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
		RTL:               true,
	},
	// ── Hindi ─────────────────────────────────────────────────────────────────
	"hi-IN": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	// ── Thai ──────────────────────────────────────────────────────────────────
	"th-TH": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	// ── Indonesian ────────────────────────────────────────────────────────────
	"id-ID": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
	},
	// ── Malay ─────────────────────────────────────────────────────────────────
	"ms-MY": {
		DateFormat:        "DD/MM/YYYY",
		DateTimeFormat:    "DD/MM/YYYY HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
	// ── ISO 8601 (machine / API contexts) ─────────────────────────────────────
	"iso": {
		DateFormat:        "YYYY-MM-DD",
		DateTimeFormat:    "YYYY-MM-DD HH:mm",
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
	},
}

// langFallback maps a 2-letter language code to a canonical locale entry.
// Used when the region subtag is missing or unrecognised.
var langFallback = map[string]LocaleFormats{
	"en": localeTable["en-US"],
	"de": localeTable["de-DE"],
	"fr": localeTable["fr-FR"],
	"es": localeTable["es-ES"],
	"pt": localeTable["pt-BR"],
	"it": localeTable["it-IT"],
	"nl": localeTable["nl-NL"],
	"pl": localeTable["pl-PL"],
	"ru": localeTable["ru-RU"],
	"tr": localeTable["tr-TR"],
	"ja": localeTable["ja-JP"],
	"zh": localeTable["zh-CN"],
	"ko": localeTable["ko-KR"],
	"ar": localeTable["ar-SA"],
	"he": localeTable["he-IL"],
	"fa": localeTable["fa-IR"],
	"hi": localeTable["hi-IN"],
	"th": localeTable["th-TH"],
	"id": localeTable["id-ID"],
	"ms": localeTable["ms-MY"],
}
