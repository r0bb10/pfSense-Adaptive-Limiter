package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/r0bb10/pfsense-adaptive-limiter/internal/config"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/status"
)

func TestRunPublishesStatusAndStops(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "status.json")
	cfg := config.Config{
		Enabled:          true,
		MonitorOnly:      true,
		WANInterface:     "pppoe0",
		Download:         config.Direction{Pipe: 1, Minimum: 500, Baseline: 1800, Maximum: 2500},
		Upload:           config.Direction{Pipe: 2, Minimum: 250, Baseline: 800, Maximum: 1000},
		Reflectors:       []string{"1.1.1.1"},
		LatencyThreshold: 15,
		SampleInterval:   config.Duration{Duration: 200 * time.Millisecond},
		AdjustmentDelay:  config.Duration{Duration: time.Second},
		StatusPath:       path,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, cfg, "test-version", slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var got status.Status
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.Version != "test-version" || got.Download.CurrentMbps != 1800 {
				t.Fatalf("unexpected status: %#v", got)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status was not published: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned an error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after cancellation")
	}
}
