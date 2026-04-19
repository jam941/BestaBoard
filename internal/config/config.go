package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultConfigPath = "./config.yaml"


type Config struct {
	RotationInterval Duration      `yaml:"rotation_interval"`
	StaticText       string        `yaml:"static_text"`
	Weather          WeatherConfig `yaml:"weather"`
	NoteDuration Duration `yaml:"note_duration"`
}


type WeatherConfig struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
	Timezone string `yaml:"timezone"`
	Units string `yaml:"units"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	d.Duration = dur
	return nil
}

func Load() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = defaultConfigPath
	}

	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	if cfg.RotationInterval.Duration <= 0 {
		cfg.RotationInterval.Duration = 5 * time.Minute
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		RotationInterval: Duration{5 * time.Minute},
		StaticText:       "HELLO WORLD",
		NoteDuration:     Duration{15 * time.Minute},
		Weather: WeatherConfig{
			Timezone: "UTC",
			Units:    "fahrenheit",
		},
	}
}
