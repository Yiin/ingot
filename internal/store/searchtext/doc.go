// Package searchtext normalizes note bodies and section titles for
// search: NFKD decomposition, combining-mark stripping, case folding,
// and Markdown syntax stripping (emphasis markers, code-span backticks,
// link brackets and destinations), while retaining a byte-offset map
// back into the raw text. internal/store/fsstore uses it to implement
// Store.Search's case-, diacritic-, and Markdown-insensitive
// AND-substring matching, and Filter to compute section-header
// visibility for the same query.
package searchtext
