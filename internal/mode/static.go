package mode

import (
	"context"
	"strings"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
)

type StaticMode struct {
	getText func() string
}

func NewStaticMode(getText func() string) *StaticMode {
	return &StaticMode{getText: getText}
}

func (m *StaticMode) ID() string { return "static" }

func (m *StaticMode) Render(_ context.Context) (vestaboard.BoardLayout, error) {
	text := m.getText()
	if strings.TrimSpace(text) == "" {
		return nil, ErrNoContent
	}

	layout := BlankLayout()
	rows := wrapWords(strings.ToUpper(text), 15, 3)
	for i, row := range rows {
		if i >= 3 {
			break
		}
		layout[i] = CenterRow(row, 15)
	}
	return layout, nil
}

func wrapWords(text string, colWidth, maxRows int) []string {
	words := strings.Fields(text)
	var lines []string
	current := ""

	for _, w := range words {
		if len(lines) >= maxRows {
			break
		}
		if current == "" {
			current = w
		} else if len(current)+1+len(w) <= colWidth {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" && len(lines) < maxRows {
		lines = append(lines, current)
	}
	return lines
}
