package tui

import (
	"io"
	"testing"
)

func TestLineInput(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want string
	}{
		{"typing", []string{"a", "b"}, "ab"},
		{"space", []string{"a", "space", "b"}, "a b"},
		{"backspace", []string{"a", "b", "backspace"}, "a"},
		{"backspace on empty", []string{"backspace"}, ""},
		{"ctrl+u clears", []string{"a", "b", "ctrl+u"}, ""},
		{"ctrl+w deletes word", []string{"a", "space", "b", "c", "space", "ctrl+w"}, "a "},
		{"ignored key", []string{"a", "up"}, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := lineInput{}
			for _, k := range tt.keys {
				in = in.Update(key(k))
			}
			if in.Value() != tt.want {
				t.Fatalf("Value() = %q, want %q", in.Value(), tt.want)
			}
			if in.View() != tt.want+inputCursor {
				t.Fatalf("View() = %q", in.View())
			}
		})
	}
}

func TestLineInputIsImmutable(t *testing.T) {
	base := lineInput{}.Update(key("a"))
	_ = base.Update(key("b"))
	_ = base.Update(key("backspace"))
	if base.Value() != "a" {
		t.Fatalf("base mutated to %q", base.Value())
	}
}

// newBlockingReader returns a reader that never yields data until closed.
func newBlockingReader() (io.Reader, io.Closer) {
	r, w := io.Pipe()
	return r, w
}
