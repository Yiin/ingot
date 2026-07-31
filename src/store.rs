use std::fs;
use std::io;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Note {
    pub id: String,
    pub text: String,
    pub done: bool,
    pub created_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct List {
    pub name: String,
    pub notes: Vec<Note>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Store {
    pub lists: Vec<List>,
    pub active_list: String,
}

pub fn store_path() -> PathBuf {
    let base = std::env::var("XDG_DATA_HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|_| {
            let home = std::env::var("HOME").unwrap_or_else(|_| "/tmp".into());
            PathBuf::from(home).join(".local/share")
        });
    base.join("copper").join("store.json")
}

impl Default for Store {
    fn default() -> Self {
        Store {
            lists: vec![List {
                name: "Inbox".into(),
                notes: Vec::new(),
            }],
            active_list: "Inbox".into(),
        }
    }
}

impl Store {
    pub fn load() -> Store {
        let path = store_path();
        let mut store: Store = fs::read_to_string(&path)
            .ok()
            .and_then(|s| serde_json::from_str(&s).ok())
            .unwrap_or_default();
        if store.lists.is_empty() {
            store.lists.push(List {
                name: "Inbox".into(),
                notes: Vec::new(),
            });
        }
        if store.list(&store.active_list).is_none() {
            store.active_list = store.lists[0].name.clone();
        }
        store
    }

    pub fn save(&self) -> io::Result<()> {
        let path = store_path();
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        let json = serde_json::to_string_pretty(self)
            .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;
        let tmp = path.with_extension("json.tmp");
        fs::write(&tmp, json)?;
        fs::rename(&tmp, &path)
    }

    pub fn list(&self, name: &str) -> Option<&List> {
        self.lists.iter().find(|l| l.name == name)
    }

    pub fn list_mut(&mut self, name: &str) -> Option<&mut List> {
        self.lists.iter_mut().find(|l| l.name == name)
    }

    pub fn ensure_list(&mut self, name: &str) {
        if self.list(name).is_none() {
            self.lists.push(List {
                name: name.into(),
                notes: Vec::new(),
            });
        }
    }

    pub fn add_note(&mut self, text: &str) {
        let text = text.trim();
        if text.is_empty() {
            return;
        }
        let active = self.active_list.clone();
        self.ensure_list(&active);
        let note = Note {
            id: uuid::Uuid::new_v4().to_string(),
            text: text.into(),
            done: false,
            created_at: chrono::Local::now().to_rfc3339(),
        };
        if let Some(list) = self.list_mut(&active) {
            list.notes.push(note);
        }
    }

    /// Notes matching `ids`, in store order: (list index, note index, id).
    pub fn positions_of(&self, ids: &std::collections::HashSet<String>) -> Vec<(usize, usize)> {
        let mut out = Vec::new();
        for (li, list) in self.lists.iter().enumerate() {
            for (ni, note) in list.notes.iter().enumerate() {
                if ids.contains(&note.id) {
                    out.push((li, ni));
                }
            }
        }
        out
    }

    pub fn find_note_mut(&mut self, id: &str) -> Option<&mut Note> {
        self.lists
            .iter_mut()
            .flat_map(|l| l.notes.iter_mut())
            .find(|n| n.id == id)
    }
}
