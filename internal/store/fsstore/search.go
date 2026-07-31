package fsstore

import (
	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/searchtext"
)

// normCacheEntry pairs a note's cached searchtext.Normalized form with
// the exact body it was computed from, so a lookup can tell a still-
// valid cache entry from a stale one with a plain string comparison
// instead of a body hash.
type normCacheEntry struct {
	body string
	norm searchtext.Normalized
}

// Search finds every note whose body matches every whitespace-split
// token of query — case-insensitive, diacritic-insensitive, and
// Markdown-syntax-insensitive AND-substring matching, via
// internal/store/searchtext. scope limits the search to Active()'s
// project (ScopeActiveProject) or every project (ScopeAll); no v1 UI
// surfaces ScopeAll, but it is implemented and tested.
//
// Each note's normalized form is computed once (a Markdown parse) and
// cached keyed by NoteID, invalidated the moment its body no longer
// matches what the cache was computed from — the common case, a search
// keystroke with no notes edited in between, costs one string
// comparison per note plus the substring scan itself, not a re-parse.
func (s *fileStore) Search(query string, scope store.Scope) ([]store.Hit, error) {
	tokens := searchtext.Tokens(query)
	if len(tokens) == 0 {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var hits []store.Hit
	flatIndex := 0
	for pi, pid := range s.order {
		if scope == store.ScopeActiveProject && pid != s.active {
			continue
		}
		pe := s.projects[pid]
		for si, sec := range pe.proj.Sections {
			for ni, n := range sec.Notes {
				norm := s.normalizedForLocked(n.ID, n.Body)
				if matched, ranges := norm.Match(tokens); matched {
					hits = append(hits, store.Hit{
						Project: pi,
						Section: si,
						Note:    ni,
						Index:   flatIndex,
						Ranges:  ranges,
					})
				}
				flatIndex++
			}
		}
	}
	return hits, nil
}

// normalizedForLocked returns id's cached normalized body, recomputing
// and re-caching it when body no longer matches the cache entry. Must
// be called with s.mu held.
func (s *fileStore) normalizedForLocked(id store.NoteID, body string) searchtext.Normalized {
	if e, ok := s.searchCache[id]; ok && e.body == body {
		return e.norm
	}
	norm := searchtext.Normalize(body)
	if s.searchCache == nil {
		s.searchCache = make(map[store.NoteID]normCacheEntry)
	}
	s.searchCache[id] = normCacheEntry{body: body, norm: norm}
	return norm
}

// evictSearchCacheLocked drops cached normalized forms for notes that no
// longer exist in the live model. Without this, searchCache would grow
// with every note ever seen in the process's lifetime rather than every
// note currently live — every note-removal path (DeleteNotes, ClearDone,
// MergeNotes' consumed inputs, MoveNotes' and DeleteProject's departing
// notes) funnels its removed ids here. Must be called with s.mu held.
func (s *fileStore) evictSearchCacheLocked(ids ...store.NoteID) {
	for _, id := range ids {
		delete(s.searchCache, id)
	}
}
