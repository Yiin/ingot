package mdpango

import (
	"sort"
	"strings"
)

// writeHighlighted escapes and writes source[start:stop) into b, wrapping
// whatever part of it falls inside one of ranges (raw-body [start, end)
// byte pairs, sorted and non-overlapping — see mergeRanges) in a
// background span. ranges may be nil, in which case this is exactly
// escapeText(source[start:stop]).
func writeHighlighted(b *strings.Builder, source []byte, start, stop int, ranges [][2]int) {
	if stop <= start {
		return
	}
	pos := start
	for _, r := range ranges {
		rs, re := r[0], r[1]
		if re <= pos {
			continue
		}
		if rs >= stop {
			break
		}
		clipStart, clipEnd := rs, re
		if clipStart < pos {
			clipStart = pos
		}
		if clipEnd > stop {
			clipEnd = stop
		}
		if clipStart > pos {
			b.WriteString(escapeText(string(source[pos:clipStart])))
		}
		b.WriteString(`<span background="` + HighlightBackground() + `">`)
		b.WriteString(escapeText(string(source[clipStart:clipEnd])))
		b.WriteString("</span>")
		pos = clipEnd
	}
	if pos < stop {
		b.WriteString(escapeText(string(source[pos:stop])))
	}
}

// mergeRanges sorts ranges by start and merges any that overlap or touch,
// so writeHighlighted's single left-to-right scan can assume disjoint,
// ordered input. A degenerate (empty or inverted) range is dropped.
func mergeRanges(ranges [][2]int) [][2]int {
	if len(ranges) == 0 {
		return nil
	}
	cp := make([][2]int, 0, len(ranges))
	for _, r := range ranges {
		if r[1] > r[0] {
			cp = append(cp, r)
		}
	}
	if len(cp) == 0 {
		return nil
	}
	sort.Slice(cp, func(i, j int) bool {
		if cp[i][0] != cp[j][0] {
			return cp[i][0] < cp[j][0]
		}
		return cp[i][1] < cp[j][1]
	})

	merged := cp[:1]
	for _, r := range cp[1:] {
		last := &merged[len(merged)-1]
		if r[0] <= last[1] {
			if r[1] > last[1] {
				last[1] = r[1]
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}
