# Ingot

Ingot is a keyboard-first quick-capture panel for Wayland: a to-do list, a clipboard, and a scratchpad for AI-assisted work, always one shortcut away.

![Ingot panel showing a note list grouped into sections, a completed note, a markdown prompt, and the composer](assets/screenshot.png)

## What it does

- Press Shift twice anywhere and your current text selection lands in the panel, no matter what app has focus.
- Type notes and prompts into a composer at the bottom of the panel.
- Organize notes into named sections inside named projects, and tick items off.
- Select several notes and "Copy as List" to put a numbered list on the clipboard, ready to paste into ChatGPT, Claude, or Cursor.
- Everything lives in plain Markdown files on disk. No sync, no account, no telemetry.

## About the original

Ingot is an unofficial reimplementation of [Copper](https://shadcn.com/copper) by shadcn, built for Wayland, a platform the original does not serve. It is not affiliated with or endorsed by him. He built the idea; if you're on a Mac, buy the real thing at shadcn.com/copper.

## Install

### AUR (Arch, Manjaro, EndeavourOS)

```
paru -S ingot
```

Or `paru -S ingot-git` to track `main` instead of the latest tagged release. Packaging sources are under [`contrib/`](contrib/): `contrib/PKGBUILD` for `ingot`, `contrib/ingot-git/PKGBUILD` for `ingot-git`. Both install a udev rule (`ingot doctor` explains what it grants), a desktop entry, an icon, and a systemd **user** unit — the unit ships disabled; run `systemctl --user enable --now ingot.service` or `ingot doctor --fix` to turn it on.

A prebuilt binary from GitHub Releases, once one exists, will be dynamically linked against `libgtk-4.so.1`, `libgtk4-layer-shell.so.0` and glibc, so it realistically only serves Arch and Fedora; the AUR package is the tested path.

### go install

```
go install github.com/Yiin/ingot/cmd/ingot@latest
```

`go install` compiles from source, and the build needs cgo. Make sure you have a C compiler, `pkg-config`, GTK4 development headers, and gtk4-layer-shell development headers first. On Arch: `pacman -S gtk4 gtk4-layer-shell pkgconf gcc`. Ubuntu before 26.04 has neither a new enough GTK4 nor a gtk4-layer-shell package, so this path won't work there yet.

### Build from source

```
git clone https://github.com/Yiin/ingot.git
cd ingot
make build
```

Same dependencies as above: Go 1.25 or newer, a C compiler, `pkg-config`, GTK4 4.22 or newer, and gtk4-layer-shell.

## Setup

Ingot reads your keyboard directly from `/dev/input` to catch the double-tap Shift chord, even when the panel isn't focused. Run:

```
ingot doctor
```

It checks your permissions and installs a udev rule if you're missing one. Be clear about what that rule does: it grants your logged-in session read access to keyboard devices through udev's `uaccess` tag, the same mechanism that already grants you access to your mouse and webcam. systemd's own rules deliberately leave keyboards out of `uaccess`, because reading raw keyboard input is what a keylogger does. Ingot narrows that risk at the read boundary: it reduces every event to "was this Shift, was it something else, and when," never logs or stores key codes, and stops reading while your session is locked.

## Keybinds

Ingot reads the Shift-Shift chord directly from the keyboard, so capture needs no compositor keybind. To show or hide the panel manually, bind a key to `ingot toggle`:

Hyprland (`hyprland.conf`):

```
bind = SUPER, grave, exec, ingot toggle
```

sway (`config`):

```
bindsym $mod+grave exec ingot toggle
```

## The panel window

The panel is an ordinary window. Move it, resize it, fullscreen it, or send it to another workspace with the same keys you use for everything else. Ingot remembers the size you leave it at, in `~/.local/state/ingot/panel.json`. Fullscreen and maximised sizes aren't remembered, so a temporary fullscreen doesn't become the new default.

It's undecorated, because your compositor already draws the frame.

Wayland gives an app no say in where its own window opens, so that part is up to your compositor. Most tiling setups will want to float it. Match on the app id `lt.yiin.ingot`:

Hyprland (`hyprland.conf`):

```
windowrule = float on, match:class ^lt\.yiin\.ingot$
```

sway (`config`):

```
for_window [app_id="lt.yiin.ingot"] floating enable
```

Add `size` or `move` rules there too if you want it in a fixed spot. A `size` rule overrides the remembered size on every open, which is worth knowing if you'd rather resize it by hand.

## Where your notes live

Each project is one Markdown file under `$XDG_DATA_HOME/ingot/projects` (usually `~/.local/share/ingot/projects`). Sections are `##` headings, tasks are `- [ ]` and `- [x]`, and anything else plain is a note. Continuation lines are indented two spaces, which keeps the file readable and means editing it by hand never confuses Ingot.

```markdown
---
ingot: 1
id: 3f2a9c
title: Launch
created: 2026-07-31T09:00:00Z
---

## Today

- [ ] Draft the release notes for v0.1
- [x] Fix the section sorter
- Idea: ship a dark theme before v1
  continuation lines are indented two spaces like this
```

Open it in any editor. Put it in git if you want history. Ingot doesn't need to be running for the file to make sense.

## Why no Flatpak

Hyprland, sway, and niri all block the two protocols Ingot depends on (data-control for the clipboard, layer-shell for the capture toast) for apps running inside a Flatpak security context. There's no sandboxed build until that changes upstream.

## License

GPL-3.0. See [LICENSE](LICENSE). Copyright (C) 2026 Stanislovas Janonis.
