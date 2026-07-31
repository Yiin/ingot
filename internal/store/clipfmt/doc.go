// Package clipfmt renders selected notes for the clipboard: Copy joins
// raw bodies verbatim, CopyAsList numbers them into a Markdown ordered
// list, and HasMarkup tells the clipboard layer whether a string is
// worth offering as text/markdown alongside text/plain. Every function
// is pure and takes notes already in document order — clipfmt itself
// never sorts.
package clipfmt
