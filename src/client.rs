use std::process::{Command, ExitCode};

use crate::ipc::{self, Request};

/// `copper capture`: read the current selection and send it to the daemon.
pub fn capture() -> ExitCode {
    match read_selection() {
        Some(text) => send_or_report(Request::Capture { text }),
        None => {
            eprintln!("copper: no text in the primary selection or clipboard");
            ExitCode::FAILURE
        }
    }
}

/// `copper toggle` / `copper add <text>`.
pub fn send_or_report(req: Request) -> ExitCode {
    match ipc::send(&req) {
        Ok(resp) if resp.ok => ExitCode::SUCCESS,
        Ok(resp) => {
            eprintln!(
                "copper: daemon error: {}",
                resp.error.unwrap_or_else(|| "unknown".into())
            );
            ExitCode::FAILURE
        }
        Err(e) => {
            eprintln!(
                "copper: cannot reach the daemon at {}: {e}",
                ipc::socket_path().display()
            );
            eprintln!("copper: is the daemon running? Start it with `copper`.");
            ExitCode::FAILURE
        }
    }
}

/// Primary selection first, regular clipboard as fallback.
fn read_selection() -> Option<String> {
    for args in [&["--primary", "--no-newline"][..], &["--no-newline"][..]] {
        if let Ok(out) = Command::new("wl-paste").args(args).output() {
            if out.status.success() {
                let text = String::from_utf8_lossy(&out.stdout).to_string();
                if !text.trim().is_empty() {
                    return Some(text);
                }
            }
        }
    }
    None
}
