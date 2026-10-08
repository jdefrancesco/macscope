package tui

import (
	"bytes"
	"sync"
	"unicode/utf8"
)

// maxOutputBytes caps how much of each stream a run keeps in memory.
const maxOutputBytes = 2 << 20 // 2 MiB

// limitedBuffer keeps the first limit bytes written and counts the rest. It
// never returns an error so commands are not interrupted by the cap.
type limitedBuffer struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	limit int
	total int64
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.total += int64(len(p))
	if room := b.limit - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

// captured is a snapshot of one stream.
type captured struct {
	text  string
	total int64
}

func (c captured) truncated() bool {
	return c.total > int64(len(c.text))
}

// snapshot returns the kept text trimmed to a rune boundary.
func (b *limitedBuffer) snapshot() captured {
	b.mu.Lock()
	defer b.mu.Unlock()
	data := b.buf.Bytes()
	if b.total > int64(len(data)) {
		data = trimPartialRune(data)
	}
	return captured{text: string(data), total: b.total}
}

// trimPartialRune drops an incomplete UTF-8 sequence cut off by the cap.
func trimPartialRune(data []byte) []byte {
	for i := len(data) - 1; i >= 0 && i >= len(data)-utf8.UTFMax; i-- {
		if utf8.RuneStart(data[i]) {
			if utf8.FullRune(data[i:]) {
				return data
			}
			return data[:i]
		}
	}
	return data
}
