package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	tabWidth = 8
	esc      = 0x1b
	bel      = 0x07
)

// sanitize makes captured command output safe to embed in the frame. It keeps
// newlines, expands tabs to spaces and preserves SGR color sequences
// (ESC [ ... m); every other escape sequence and all C0/C1 control characters
// (including \r and BEL) are dropped so output cannot move the cursor, set
// the clipboard (OSC 52) or otherwise corrupt the UI.
func sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	col := 0
	for i := 0; i < len(s); {
		if s[i] == esc {
			end, keep := scanEscape(s, i)
			if keep {
				b.WriteString(s[i:end])
			}
			i = end
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		col = writeSafeRune(&b, r, col)
	}
	return b.String()
}

// writeSafeRune writes r unless it is a control character and returns the
// new column.
func writeSafeRune(b *strings.Builder, r rune, col int) int {
	switch {
	case r == '\n':
		b.WriteByte('\n')
		return 0
	case r == '\t':
		spaces := tabWidth - col%tabWidth
		b.WriteString(strings.Repeat(" ", spaces))
		return col + spaces
	case unicode.IsControl(r):
		return col
	}
	b.WriteRune(r)
	return col + 1
}

// scanEscape returns the index just past the escape sequence starting at
// s[start] and whether it is an SGR sequence worth keeping.
func scanEscape(s string, start int) (int, bool) {
	next := start + 1
	if next >= len(s) {
		return next, false
	}
	switch s[next] {
	case '[':
		return scanCSI(s, next+1)
	case ']', 'P', 'X', '^', '_':
		return scanString(s, next+1), false
	}
	if s[next] >= 0x20 && s[next] <= 0x7e {
		return next + 1, false
	}
	return next, false
}

// scanCSI consumes parameter, intermediate and final bytes of a CSI sequence.
func scanCSI(s string, i int) (int, bool) {
	sgrParams := true
	for ; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 0x30 && c <= 0x3f:
			sgrParams = sgrParams && (c <= '9' || c == ';' || c == ':')
		case c >= 0x20 && c <= 0x2f:
			sgrParams = false
		case c >= 0x40 && c <= 0x7e:
			return i + 1, sgrParams && c == 'm'
		default:
			return i, false // malformed: drop what we consumed
		}
	}
	return i, false
}

// scanString consumes an OSC/DCS/SOS/PM/APC payload up to BEL or ST.
func scanString(s string, i int) int {
	for ; i < len(s); i++ {
		if s[i] == bel {
			return i + 1
		}
		if s[i] == esc && i+1 < len(s) && s[i+1] == '\\' {
			return i + 2
		}
	}
	return i
}
