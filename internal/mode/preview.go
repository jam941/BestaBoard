package mode

import (
	"context"
	"strings"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
)

// RenderText renders the mode and returns the layout as a human-readable
// string. Useful in tests and for a future /preview endpoint without needing
// a live board connection.
func RenderText(ctx context.Context, m Mode) (string, error) {
	layout, err := m.Render(ctx)
	if err != nil {
		return "", err
	}
	return LayoutToText(layout), nil
}

// LayoutToText converts a BoardLayout to a bracketed multi-line string,
// one row per line. Unknown character codes are represented as "·".
//
// Example output:
//
//	[   S A T   A P R   1 9   ]
//	[      1 0 : 3 0   A M   ]
//	[                         ]
func LayoutToText(layout vestaboard.BoardLayout) string {
	var sb strings.Builder
	for r, row := range layout {
		if r > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteByte('[')
		for c, code := range row {
			if c > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(glyphFor(code))
		}
		sb.WriteByte(']')
	}
	return sb.String()
}

func glyphFor(code int) string {
	switch {
	case code >= vestaboard.CharA && code <= vestaboard.CharA+25:
		return string(rune('A' + code - vestaboard.CharA))
	case code == vestaboard.Char0:
		return "0"
	case code >= vestaboard.Char1 && code <= vestaboard.Char1+8:
		return string(rune('1' + code - vestaboard.Char1))
	case code == vestaboard.CharSpace:
		return " "
	case code == vestaboard.CharColon:
		return ":"
	case code == vestaboard.CharHyphen:
		return "-"
	case code == vestaboard.CharPeriod:
		return "."
	case code == vestaboard.CharComma:
		return ","
	case code == vestaboard.CharExclamation:
		return "!"
	case code == vestaboard.CharQuestion:
		return "?"
	case code == vestaboard.CharPercent:
		return "%"
	case code == vestaboard.CharAnd:
		return "&"
	case code == vestaboard.CharPlus:
		return "+"
	case code == vestaboard.CharAt:
		return "@"
	default:
		return "·"
	}
}
