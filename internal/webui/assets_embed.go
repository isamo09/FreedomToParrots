package webui

import "embed"

// assets holds every file under assets/ - HTML pages, the shared
// stylesheet and the panel's client-side JS. Keeping them as plain files
// (instead of Go string literals) is what makes editing the UI pleasant:
// syntax highlighting, no escaping, a real diff per change.
//
//go:embed assets/*.html assets/*.css assets/*.js
var assets embed.FS
