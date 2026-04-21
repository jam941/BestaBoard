package mode

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
)

type DemoNote struct {
	Text     string
	Duration time.Duration
}

type DemoNoteMode struct {
	notes        []DemoNote
	mu           sync.Mutex
	index        int
	lastDuration time.Duration
}

func NewDemoNoteMode(notes []DemoNote) *DemoNoteMode {
	d := &DemoNoteMode{notes: notes}
	if len(notes) > 0 {
		d.lastDuration = notes[0].Duration
	}
	return d
}

func (m *DemoNoteMode) ID() string { return "notes" }

func (m *DemoNoteMode) Render(_ context.Context) (vestaboard.BoardLayout, error) {
	m.mu.Lock()
	if len(m.notes) == 0 {
		m.mu.Unlock()
		return nil, ErrNoContent
	}
	note := m.notes[m.index]
	m.lastDuration = note.Duration
	m.index = (m.index + 1) % len(m.notes)
	m.mu.Unlock()

	text := strings.ToUpper(note.Text)
	rows := wrapWords(text, 15, 3)
	layout := BlankLayout()
	for i, row := range rows {
		if i >= 3 {
			break
		}
		layout[i] = CenterRow(row, 15)
	}
	return layout, nil
}

func (m *DemoNoteMode) Duration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastDuration
}
