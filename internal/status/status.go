package status

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Direction struct {
	Pipe           int     `json:"pipe"`
	MinimumMbps    float64 `json:"minimum_mbps"`
	BaselineMbps   float64 `json:"baseline_mbps"`
	MaximumMbps    float64 `json:"maximum_mbps"`
	CurrentMbps    float64 `json:"current_mbps"`
	ThroughputMbps float64 `json:"throughput_mbps"`
	State          string  `json:"state"`
}

type Reflector struct {
	Address    string     `json:"address"`
	Healthy    bool       `json:"healthy"`
	CurrentMs  float64    `json:"current_rtt_ms"`
	BaselineMs float64    `json:"baseline_rtt_ms"`
	DeltaMs    float64    `json:"delay_delta_ms"`
	LastSeen   *time.Time `json:"last_seen,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
}

type Status struct {
	Version          string      `json:"version"`
	UpdatedAt        time.Time   `json:"updated_at"`
	StartedAt        time.Time   `json:"started_at"`
	Mode             string      `json:"mode"`
	WANInterface     string      `json:"wan_interface"`
	Download         Direction   `json:"download"`
	Upload           Direction   `json:"upload"`
	BaselineRTTMs    float64     `json:"baseline_rtt_ms"`
	CurrentRTTMs     float64     `json:"current_rtt_ms"`
	DelayDeltaMs     float64     `json:"delay_delta_ms"`
	HealthyReflector int         `json:"healthy_reflectors"`
	Reflectors       []Reflector `json:"reflectors"`
	LastError        string      `json:"last_error,omitempty"`
	LastAdjustmentAt *time.Time  `json:"last_adjustment_at,omitempty"`
	LastReason       string      `json:"last_reason"`
}

func WriteAtomic(path string, value Status) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode status: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create status directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".status-*")
	if err != nil {
		return fmt.Errorf("create temporary status: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("set status permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write status: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync status: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close status: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("publish status: %w", err)
	}
	return nil
}
