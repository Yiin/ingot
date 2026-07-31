package menus

// Project and Section are the minimal read models the context, overflow,
// and Move to menus need. menus never imports internal/store — the
// caller converts internal/store's richer types into these before
// building a menu — keeping internal/ui decoupled from storage.
type Project struct {
	ID    string
	Title string
}

type Section struct {
	ID    string
	Title string
}
