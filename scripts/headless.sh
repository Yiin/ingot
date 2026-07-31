#!/usr/bin/env bash
#
# scripts/headless.sh — run a command inside a headless sway session.
#
# Starts sway on a software-only headless Wayland backend, runs the given
# command inside that session once sway has come up, tears sway down, and
# exits with the command's exit status. Used by `make test-integration`
# and `make screenshot`.
#
# Requirements below are measured, not assumed — see copper-l2z's
# "Wayland behaviour" notes:
#
#   - XDG_RUNTIME_DIR must be a short path. AF_UNIX sun_path is capped at
#     108 bytes; a long runtime dir makes sway die with "Unable to open
#     wayland socket" plus an assertion failure.
#   - WAYLAND_DISPLAY, DISPLAY and XDG_SESSION_TYPE must be unset before
#     sway starts, or it nests inside the outer session instead of going
#     headless.
#   - WLR_BACKENDS=headless, WLR_LIBINPUT_NO_DEVICES=1 and
#     WLR_RENDERER=pixman drive sway itself; GSK_RENDERER=cairo drives the
#     GTK client under test. Both are pure software: no GPU, no /dev/dri,
#     no privileged container needed.
set -euo pipefail

if ! command -v sway >/dev/null 2>&1; then
	echo "headless.sh: sway is not installed" >&2
	exit 1
fi

if [ "$#" -eq 0 ]; then
	echo "usage: $(basename "$0") <command> [args...]" >&2
	exit 2
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

runtime_dir="/run/user/$(id -u)/ingot-test"
mkdir -p "$runtime_dir"
chmod 700 "$runtime_dir"

status_file="$work_dir/status"
run_script="$work_dir/run.sh"
sway_config="$work_dir/sway.conf"
sway_log="$work_dir/sway.log"

# sway's `exec` runs once the compositor is up, so the command under test
# only ever runs against a live session — no manual "wait for sway ready"
# polling needed. Quote every argument through %q so paths and flags with
# spaces survive the round trip through the generated script.
{
	printf '#!/usr/bin/env bash\n'
	printf '%q ' "$@"
	printf '\n'
	printf 'echo $? > %q\n' "$status_file"
	printf 'swaymsg exit >/dev/null 2>&1 || true\n'
} >"$run_script"
chmod +x "$run_script"

cat >"$sway_config" <<EOF
output HEADLESS-1 mode 1920x1080
exec $run_script
EOF

unset WAYLAND_DISPLAY DISPLAY XDG_SESSION_TYPE

export XDG_RUNTIME_DIR="$runtime_dir"
export WLR_BACKENDS=headless
export WLR_LIBINPUT_NO_DEVICES=1
export WLR_RENDERER=pixman
export GSK_RENDERER=cairo

if ! sway -c "$sway_config" >"$sway_log" 2>&1; then
	status=$?
	echo "headless.sh: sway exited with status $status" >&2
	cat "$sway_log" >&2
	exit "$status"
fi

if [ ! -f "$status_file" ]; then
	echo "headless.sh: command never ran (sway exited before its exec line completed)" >&2
	cat "$sway_log" >&2
	exit 1
fi

exit "$(cat "$status_file")"
