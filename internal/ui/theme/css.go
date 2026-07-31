package theme

import _ "embed"

// CSS is the panel stylesheet, defining the :root design tokens as CSS
// custom properties and the classes every other internal/ui package styles
// against.
//
//go:embed style.css
var CSS string
