package renderer

// Message keys for renderer-emitted UI strings that are not entity metadata.
const (
	MsgEmptyState = "empty_state"
	MsgActions    = "actions"
)

// messageTable maps a language subtag to its translated strings. Only English
// ships today; other languages fall back to English until translations are added.
var messageTable = map[string]map[string]string{
	"en": {
		MsgEmptyState: "No records found.",
		MsgActions:    "Actions",
	},
}

// Message returns the UI string for key in the given BCP 47 locale, matching on
// the language subtag and falling back to English, then to the key itself.
func Message(locale, key string) string {
	lang := "en"
	if len(locale) >= 2 {
		lang = locale[:2]
	}
	if m, ok := messageTable[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := messageTable["en"][key]; ok {
		return s
	}
	return key
}
