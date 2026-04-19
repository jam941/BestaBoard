package mode

import (
	"context"
	"log/slog"
	"strings"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
	"github.com/jam941/bestaboard/internal/store"
)

type NoteMode struct {
	store *store.Store
}

func NewNoteMode(s *store.Store) *NoteMode {
	return &NoteMode{store: s}
}

func (m *NoteMode) ID() string { return "notes" }

func (m *NoteMode) Render(_ context.Context) (vestaboard.BoardLayout, error) {
	note, err := m.store.ActiveNote()
	if err != nil {
		slog.Error("notes: db error", "error", err)
		return nil, ErrNoContent
	}
	if note == nil {
		return nil, ErrNoContent
	}

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
