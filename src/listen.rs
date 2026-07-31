//! `copper listen`: optional double-Shift detector over evdev.
//!
//! Watches every readable /dev/input/event* device. Two Shift presses
//! within 400 ms, with no other key press in between, trigger a capture
//! (same as `copper capture`).
//!
//! This needs read access to input devices: put your user in the `input`
//! group or install a udev rule. The primary integration stays a
//! compositor keybinding for `copper capture`; this is an optional extra.

use std::io::Read;
use std::process::ExitCode;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

const EV_KEY: u16 = 0x01;
const KEY_LEFTSHIFT: u16 = 42;
const KEY_RIGHTSHIFT: u16 = 54;
const DOUBLE_MS: u64 = 400;

pub fn run() -> ExitCode {
    let entries = match std::fs::read_dir("/dev/input") {
        Ok(e) => e,
        Err(e) => {
            eprintln!("copper listen: cannot read /dev/input: {e}");
            return ExitCode::FAILURE;
        }
    };

    let mut devices = Vec::new();
    for entry in entries.flatten() {
        let name = entry.file_name().to_string_lossy().into_owned();
        if !name.starts_with("event") {
            continue;
        }
        let path = entry.path();
        if let Ok(file) = std::fs::OpenOptions::new().read(true).open(&path) {
            devices.push((path, file));
        }
    }

    if devices.is_empty() {
        eprintln!("copper listen: no readable input devices found.");
        eprintln!(
            "copper listen: add your user to the `input` group or install a udev rule, then retry."
        );
        return ExitCode::FAILURE;
    }
    for (path, _) in &devices {
        eprintln!("copper listen: watching {}", path.display());
    }

    let detector = Arc::new(Mutex::new(Detector::default()));
    let mut handles = Vec::new();
    for (path, mut file) in devices {
        let detector = detector.clone();
        handles.push(std::thread::spawn(move || {
            // struct input_event on 64-bit Linux:
            //   timeval (16 bytes), type (u16), code (u16), value (i32)
            let mut buf = [0u8; 24];
            loop {
                if file.read_exact(&mut buf).is_err() {
                    eprintln!("copper listen: {} went away, thread exiting", path.display());
                    return;
                }
                let ev_type = u16::from_ne_bytes([buf[16], buf[17]]);
                let code = u16::from_ne_bytes([buf[18], buf[19]]);
                let value = i32::from_ne_bytes([buf[20], buf[21], buf[22], buf[23]]);
                if ev_type != EV_KEY {
                    continue;
                }
                detector.lock().unwrap().on_key(code, value);
            }
        }));
    }
    for h in handles {
        let _ = h.join();
    }
    ExitCode::SUCCESS
}

#[derive(Default)]
struct Detector {
    last_shift: Option<Instant>,
}

impl Detector {
    fn on_key(&mut self, code: u16, value: i32) {
        // value: 0 = release, 1 = press, 2 = autorepeat. Only presses count.
        if value != 1 {
            return;
        }
        let is_shift = code == KEY_LEFTSHIFT || code == KEY_RIGHTSHIFT;
        if !is_shift {
            // Any other key in between breaks the double press.
            self.last_shift = None;
            return;
        }
        let now = Instant::now();
        if let Some(t) = self.last_shift {
            if now.duration_since(t) <= Duration::from_millis(DOUBLE_MS) {
                self.last_shift = None;
                trigger_capture();
                return;
            }
        }
        self.last_shift = Some(now);
    }
}

fn trigger_capture() {
    std::thread::spawn(|| {
        eprintln!("copper listen: double Shift detected, capturing");
        crate::client::capture();
    });
}
