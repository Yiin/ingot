package fsstore

import (
	"strings"

	"github.com/Yiin/ingot/internal/store"
)

// Search is a minimal placeholder: case-insensitive, whitespace-split,
// AND-substring matching against the raw body, with no diacritic
// folding, Markdown stripping, or match-offset ranges. The full
// normalized matcher — internal/store/searchtext, byte-offset Ranges,
// section-header jump behavior — is a separate piece of work; this
// keeps Store satisfiable in the meantime and gives ScopeAll/
// ScopeActiveProject their documented meaning.
func (s *fileStore) Search(query string, scope store.Scope) ([]store.Hit, error) {
	tokens := strings.Fields(strings.ToLower(query))
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
				if matchesAllTokens(strings.ToLower(n.Body), tokens) {
					hits = append(hits, store.Hit{Project: pi, Section: si, Note: ni, Index: flatIndex})
				}
				flatIndex++
			}
		}
	}
	return hits, nil
}

func matchesAllTokens(body string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(body, t) {
			return false
		}
	}
	return true
}
