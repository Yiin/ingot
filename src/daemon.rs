use std::cell::RefCell;
use std::collections::{HashMap, HashSet};
use std::io::{BufRead, BufReader, Write};
use std::os::unix::net::UnixListener;
use std::rc::Rc;
use std::time::Duration;

use gtk4::prelude::*;
use gtk4::{gdk, glib, Orientation};
use gtk4_layer_shell::{Edge, KeyboardMode, Layer, LayerShell};

use crate::ipc::{socket_path, Request, Response};
use crate::store::{Note, Store};

const CSS: &str = r#"
window { background: transparent; }
.panel {
    background: #f7f7f5;
    border: 1px solid #e2e2e0;
    border-radius: 16px;
    margin: 8px 8px 8px 0;
}
.header { padding: 10px 10px 6px 10px; }
.search-pill {
    background: #ffffff;
    border: 1px solid #e5e5e3;
    border-radius: 999px;
    min-height: 30px;
}
.menu-btn {
    border-radius: 999px;
    min-width: 30px;
    min-height: 30px;
    padding: 0;
    background: #ffffff;
    border: 1px solid #e5e5e3;
}
.list-header {
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.08em;
    color: #6b7280;
    margin: 10px 14px 2px 14px;
}
.notes { background: transparent; }
.notes row { background: transparent; padding: 0; }
.card {
    background: #ffffff;
    border: 1px solid #e5e5e3;
    border-radius: 10px;
    padding: 8px 10px;
    margin: 3px 8px;
}
.card.selected {
    border-color: #2563eb;
    box-shadow: 0 0 0 1px #2563eb;
}
.note-text { color: #111827; }
.note-text.done {
    color: #9ca3af;
    text-decoration: line-through;
}
checkbutton.circle-check { padding: 0; min-width: 0; min-height: 0; }
checkbutton.circle-check check {
    border-radius: 999px;
    min-width: 18px;
    min-height: 18px;
    padding: 0;
    margin: 2px;
    background: #ffffff;
    border: 1.5px solid #9ca3af;
    color: transparent;
    -gtk-icon-size: 12px;
}
checkbutton.circle-check check:checked {
    background: #2563eb;
    border-color: #2563eb;
    color: #ffffff;
}
.input {
    background: #ffffff;
    border: 1px solid #e5e5e3;
    border-radius: 999px;
    margin: 6px 10px 10px 10px;
    min-height: 34px;
    padding: 0 12px;
}
.toast {
    background: #1f2937;
    color: #f9fafb;
    border-radius: 999px;
    padding: 6px 16px;
    font-size: 13px;
}
.context-menu { padding: 4px; }
.context-menu button {
    border-radius: 6px;
    padding: 5px 8px;
    background: none;
    border: none;
    box-shadow: none;
}
.context-menu button:hover { background: #eef0f2; }
.context-menu .shortcut { color: #9ca3af; font-size: 11px; }
.context-menu .section-label {
    color: #6b7280;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.08em;
    padding: 4px 8px;
}
"#;

struct Ui {
    window: gtk4::Window,
    notes_box: gtk4::ListBox,
    search: gtk4::SearchEntry,
    menu_popover: gtk4::Popover,
    menu_stack: gtk4::Stack,
    menu_main: gtk4::Box,
    new_list_entry: gtk4::Entry,
    input: gtk4::Entry,
    toast: gtk4::Label,
    rows: RefCell<HashMap<String, gtk4::ListBoxRow>>,
}

struct App {
    store: Store,
    selected: HashSet<String>,
    cursor: Option<String>,
    visible: Vec<String>,
    rebuilding: bool,
}

type Shared<T> = Rc<RefCell<T>>;

pub fn run() {
    let app = gtk4::Application::new(Some("lt.yiin.copper"), Default::default());
    app.connect_activate(|app| {
        let ui = Rc::new(build_ui(app));
        let state: Shared<App> = Rc::new(RefCell::new(App {
            store: Store::load(),
            selected: HashSet::new(),
            cursor: None,
            visible: Vec::new(),
            rebuilding: false,
        }));
        wire_ui(&state, &ui);
        rebuild(&state, &ui);
        DAEMON.with(|d| *d.borrow_mut() = Some((state.clone(), ui.clone())));
        start_ipc(state.clone(), ui.clone());
    });
    app.run();
}

fn build_ui(app: &gtk4::Application) -> Ui {
    let provider = gtk4::CssProvider::new();
    provider.load_from_data(CSS);
    gtk4::style_context_add_provider_for_display(
        &gdk::Display::default().expect("no display"),
        &provider,
        gtk4::STYLE_PROVIDER_PRIORITY_APPLICATION,
    );

    let window = gtk4::Window::builder()
        .application(app)
        .title("Copper")
        .decorated(false)
        .resizable(false)
        .build();
    window.init_layer_shell();
    window.set_layer(Layer::Overlay);
    window.set_anchor(Edge::Right, true);
    window.set_anchor(Edge::Bottom, true);
    window.set_anchor(Edge::Top, false);
    window.set_anchor(Edge::Left, false);
    window.set_exclusive_zone(0);
    window.set_keyboard_mode(KeyboardMode::OnDemand);

    // Size to full screen height. Layer-shell configure gave no usable
    // size here, so take the monitor geometry as the target. Anchored to
    // the bottom-right, this lands exactly on the screen edges.
    if let Some(display) = gdk::Display::default() {
        if let Some(monitor) = display.monitors().item(0).and_downcast::<gdk::Monitor>() {
            let geom = monitor.geometry();
            window.set_default_size(360, geom.height());
        }
    }

    let panel = gtk4::Box::new(Orientation::Vertical, 0);
    panel.add_css_class("panel");
    panel.set_width_request(360);
    panel.set_vexpand(true);

    // Header: search pill + "..." menu button.
    let header = gtk4::Box::new(Orientation::Horizontal, 6);
    header.add_css_class("header");
    let search = gtk4::SearchEntry::new();
    search.add_css_class("search-pill");
    search.set_hexpand(true);
    search.set_placeholder_text(Some("Search"));
    let menu_button = gtk4::Button::with_label("…");
    menu_button.add_css_class("menu-btn");
    header.append(&search);
    header.append(&menu_button);
    panel.append(&header);

    // Menu popover: two pages (list of lists, new-list entry) in a stack.
    let menu_stack = gtk4::Stack::new();
    let menu_main = gtk4::Box::new(Orientation::Vertical, 0);
    menu_main.add_css_class("context-menu");
    menu_stack.add_named(&menu_main, Some("main"));
    let new_list_entry = gtk4::Entry::new();
    new_list_entry.set_placeholder_text(Some("List name"));
    new_list_entry.set_margin_start(6);
    new_list_entry.set_margin_end(6);
    new_list_entry.set_margin_top(4);
    new_list_entry.set_margin_bottom(4);
    menu_stack.add_named(&new_list_entry, Some("new"));
    menu_stack.set_visible_child(&menu_main);
    let menu_popover = gtk4::Popover::new();
    menu_popover.set_child(Some(&menu_stack));
    menu_popover.set_parent(&menu_button);
    menu_button.connect_clicked({
        let popover = menu_popover.clone();
        let stack = menu_stack.clone();
        let main = menu_main.clone();
        move |_| {
            stack.set_visible_child(&main);
            popover.popup();
        }
    });

    // Notes area.
    let notes_box = gtk4::ListBox::new();
    notes_box.add_css_class("notes");
    notes_box.set_selection_mode(gtk4::SelectionMode::None);
    let scrolled = gtk4::ScrolledWindow::new();
    scrolled.set_hscrollbar_policy(gtk4::PolicyType::Never);
    scrolled.set_vexpand(true);
    scrolled.set_child(Some(&notes_box));
    panel.append(&scrolled);

    // Bottom input row.
    let input = gtk4::Entry::new();
    input.add_css_class("input");
    input.set_icon_from_icon_name(gtk4::EntryIconPosition::Primary, Some("media-record-symbolic"));
    panel.append(&input);

    // Overlay for the toast.
    let overlay = gtk4::Overlay::new();
    overlay.set_child(Some(&panel));
    let toast = gtk4::Label::new(Some("Captured"));
    toast.add_css_class("toast");
    toast.set_halign(gtk4::Align::Center);
    toast.set_valign(gtk4::Align::End);
    toast.set_margin_bottom(56);
    toast.set_visible(false);
    overlay.add_overlay(&toast);

    window.set_child(Some(&overlay));
    window.present();

    Ui {
        window,
        notes_box,
        search,
        menu_popover,
        menu_stack,
        menu_main,
        new_list_entry,
        input,
        toast,
        rows: RefCell::new(HashMap::new()),
    }
}

fn wire_ui(state: &Shared<App>, ui: &Rc<Ui>) {
    // Search filters cards.
    ui.search.connect_search_changed({
        let state = state.clone();
        let ui = ui.clone();
        move |_| rebuild(&state, &ui)
    });

    // Bottom input adds a note to the active list.
    ui.input.connect_activate({
        let state = state.clone();
        let ui = ui.clone();
        move |entry| {
            let text = entry.text().to_string();
            if text.trim().is_empty() {
                return;
            }
            {
                let mut st = state.borrow_mut();
                st.store.add_note(&text);
                save(&st);
            }
            entry.set_text("");
            rebuild(&state, &ui);
        }
    });

    // New-list entry in the menu popover.
    ui.new_list_entry.connect_activate({
        let state = state.clone();
        let ui = ui.clone();
        move |entry| {
            let name = entry.text().trim().to_string();
            if !name.is_empty() {
                {
                    let mut st = state.borrow_mut();
                    st.store.ensure_list(&name);
                    st.store.active_list = name;
                    save(&st);
                }
                entry.set_text("");
                ui.menu_popover.popdown();
                rebuild(&state, &ui);
            }
        }
    });

    // Panel-wide keyboard shortcuts.
    let keyctl = gtk4::EventControllerKey::new();
    keyctl.connect_key_pressed({
        let state = state.clone();
        let ui = ui.clone();
        move |_, key, _, mods| {
            let ctrl = mods.contains(gdk::ModifierType::CONTROL_MASK);
            let shift = mods.contains(gdk::ModifierType::SHIFT_MASK);
            match (key, ctrl, shift) {
                (gdk::Key::Escape, _, _) => {
                    ui.window.set_visible(false);
                    glib::Propagation::Stop
                }
                (gdk::Key::Up, false, false) => {
                    move_cursor(&state, &ui, -1);
                    glib::Propagation::Stop
                }
                (gdk::Key::Down, false, false) => {
                    move_cursor(&state, &ui, 1);
                    glib::Propagation::Stop
                }
                (gdk::Key::space, false, false) => {
                    toggle_done(&state, &ui);
                    glib::Propagation::Stop
                }
                (gdk::Key::c | gdk::Key::C, true, false) => {
                    copy_selected(&state, &ui, false);
                    glib::Propagation::Stop
                }
                (gdk::Key::C | gdk::Key::c, true, true) => {
                    copy_selected(&state, &ui, true);
                    glib::Propagation::Stop
                }
                (gdk::Key::Return | gdk::Key::KP_Enter, false, false) => {
                    edit_cursor(&state, &ui);
                    glib::Propagation::Stop
                }
                (gdk::Key::Delete | gdk::Key::BackSpace, false, _) => {
                    delete_selected(&state, &ui);
                    glib::Propagation::Stop
                }
                (gdk::Key::M | gdk::Key::m, true, true) => {
                    merge_selected(&state, &ui);
                    glib::Propagation::Stop
                }
                _ => glib::Propagation::Proceed,
            }
        }
    });
    ui.window.add_controller(keyctl);
}

fn start_ipc(_state: Shared<App>, _ui: Rc<Ui>) {
    let path = socket_path();
    let _ = std::fs::remove_file(&path);
    let listener = match UnixListener::bind(&path) {
        Ok(l) => l,
        Err(e) => {
            eprintln!("copper: cannot bind {}: {e}", path.display());
            return;
        }
    };
    std::thread::spawn(move || {
        for stream in listener.incoming() {
            let stream = match stream {
                Ok(s) => s,
                Err(_) => break,
            };
            let resp = match stream.try_clone() {
                Ok(clone) => {
                    let mut line = String::new();
                    match BufReader::new(clone).read_line(&mut line) {
                        Ok(_) => match serde_json::from_str::<Request>(line.trim()) {
                            Ok(req) => {
                                // Hand the request to the GTK main thread. The
                                // handler reaches the state through DAEMON, so
                                // this closure only needs to be Send.
                                glib::idle_add_once(move || handle_request(req));
                                Response { ok: true, error: None }
                            }
                            Err(e) => Response {
                                ok: false,
                                error: Some(format!("bad request: {e}")),
                            },
                        },
                        Err(e) => Response {
                            ok: false,
                            error: Some(e.to_string()),
                        },
                    }
                }
                Err(e) => Response {
                    ok: false,
                    error: Some(e.to_string()),
                },
            };
            let mut w = stream;
            let _ = writeln!(w, "{}", serde_json::to_string(&resp).unwrap());
        }
    });
}

fn handle_request(req: Request) {
    DAEMON.with(|d| {
        let borrowed = d.borrow();
        let Some((state, ui)) = borrowed.as_ref() else {
            return;
        };
        match req {
            Request::Capture { ref text } | Request::Add { ref text } => {
                let is_capture = matches!(req, Request::Capture { .. });
                {
                    let mut st = state.borrow_mut();
                    st.store.add_note(text);
                    save(&st);
                }
                rebuild(state, ui);
                if is_capture {
                    show_toast(ui);
                }
            }
            Request::Toggle => {
                if ui.window.is_visible() {
                    ui.window.set_visible(false);
                } else {
                    ui.window.present();
                }
            }
        }
    });
}

fn save(st: &App) {
    if let Err(e) = st.store.save() {
        eprintln!("copper: failed to save store: {e}");
    }
}

fn show_toast(ui: &Rc<Ui>) {
    ui.toast.set_visible(true);
    let toast = ui.toast.clone();
    glib::timeout_add_local_once(Duration::from_millis(1500), move || {
        toast.set_visible(false);
    });
}

fn rebuild(state: &Shared<App>, ui: &Rc<Ui>) {
    let mut st = state.borrow_mut();
    st.rebuilding = true;
    while let Some(child) = ui.notes_box.first_child() {
        ui.notes_box.remove(&child);
    }
    ui.rows.borrow_mut().clear();

    let query = ui.search.text().to_lowercase();
    st.visible.clear();
    let store = st.store.clone();
    let selected = st.selected.clone();
    for list in &store.lists {
        let notes: Vec<&Note> = list
            .notes
            .iter()
            .filter(|n| query.is_empty() || n.text.to_lowercase().contains(&query))
            .collect();
        if notes.is_empty() {
            continue;
        }
        let header = gtk4::Label::new(Some(&list.name.to_uppercase()));
        header.add_css_class("list-header");
        header.set_xalign(0.0);
        let hrow = gtk4::ListBoxRow::new();
        hrow.set_selectable(false);
        hrow.set_activatable(false);
        hrow.set_child(Some(&header));
        ui.notes_box.append(&hrow);

        for note in notes {
            st.visible.push(note.id.clone());
            let row = make_card(state, ui, note.clone(), selected.contains(&note.id));
            ui.rows.borrow_mut().insert(note.id.clone(), row.clone());
            ui.notes_box.append(&row);
        }
    }

    ui.input.set_placeholder_text(Some(&format!(
        "Add a note or a prompt ({})",
        st.store.active_list
    )));
    rebuild_menu(&st, ui);
    st.rebuilding = false;
}

fn rebuild_menu(st: &App, ui: &Rc<Ui>) {
    while let Some(child) = ui.menu_main.first_child() {
        ui.menu_main.remove(&child);
    }
    let label = gtk4::Label::new(Some("LISTS"));
    label.add_css_class("section-label");
    label.set_xalign(0.0);
    ui.menu_main.append(&label);
    for list in &st.store.lists {
        let name = list.name.clone();
        let title = if name == st.store.active_list {
            format!("✓ {name}")
        } else {
            format!("   {name}")
        };
        let btn = gtk4::Button::with_label(&title);
        btn.connect_clicked({
            let popover = ui.menu_popover.clone();
            move |_| {
                set_active_list(&name);
                popover.popdown();
            }
        });
        ui.menu_main.append(&btn);
    }
    let sep = gtk4::Separator::new(Orientation::Horizontal);
    ui.menu_main.append(&sep);
    let new_btn = gtk4::Button::with_label("New list...");
    new_btn.connect_clicked({
        let stack = ui.menu_stack.clone();
        let entry = ui.new_list_entry.clone();
        move |_| {
            stack.set_visible_child(&entry);
            entry.grab_focus();
        }
    });
    ui.menu_main.append(&new_btn);
}

// The list-switcher buttons outlive the rebuild borrow, so they reach the
// daemon state through this thread-local, registered once in run().
thread_local! {
    static DAEMON: RefCell<Option<(Shared<App>, Rc<Ui>)>> = const { RefCell::new(None) };
}

fn set_active_list(name: &str) {
    DAEMON.with(|d| {
        if let Some((state, ui)) = d.borrow().as_ref() {
            {
                let mut st = state.borrow_mut();
                st.store.active_list = name.to_string();
                save(&st);
            }
            rebuild(state, ui);
        }
    });
}

fn make_card(
    state: &Shared<App>,
    ui: &Rc<Ui>,
    note: Note,
    is_selected: bool,
) -> gtk4::ListBoxRow {
    let row = gtk4::ListBoxRow::new();
    row.set_selectable(false);
    row.set_activatable(false);
    row.set_focusable(false);

    let h = gtk4::Box::new(Orientation::Horizontal, 8);
    h.add_css_class("card");
    if is_selected {
        h.add_css_class("selected");
    }

    let check = gtk4::CheckButton::new();
    check.add_css_class("circle-check");
    check.set_active(note.done);
    check.set_valign(gtk4::Align::Start);

    let label = gtk4::Label::new(Some(&note.text));
    label.add_css_class("note-text");
    if note.done {
        label.add_css_class("done");
    }
    label.set_xalign(0.0);
    label.set_wrap(true);
    label.set_wrap_mode(gtk4::pango::WrapMode::WordChar);
    label.set_ellipsize(gtk4::pango::EllipsizeMode::End);
    label.set_lines(4);
    label.set_hexpand(true);

    h.append(&check);
    h.append(&label);
    row.set_child(Some(&h));

    let id = note.id.clone();
    check.connect_toggled({
        let state = state.clone();
        let ui = ui.clone();
        let id = id.clone();
        move |c| {
            let done = c.is_active();
            {
                let mut st = state.borrow_mut();
                if st.rebuilding {
                    return;
                }
                if let Some(n) = st.store.find_note_mut(&id) {
                    n.done = done;
                }
                save(&st);
            }
            rebuild(&state, &ui);
        }
    });

    // Left click: toggle selection.
    let click = gtk4::GestureClick::new();
    click.set_button(1);
    click.connect_released({
        let state = state.clone();
        let ui = ui.clone();
        let id = id.clone();
        move |_, _, _, _| {
            {
                let mut st = state.borrow_mut();
                if !st.selected.remove(&id) {
                    st.selected.insert(id.clone());
                }
                st.cursor = Some(id.clone());
            }
            rebuild(&state, &ui);
        }
    });
    row.add_controller(click);

    // Right click: context menu.
    let right = gtk4::GestureClick::new();
    right.set_button(3);
    right.connect_pressed({
        let state = state.clone();
        let ui = ui.clone();
        let id = id.clone();
        move |_, _, _, _| {
            {
                let mut st = state.borrow_mut();
                if !st.selected.contains(&id) {
                    st.selected.clear();
                    st.selected.insert(id.clone());
                }
                st.cursor = Some(id.clone());
            }
            rebuild(&state, &ui);
            let anchor = ui.rows.borrow().get(&id).cloned();
            if let Some(anchor) = anchor {
                open_context_menu(&state, &ui, anchor.upcast_ref());
            }
        }
    });
    row.add_controller(right);

    row
}

fn menu_item(
    label: &str,
    shortcut: &str,
    popover: &gtk4::Popover,
    action: impl Fn() + 'static,
) -> gtk4::Button {
    let btn = gtk4::Button::new();
    let h = gtk4::Box::new(Orientation::Horizontal, 8);
    let l = gtk4::Label::new(Some(label));
    l.set_xalign(0.0);
    l.set_hexpand(true);
    let s = gtk4::Label::new(Some(shortcut));
    s.add_css_class("shortcut");
    h.append(&l);
    h.append(&s);
    btn.set_child(Some(&h));
    let pop = popover.clone();
    btn.connect_clicked(move |_| {
        action();
        pop.popdown();
    });
    btn
}

fn open_context_menu(state: &Shared<App>, ui: &Rc<Ui>, anchor: &gtk4::Widget) {
    let pop = gtk4::Popover::new();
    pop.add_css_class("context-menu");
    pop.set_parent(anchor);
    pop.set_has_arrow(false);
    pop.set_pointing_to(Some(&gdk::Rectangle::new(40, 8, 1, 1)));

    let v = gtk4::Box::new(Orientation::Vertical, 0);
    v.append(&menu_item("Copy", "Ctrl+C", &pop, {
        let state = state.clone();
        let ui = ui.clone();
        move || copy_selected(&state, &ui, false)
    }));
    v.append(&menu_item("Copy as List", "Ctrl+Shift+C", &pop, {
        let state = state.clone();
        let ui = ui.clone();
        move || copy_selected(&state, &ui, true)
    }));
    v.append(&menu_item("Mark as Done", "Space", &pop, {
        let state = state.clone();
        let ui = ui.clone();
        move || mark_done(&state, &ui)
    }));
    v.append(&menu_item("Edit", "Enter", &pop, {
        let state = state.clone();
        let ui = ui.clone();
        move || edit_cursor(&state, &ui)
    }));
    v.append(&menu_item("Merge Notes", "Ctrl+Shift+M", &pop, {
        let state = state.clone();
        let ui = ui.clone();
        move || merge_selected(&state, &ui)
    }));

    v.append(&gtk4::Separator::new(Orientation::Horizontal));
    let move_label = gtk4::Label::new(Some("MOVE TO"));
    move_label.add_css_class("section-label");
    move_label.set_xalign(0.0);
    v.append(&move_label);
    let lists: Vec<String> = state
        .borrow()
        .store
        .lists
        .iter()
        .map(|l| l.name.clone())
        .collect();
    for name in lists {
        let display = name.clone();
        v.append(&menu_item(&display, "", &pop, {
            let state = state.clone();
            let ui = ui.clone();
            move || move_selected_to(&state, &ui, &name)
        }));
    }

    v.append(&gtk4::Separator::new(Orientation::Horizontal));
    v.append(&menu_item("Delete", "Del", &pop, {
        let state = state.clone();
        let ui = ui.clone();
        move || delete_selected(&state, &ui)
    }));

    pop.set_child(Some(&v));
    pop.connect_closed(|p| p.unparent());
    pop.popup();
}

fn selected_texts(st: &App) -> Vec<String> {
    let mut out = Vec::new();
    for list in &st.store.lists {
        for note in &list.notes {
            if st.selected.contains(&note.id) {
                out.push(note.text.clone());
            }
        }
    }
    out
}

fn copy_selected(state: &Shared<App>, ui: &Rc<Ui>, as_list: bool) {
    let texts = selected_texts(&state.borrow());
    if texts.is_empty() {
        return;
    }
    let text = if as_list {
        texts
            .iter()
            .enumerate()
            .map(|(i, t)| format!("{}. {}", i + 1, t))
            .collect::<Vec<_>>()
            .join("\n")
    } else {
        texts.join("\n")
    };
    ui.window.clipboard().set_text(&text);
}

fn mark_done(state: &Shared<App>, ui: &Rc<Ui>) {
    {
        let mut st = state.borrow_mut();
        let ids = st.selected.clone();
        for id in &ids {
            if let Some(n) = st.store.find_note_mut(id) {
                n.done = true;
            }
        }
        save(&st);
    }
    rebuild(state, ui);
}

fn toggle_done(state: &Shared<App>, ui: &Rc<Ui>) {
    {
        let mut st = state.borrow_mut();
        let ids: Vec<String> = if st.selected.is_empty() {
            st.cursor.clone().into_iter().collect()
        } else {
            st.selected.iter().cloned().collect()
        };
        for id in &ids {
            if let Some(n) = st.store.find_note_mut(id) {
                n.done = !n.done;
            }
        }
        save(&st);
    }
    rebuild(state, ui);
}

fn delete_selected(state: &Shared<App>, ui: &Rc<Ui>) {
    {
        let mut st = state.borrow_mut();
        let ids = st.selected.clone();
        if ids.is_empty() {
            return;
        }
        for list in &mut st.store.lists {
            list.notes.retain(|n| !ids.contains(&n.id));
        }
        st.selected.clear();
        st.cursor = None;
        save(&st);
    }
    rebuild(state, ui);
}

fn merge_selected(state: &Shared<App>, ui: &Rc<Ui>) {
    {
        let mut st = state.borrow_mut();
        if st.selected.len() < 2 {
            return;
        }
        let positions = st.store.positions_of(&st.selected);
        if positions.is_empty() {
            return;
        }
        let (first_list, _) = positions[0];
        let mut texts: Vec<String> = Vec::new();
        for (li, ni) in &positions {
            texts.push(st.store.lists[*li].notes[*ni].text.clone());
        }
        let merged = Note {
            id: uuid::Uuid::new_v4().to_string(),
            text: texts.join("\n\n"),
            done: false,
            created_at: chrono::Local::now().to_rfc3339(),
        };
        let ids = st.selected.clone();
        for list in &mut st.store.lists {
            list.notes.retain(|n| !ids.contains(&n.id));
        }
        st.store.lists[first_list].notes.push(merged.clone());
        st.selected.clear();
        st.selected.insert(merged.id.clone());
        st.cursor = Some(merged.id);
        save(&st);
    }
    rebuild(state, ui);
}

fn move_selected_to(state: &Shared<App>, ui: &Rc<Ui>, list_name: &str) {
    {
        let mut st = state.borrow_mut();
        let ids = st.selected.clone();
        if ids.is_empty() {
            return;
        }
        st.store.ensure_list(list_name);
        let mut moved: Vec<Note> = Vec::new();
        for list in &mut st.store.lists {
            let mut keep = Vec::new();
            for note in std::mem::take(&mut list.notes) {
                if ids.contains(&note.id) {
                    moved.push(note);
                } else {
                    keep.push(note);
                }
            }
            list.notes = keep;
        }
        if let Some(target) = st.store.list_mut(list_name) {
            target.notes.extend(moved);
        }
        save(&st);
    }
    rebuild(state, ui);
}

fn move_cursor(state: &Shared<App>, ui: &Rc<Ui>, delta: i32) {
    {
        let mut st = state.borrow_mut();
        if st.visible.is_empty() {
            return;
        }
        let idx = st
            .cursor
            .as_ref()
            .and_then(|c| st.visible.iter().position(|v| v == c))
            .map(|i| i as i32 + delta)
            .unwrap_or(if delta > 0 { 0 } else { st.visible.len() as i32 - 1 })
            .clamp(0, st.visible.len() as i32 - 1) as usize;
        let id = st.visible[idx].clone();
        st.cursor = Some(id.clone());
        st.selected.clear();
        st.selected.insert(id);
    }
    rebuild(state, ui);
}

fn edit_cursor(state: &Shared<App>, ui: &Rc<Ui>) {
    let (id, text) = {
        let st = state.borrow();
        let id = match st.cursor.clone().or_else(|| st.selected.iter().next().cloned()) {
            Some(id) => id,
            None => return,
        };
        let text = st
            .store
            .lists
            .iter()
            .flat_map(|l| l.notes.iter())
            .find(|n| n.id == id)
            .map(|n| n.text.clone());
        match text {
            Some(t) => (id, t),
            None => return,
        }
    };
    let anchor = match ui.rows.borrow().get(&id).cloned() {
        Some(r) => r,
        None => return,
    };

    let pop = gtk4::Popover::new();
    pop.set_parent(&anchor);
    pop.set_has_arrow(false);
    let entry = gtk4::Entry::new();
    entry.set_text(&text);
    entry.set_width_request(300);
    pop.set_child(Some(&entry));
    entry.connect_activate({
        let state = state.clone();
        let ui = ui.clone();
        let pop = pop.clone();
        move |entry| {
            let new_text = entry.text().trim().to_string();
            if !new_text.is_empty() {
                {
                    let mut st = state.borrow_mut();
                    if let Some(n) = st.store.find_note_mut(&id) {
                        n.text = new_text;
                    }
                    save(&st);
                }
                rebuild(&state, &ui);
            }
            pop.popdown();
        }
    });
    pop.connect_closed(|p| p.unparent());
    pop.popup();
    entry.grab_focus();
}
