# Copper for Wayland

Copper combines a to-do list, a clipboard, and a scratchpad in one narrow panel
that floats on the right edge of your screen. Select text anywhere, press your
capture key, and the text lands in the panel as a note. Later you copy notes
back into a chat, one by one or as a numbered list, and check them off as done.

This is a recreation of [shadcn's Copper](https://shadcn.com/copper) for macOS,
rebuilt for Linux/Wayland in Rust. It is unofficial and not affiliated with
shadcn. Everything stays local: no account, no sync, one JSON file.

## Install

System dependencies:

- GTK 4 (4.10 or newer)
- gtk4-layer-shell
- wl-clipboard (`wl-copy`, `wl-paste`)

On Arch Linux:

```sh
sudo pacman -S gtk4 gtk4-layer-shell wl-clipboard
```

Build and install:

```sh
cargo install --path .
```

Or build without installing:

```sh
cargo build --release
# binary at ./target/release/copper
```

## Usage

Start the daemon. It shows the panel and listens for commands:

```sh
copper
```

Then, from anywhere:

```sh
copper capture       # capture the current selection as a note
copper toggle        # show or hide the panel
copper add "text"    # add a note directly
copper listen        # optional: capture on double Shift (see below)
```

`copper capture` reads the primary selection with `wl-paste`. If that is empty
it falls back to the regular clipboard. The daemon shows a small "Captured"
toast when a note lands. If the daemon is not running, the command prints an
error and exits with status 1.

## Compositor keybindings

Bind `copper capture` to a key. This is the primary way to capture.

Hyprland (`~/.config/hyprland/hyprland.conf`):

```
bind = SUPER, C, exec, copper capture
bind = SUPER, V, exec, copper toggle
```

sway (`~/.config/sway/config`):

```
bindsym Mod4+c exec copper capture
bindsym Mod4+v exec copper toggle
```

## Double Shift capture (optional)

The macOS original captures when you tap Shift twice. `copper listen` does the
same on Linux. It watches every keyboard under `/dev/input/event*` and runs a
capture when it sees two Shift presses within 400 ms with no other key in
between:

```sh
copper listen
```

This needs read access to input devices. Add your user to the `input` group
(and log in again), or install a udev rule:

```sh
sudo usermod -aG input $USER
```

Start it from your compositor autostart if you want it always on. You do not
need it if you bind `copper capture` to a key instead.

## Inside the panel

- Type in the bottom field and press Enter to add a note to the active list.
- Click a card to select it. Click more cards to multi-select.
- Right-click a card for the action menu. The "..." button at the top switches
  the active list or creates a new one.
- The search field filters notes across all lists.

Keyboard shortcuts:

| Key | Action |
| --- | --- |
| Up / Down | Move selection |
| Space | Toggle done |
| Ctrl+C | Copy selected notes |
| Ctrl+Shift+C | Copy as a numbered list (`1. ...`) |
| Enter | Edit the note inline (Enter saves, Esc cancels) |
| Delete | Delete selected notes |
| Ctrl+Shift+M | Merge selected notes into one |
| Escape | Hide the panel |

Done notes stay visible with a strikethrough until you delete them.

## Where notes are stored

Notes live in `~/.local/share/copper/store.json`. It is plain, human-readable
JSON. The daemon saves it on every change. The daemon talks to the CLI over a
unix socket at `$XDG_RUNTIME_DIR/copper.sock` (fallback
`/tmp/copper-$UID.sock`).

## Not affiliated

This project is an unofficial recreation. It is not affiliated with, endorsed
by, or connected to shadcn. The name and concept credit the original
[Copper for macOS](https://shadcn.com/copper). Possible future work: Expand
and Edit in New Window from the original menu.

## License

MIT. See [LICENSE](LICENSE).
