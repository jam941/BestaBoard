package mode

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
)

type ClockMode struct {
	getTimezone func() string
}

func NewClockMode(getTimezone func() string) *ClockMode {
	return &ClockMode{getTimezone: getTimezone}
}

func (m *ClockMode) ID() string { return "clock" }

func (m *ClockMode) Render(_ context.Context) (vestaboard.BoardLayout, error) {
	loc := time.UTC
	if tz := m.getTimezone(); tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			slog.Warn("clock: unknown timezone, falling back to UTC", "timezone", tz)
		} else {
			loc = l
		}
	}

	now := time.Now().In(loc)

	day := strings.ToUpper(now.Weekday().String()[:3])
	month := strings.ToUpper(now.Month().String()[:3])
	dateLine := fmt.Sprintf("%s %s %d", day, month, now.Day())

	hour := now.Hour()
	ampm := "AM"
	if hour >= 12 {
		ampm = "PM"
	}
	if hour == 0 {
		hour = 12
	} else if hour > 12 {
		hour -= 12
	}
	timeLine := fmt.Sprintf("%d:%02d %s", hour, now.Minute(), ampm)

	layout := BlankLayout()
	layout[0] = CenterRow(dateLine, 15)
	layout[1] = CenterRow(timeLine, 15)

	return layout, nil
}
