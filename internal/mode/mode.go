package mode

import (
	"context"
	"errors"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
)


var ErrNoContent = errors.New("no content")

type Mode interface {
	ID() string
	Render(ctx context.Context) (vestaboard.BoardLayout, error)
}

func CharFor(r rune) int {
	switch {
	case r >= 'A' && r <= 'Z':
		return int(r-'A') + vestaboard.CharA
	case r >= 'a' && r <= 'z':
		return int(r-'a') + vestaboard.CharA
	case r >= '0' && r <= '9':
		return int(r-'0') + vestaboard.Char0
	case r == ' ':
		return vestaboard.CharSpace
	case r == '!':
		return vestaboard.CharExclamation
	case r == '@':
		return vestaboard.CharAt
	case r == '#':
		return vestaboard.CharHash
	case r == '$':
		return vestaboard.CharDollar
	case r == '(':
		return vestaboard.CharLeftParen
	case r == ')':
		return vestaboard.CharRightParen
	case r == '-':
		return vestaboard.CharHyphen
	case r == '+':
		return vestaboard.CharPlus
	case r == '&':
		return vestaboard.CharAnd
	case r == '=':
		return vestaboard.CharEquals
	case r == ';':
		return vestaboard.CharSemicolon
	case r == ':':
		return vestaboard.CharColon
	case r == '\'':
		return vestaboard.CharApostrophe
	case r == '"':
		return vestaboard.CharQuote
	case r == '%':
		return vestaboard.CharPercent
	case r == ',':
		return vestaboard.CharComma
	case r == '.':
		return vestaboard.CharPeriod
	case r == '/':
		return vestaboard.CharSlash
	case r == '?':
		return vestaboard.CharQuestion
	case r == '°':
		return vestaboard.CharDegree
	default:
		return vestaboard.CharSpace
	}
}

func StringToRow(s string, width int) []int {
	row := make([]int, width)
	for i, r := range []rune(s) {
		if i >= width {
			break
		}
		row[i] = CharFor(r)
	}
	return row
}

func CenterRow(s string, width int) []int {
	runes := []rune(s)
	if len(runes) > width {
		runes = runes[:width]
	}
	padding := (width - len(runes)) / 2
	padded := make([]rune, width)
	for i := range padded {
		padded[i] = ' '
	}
	for i, r := range runes {
		padded[padding+i] = r
	}
	return StringToRow(string(padded), width)
}

func BlankRow(width int) []int {
	return make([]int, width)
}

func BlankLayout() vestaboard.BoardLayout {
	layout := make(vestaboard.BoardLayout, 3)
	for i := range layout {
		layout[i] = BlankRow(15)
	}
	return layout
}
