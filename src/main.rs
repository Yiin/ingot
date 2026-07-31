mod client;
mod daemon;
mod ipc;
mod listen;
mod store;

use std::process::ExitCode;

fn usage() -> ! {
    eprintln!("usage:");
    eprintln!("  copper            run the daemon (panel + IPC listener)");
    eprintln!("  copper capture    capture the current selection as a note");
    eprintln!("  copper toggle     show or hide the panel");
    eprintln!("  copper add <text> add a note directly");
    eprintln!("  copper listen     double-Shift capture detector (evdev, optional)");
    std::process::exit(2)
}

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    match args.first().map(String::as_str) {
        None | Some("daemon") => {
            daemon::run();
            ExitCode::SUCCESS
        }
        Some("capture") => client::capture(),
        Some("toggle") => client::send_or_report(ipc::Request::Toggle),
        Some("add") => {
            let text = args[1..].join(" ");
            if text.trim().is_empty() {
                usage();
            }
            client::send_or_report(ipc::Request::Add { text })
        }
        Some("listen") => listen::run(),
        Some("-h" | "--help" | "help") => usage(),
        Some(_) => usage(),
    }
}
