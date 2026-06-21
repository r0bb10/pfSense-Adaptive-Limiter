package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		Enabled:          true,
		MonitorOnly:      true,
		WANInterface:     "pppoe0",
		Download:         Direction{Pipe: 1, Minimum: 500, Baseline: 1800, Maximum: 2500},
		Upload:           Direction{Pipe: 2, Minimum: 250, Baseline: 800, Maximum: 1000},
		Reflectors:       []string{"1.1.1.1", "9.9.9.9"},
		LatencyThreshold: 15,
		SampleInterval:   Duration{time.Second},
		AdjustmentDelay:  Duration{2 * time.Second},
		StatusPath:       "/var/run/adaptive-limiter/status.json",
	}
}

func TestValidate(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}
}

func TestValidateRejectsInvalidRates(t *testing.T) {
	cfg := validConfig()
	cfg.Download.Minimum = cfg.Download.Maximum + 1
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid rates accepted")
	}
}

func TestValidateRejectsSharedPipe(t *testing.T) {
	cfg := validConfig()
	cfg.Upload.Pipe = cfg.Download.Pipe
	if err := cfg.Validate(); err == nil {
		t.Fatal("shared pipe accepted")
	}
}

func TestValidateRejectsInvalidReflector(t *testing.T) {
	cfg := validConfig()
	cfg.Reflectors = []string{"not-an-ip"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid reflector accepted")
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{
  "enabled": true,
  "monitor_only": true,
  "wan_interface": "pppoe0",
  "download": {"pipe": 1, "minimum_mbps": 500, "baseline_mbps": 1800, "maximum_mbps": 2500},
  "upload": {"pipe": 2, "minimum_mbps": 250, "baseline_mbps": 800, "maximum_mbps": 1000},
  "reflectors": ["1.1.1.1"],
  "latency_threshold_ms": 15,
  "sample_interval": "1s",
  "adjustment_delay": "2s",
  "status_path": "/tmp/status.json",
  "unexpected": true
}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestLoadExampleConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "configs", "config.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WANInterface != "pppoe0" || cfg.Download.Pipe != 1 || cfg.Upload.Pipe != 2 {
		t.Fatalf("unexpected example configuration: %#v", cfg)
	}
}
