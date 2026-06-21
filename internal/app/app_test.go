package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/r0bb10/pfsense-adaptive-limiter/internal/config"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/counters"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/status"
)

type fakeCounters struct {
	mu      sync.Mutex
	samples []counters.Sample
	index   int
}

func (f *fakeCounters) Read(context.Context) (counters.Sample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sample := f.samples[min(f.index, len(f.samples)-1)]
	f.index++
	return sample, nil
}

type fakeProber struct {
	rtt time.Duration
}

func (f *fakeProber) Probe(context.Context, string) (time.Duration, error) { return f.rtt, nil }
func (f *fakeProber) Close() error                                         { return nil }

type fakeLimiters struct {
	rates map[int]float64
}

func (f *fakeLimiters) Rates(context.Context) (map[int]float64, error) { return f.rates, nil }

func TestRunPublishesMeasurementsAndStops(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "status.json")
	start := time.Unix(100, 0)
	cfg := config.Config{
		Enabled:          true,
		MonitorOnly:      true,
		WANInterface:     "pppoe0",
		Download:         config.Direction{Pipe: 1, Minimum: 500, Baseline: 1800, Maximum: 2500},
		Upload:           config.Direction{Pipe: 2, Minimum: 250, Baseline: 800, Maximum: 1000},
		Reflectors:       []string{"1.1.1.1"},
		LatencyThreshold: 15,
		SampleInterval:   config.Duration{Duration: 200 * time.Millisecond},
		ProbeInterval:    config.Duration{Duration: 100 * time.Millisecond},
		ProbeTimeout:     config.Duration{Duration: time.Second},
		AdjustmentDelay:  config.Duration{Duration: time.Second},
		StatusPath:       path,
	}
	deps := dependencies{
		counters: &fakeCounters{samples: []counters.Sample{
			{ReceivedBytes: 1000, TransmittedBytes: 1000, TakenAt: start},
			{ReceivedBytes: 126000, TransmittedBytes: 63500, TakenAt: start.Add(time.Second)},
		}},
		limiters: &fakeLimiters{rates: map[int]float64{1: 2150, 2: 970}},
		prober:   &fakeProber{rtt: 7 * time.Millisecond},
		now:      time.Now,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWithDependencies(ctx, cfg, "test-version", slog.New(slog.NewTextHandler(io.Discard, nil)), deps)
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var got status.Status
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.Download.ThroughputMbps == 1 && got.Upload.ThroughputMbps == 0.5 && got.HealthyReflector == 1 {
				if got.CurrentRTTMs != 7 || got.Reflectors[0].Address != "1.1.1.1" {
					t.Fatalf("unexpected status: %#v", got)
				}
				if got.Download.CurrentMbps != 2150 || got.Upload.CurrentMbps != 970 {
					t.Fatalf("live limiter rates not published: %#v", got)
				}
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("measurements were not published: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned an error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run did not stop after cancellation")
	}
}
