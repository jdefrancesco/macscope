package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Streams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

func (s Streams) WithDefaults() Streams {
	if s.In == nil {
		s.In = os.Stdin
	}
	if s.Out == nil {
		s.Out = os.Stdout
	}
	if s.Err == nil {
		s.Err = os.Stderr
	}
	return s
}

type TextWriter struct {
	w io.Writer
}

func NewTextWriter(w io.Writer) TextWriter {
	return TextWriter{w: w}
}

func (tw TextWriter) Section(title string) error {
	_, err := fmt.Fprintf(tw.w, "%s:\n", title)
	return err
}

func (tw TextWriter) KeyValue(key, value string) error {
	_, err := fmt.Fprintf(tw.w, "  %-24s %s\n", key+":", value)
	return err
}

func (tw TextWriter) Bullet(value string) error {
	_, err := fmt.Fprintf(tw.w, "  - %s\n", value)
	return err
}

func WriteJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
