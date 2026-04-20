package mode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
)

type WeatherConfig struct {
	Latitude  float64
	Longitude float64
	Timezone  string
	// Units is "fahrenheit" or "celsius".
	Units string
}

type WeatherMode struct {
	getConfig func() WeatherConfig
	mu        sync.Mutex
	cached    *weatherData
	lastCfg   WeatherConfig
	cacheTTL  time.Duration
	client    *http.Client
}

type weatherData struct {
	today     dailyForecast
	tomorrow  dailyForecast
	fetchedAt time.Time
}

type dailyForecast struct {
	high       int
	low        int
	precipProb int    // 0-100 percent
	condition  string // short label derived from WMO weather code
}

func NewWeatherMode(getConfig func() WeatherConfig) *WeatherMode {
	return &WeatherMode{
		getConfig: getConfig,
		cacheTTL:  10 * time.Minute,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *WeatherMode) ID() string { return "weather" }

func (m *WeatherMode) Render(ctx context.Context) (vestaboard.BoardLayout, error) {
	cfg := m.getConfig()

	data, err := m.getData(ctx, cfg)
	if err != nil {
		slog.Error("weather: fetch failed, skipping", "error", err)
		return nil, ErrNoContent
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		slog.Warn("weather: unknown timezone, falling back to UTC", "timezone", cfg.Timezone)
		loc = time.UTC
	}
	now := time.Now().In(loc)

	var label string
	var fc dailyForecast
	if now.Hour() < 12 {
		label = "TODAY"
		fc = data.today
	} else {
		label = "TOMORROW"
		fc = data.tomorrow
	}

	unitSuffix := "F"
	if cfg.Units == "celsius" {
		unitSuffix = "C"
	}

	hiLo := fmt.Sprintf("HI %d  LO %d%s", fc.high, fc.low, unitSuffix)
	precip := fmt.Sprintf("%s %d%%", fc.condition, fc.precipProb)

	layout := BlankLayout()
	layout[0] = CenterRow(label, 15)
	layout[1] = CenterRow(hiLo, 15)
	layout[2] = CenterRow(precip, 15)
	return layout, nil
}

// Open-Meteo JSON response shape (only the fields we care about).
type openMeteoResponse struct {
	Daily struct {
		TemperatureMax []float64 `json:"temperature_2m_max"`
		TemperatureMin []float64 `json:"temperature_2m_min"`
		PrecipProbMax  []int     `json:"precipitation_probability_max"`
		WeatherCode    []int     `json:"weather_code"`
	} `json:"daily"`
}

func (m *WeatherMode) getData(ctx context.Context, cfg WeatherConfig) (*weatherData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Bust cache if config changed.
	if m.cached != nil && cfg == m.lastCfg && time.Since(m.cached.fetchedAt) < m.cacheTTL {
		return m.cached, nil
	}
	m.lastCfg = cfg

	timezone := cfg.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	units := cfg.Units
	if units == "" {
		units = "fahrenheit"
	}

	params := url.Values{
		"latitude":         {fmt.Sprintf("%g", cfg.Latitude)},
		"longitude":        {fmt.Sprintf("%g", cfg.Longitude)},
		"daily":            {"temperature_2m_max,temperature_2m_min,precipitation_probability_max,weather_code"},
		"temperature_unit": {units},
		"timezone":         {timezone},
		"forecast_days":    {"2"},
	}
	apiURL := "https://api.open-meteo.com/v1/forecast?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo status %d", resp.StatusCode)
	}

	var omr openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&omr); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if len(omr.Daily.TemperatureMax) < 2 || len(omr.Daily.TemperatureMin) < 2 {
		return nil, fmt.Errorf("open-meteo: expected 2 days, got %d", len(omr.Daily.TemperatureMax))
	}

	precipToday, precipTmr := 0, 0
	if len(omr.Daily.PrecipProbMax) >= 2 {
		precipToday = omr.Daily.PrecipProbMax[0]
		precipTmr = omr.Daily.PrecipProbMax[1]
	}
	codeToday, codeTmr := 0, 0
	if len(omr.Daily.WeatherCode) >= 2 {
		codeToday = omr.Daily.WeatherCode[0]
		codeTmr = omr.Daily.WeatherCode[1]
	}

	data := &weatherData{
		today: dailyForecast{
			high:       int(math.Round(omr.Daily.TemperatureMax[0])),
			low:        int(math.Round(omr.Daily.TemperatureMin[0])),
			precipProb: precipToday,
			condition:  wmoCondition(codeToday),
		},
		tomorrow: dailyForecast{
			high:       int(math.Round(omr.Daily.TemperatureMax[1])),
			low:        int(math.Round(omr.Daily.TemperatureMin[1])),
			precipProb: precipTmr,
			condition:  wmoCondition(codeTmr),
		},
		fetchedAt: time.Now(),
	}
	m.cached = data
	slog.Info("weather: fetched forecast",
		"today_hi", data.today.high, "today_lo", data.today.low,
		"today_cond", data.today.condition, "today_precip", data.today.precipProb,
		"tomorrow_hi", data.tomorrow.high, "tomorrow_lo", data.tomorrow.low,
		"tomorrow_cond", data.tomorrow.condition, "tomorrow_precip", data.tomorrow.precipProb,
	)
	return data, nil
}

// wmoCondition maps a WMO weather interpretation code to a short board label.
// Reference: https://open-meteo.com/en/docs#weathervariables
func wmoCondition(code int) string {
	switch {
	case code == 0:
		return "CLEAR"
	case code <= 3:
		return "CLOUDY"
	case code <= 48:
		return "FOGGY"
	case code <= 55:
		return "DRIZZLE"
	case code <= 67:
		return "RAIN"
	case code <= 77:
		return "SNOW"
	case code <= 82:
		return "SHOWERS"
	case code <= 86:
		return "SNOW SHWRS"
	default:
		return "STORM"
	}
}
