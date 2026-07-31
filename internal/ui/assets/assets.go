// Package assets embeds binary resources bundled into the ingot binary,
// currently the Inter Variable font. Embedding it means the app's look does
// not depend on the user's fontconfig having Inter installed.
package assets

import _ "embed"

// InterVariableTTF is the Inter Variable font (SIL OFL 1.1, see
// InterVariableLicense), used for every label in the panel so the app does
// not depend on the user's fontconfig having Inter installed.
//
//go:embed InterVariable.ttf
var InterVariableTTF []byte

// InterVariableLicense is the SIL Open Font License text required to
// accompany redistribution of InterVariableTTF.
//
//go:embed InterVariable-LICENSE.txt
var InterVariableLicense []byte
