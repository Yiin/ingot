use std::io::{BufRead, BufReader, Write};
use std::io;
use std::os::unix::net::UnixStream;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "cmd", rename_all = "snake_case")]
pub enum Request {
    Capture { text: String },
    Add { text: String },
    Toggle,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Response {
    pub ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

pub fn socket_path() -> PathBuf {
    match std::env::var("XDG_RUNTIME_DIR") {
        Ok(dir) if !dir.is_empty() => PathBuf::from(dir).join("copper.sock"),
        _ => {
            let uid = unsafe { libc::getuid() };
            PathBuf::from(format!("/tmp/copper-{uid}.sock"))
        }
    }
}

pub fn send(req: &Request) -> io::Result<Response> {
    let mut stream = UnixStream::connect(socket_path())?;
    let mut msg = serde_json::to_string(req)
        .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))?;
    msg.push('\n');
    stream.write_all(msg.as_bytes())?;
    let mut line = String::new();
    BufReader::new(stream).read_line(&mut line)?;
    serde_json::from_str(line.trim()).map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))
}
