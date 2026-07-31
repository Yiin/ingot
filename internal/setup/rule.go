package setup

// RuleDir is where a udev rule shipped by a package (rather than
// hand-edited by a user) belongs. It is searched before /etc/udev/rules.d
// and, thanks to the "70-" prefix on RuleName, sorts ahead of systemd's
// own 73-seat-late.rules — the rule that actually turns a "uaccess" tag
// into a granted ACL.
const RuleDir = "/usr/lib/udev/rules.d"

// RuleName is the rule file's name within RuleDir.
const RuleName = "70-ingot-input.rules"

// RulePath is the full path to the installed rule file.
const RulePath = RuleDir + "/" + RuleName

// RuleContent is the exact byte content Install writes to RulePath.
//
// systemd's own 70-uaccess.rules deliberately excludes keyboards from the
// uaccess tag, because a blanket keyboard ACL is a keylogging risk. This
// rule scopes the grant to devices udev has already classified as a
// keyboard via ID_INPUT_KEYBOARD, so Ingot never asks a user to join the
// input group — which is permanent and covers every input device,
// including mice.
const RuleContent = `SUBSYSTEM=="input", KERNEL=="event*", ENV{ID_INPUT_KEYBOARD}=="1", TAG+="uaccess"
`

// reloadCommands are the udevadm invocations that make an installed or
// removed rule take effect immediately, without requiring a re-login.
// Unexported and only ever read into installScript/uninstallScript: it is
// concatenated verbatim into a script that runs as root, so it must never
// be an exported var another package could mutate.
var reloadCommands = []string{
	"udevadm control --reload-rules",
	"udevadm trigger",
}

// ReloadCommands returns a copy of the udevadm invocations
// installScript/uninstallScript run after writing or removing the rule,
// for display to the user.
func ReloadCommands() []string {
	return append([]string(nil), reloadCommands...)
}
