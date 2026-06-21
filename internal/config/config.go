package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"time"
)

var interfacePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

type Direction struct {
	Pipe     int     `json:"pipe"`
	Minimum  float64 `json:"minimum_mbps"`
	Baseline float64 `json:"baseline_mbps"`
	Maximum  float64 `json:"maximum_mbps"`
}

type Config struct {
	Enabled          bool      `json:"enabled"`
	MonitorOnly      bool      `json:"monitor_only"`
	WANInterface     string    `json:"wan_interface"`
	Download         Direction `json:"download"`
	Upload           Direction `json:"upload"`
	Reflectors       []string  `json:"reflectors"`
	LatencyThreshold float64   `json:"latency_threshold_ms"`
	SampleInterval   Duration  `json:"sample_interval"`
	ProbeInterval    Duration  `json:"probe_interval"`
	ProbeTimeout     Duration  `json:"probe_timeout"`
	AdjustmentDelay  Duration  `json:"adjustment_delay"`
	StatusPath       string    `json:"status_path"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("duration must be a string such as 1s")
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Config{}, fmt.Errorf("decode %s: trailing JSON data", path)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !interfacePattern.MatchString(c.WANInterface) {
		return fmt.Errorf("invalid WAN interface %q", c.WANInterface)
	}
	if err := validateDirection("download", c.Download); err != nil {
		return err
	}
	if err := validateDirection("upload", c.Upload); err != nil {
		return err
	}
	if c.Download.Pipe == c.Upload.Pipe {
		return errors.New("download and upload pipes must differ")
	}
	if len(c.Reflectors) == 0 {
		return errors.New("at least one reflector is required")
	}
	for _, reflector := range c.Reflectors {
		ip := net.ParseIP(reflector)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("invalid IPv4 reflector address %q", reflector)
		}
	}
	if c.LatencyThreshold <= 0 || c.LatencyThreshold > 1000 {
		return errors.New("latency threshold must be greater than 0 and no more than 1000 ms")
	}
	if c.SampleInterval.Duration < 200*time.Millisecond || c.SampleInterval.Duration > time.Minute {
		return errors.New("sample interval must be between 200ms and 1m")
	}
	if c.ProbeInterval.Duration < 100*time.Millisecond || c.ProbeInterval.Duration > 10*time.Second {
		return errors.New("probe interval must be between 100ms and 10s")
	}
	if c.ProbeTimeout.Duration < 100*time.Millisecond || c.ProbeTimeout.Duration > 10*time.Second {
		return errors.New("probe timeout must be between 100ms and 10s")
	}
	if c.AdjustmentDelay.Duration < c.SampleInterval.Duration || c.AdjustmentDelay.Duration > 10*time.Minute {
		return errors.New("adjustment delay must be at least the sample interval and no more than 10m")
	}
	if c.StatusPath == "" || c.StatusPath[0] != '/' {
		return errors.New("status path must be absolute")
	}
	return nil
}

func validateDirection(name string, d Direction) error {
	if d.Pipe <= 0 {
		return fmt.Errorf("%s pipe must be positive", name)
	}
	if d.Minimum <= 0 || d.Minimum > d.Baseline || d.Baseline > d.Maximum {
		return fmt.Errorf("%s rates must satisfy 0 < minimum <= baseline <= maximum", name)
	}
	return nil
}
