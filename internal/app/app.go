package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/r0bb10/pfsense-adaptive-limiter/internal/config"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/controller"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/counters"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/dummynet"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/latency"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/probe"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/status"
)

type counterReader interface {
	Read(ctx context.Context) (counters.Sample, error)
}

type limiterReader interface {
	Rates(ctx context.Context) (map[int]float64, error)
}

type dependencies struct {
	counters counterReader
	limiters limiterReader
	prober   probe.Prober
	now      func() time.Time
}

type probeResult struct {
	address string
	rtt     time.Duration
	at      time.Time
	err     error
}

func Run(ctx context.Context, cfg config.Config, version string, logger *slog.Logger) error {
	deps := dependencies{
		counters: counters.NewNetstatReader(cfg.WANInterface),
		limiters: dummynet.NewReader(),
		prober:   probe.NewInterfaceICMP(cfg.WANInterface, cfg.ProbeTimeout.Duration),
		now:      time.Now,
	}
	return runWithDependencies(ctx, cfg, version, logger, deps)
}

func runWithDependencies(ctx context.Context, cfg config.Config, version string, logger *slog.Logger, deps dependencies) error {
	defer deps.prober.Close()

	startedAt := deps.now().UTC()
	mode := "monitor"
	if !cfg.MonitorOnly {
		logger.Warn("active mode requested but not implemented; remaining in monitor mode")
	}

	current := initialStatus(cfg, version, startedAt, mode)
	trackers := make(map[string]*latency.Reflector, len(cfg.Reflectors))
	for _, address := range cfg.Reflectors {
		trackers[address] = &latency.Reflector{Address: address}
	}
	staleAfter := cfg.ProbeInterval.Duration * time.Duration(len(cfg.Reflectors)) * 3
	if minimum := cfg.ProbeTimeout.Duration * 2; staleAfter < minimum {
		staleAfter = minimum
	}

	write := func() error {
		now := deps.now().UTC()
		current.UpdatedAt = now
		updateLatencyStatus(&current, cfg.Reflectors, trackers, now, staleAfter)
		updateControllerSimulation(&current, cfg)
		return status.WriteAtomic(cfg.StatusPath, current)
	}
	if err := write(); err != nil {
		return err
	}

	logger.Info("monitoring started", "version", version, "interface", cfg.WANInterface,
		"reflectors", len(cfg.Reflectors), "sample_interval", cfg.SampleInterval.String(),
		"probe_interval", cfg.ProbeInterval.String())

	counterTicker := time.NewTicker(cfg.SampleInterval.Duration)
	probeTicker := time.NewTicker(cfg.ProbeInterval.Duration)
	defer counterTicker.Stop()
	defer probeTicker.Stop()

	var previousCounter *counters.Sample
	if sample, err := deps.counters.Read(ctx); err != nil {
		current.LastError = err.Error()
		logger.Warn("initial counter read failed", "error", err)
	} else {
		previousCounter = &sample
	}

	probeResults := make(chan probeResult, 1)
	probeIndex := 0
	probeInFlight := false
	launchProbe := func() {
		if probeInFlight || len(cfg.Reflectors) == 0 {
			return
		}
		address := cfg.Reflectors[probeIndex%len(cfg.Reflectors)]
		probeIndex++
		probeInFlight = true
		go func() {
			rtt, err := deps.prober.Probe(ctx, address)
			result := probeResult{address: address, rtt: rtt, at: deps.now().UTC(), err: err}
			select {
			case probeResults <- result:
			case <-ctx.Done():
			}
		}()
	}
	launchProbe()

	for {
		select {
		case <-ctx.Done():
			logger.Info("monitoring stopped")
			return nil

		case <-counterTicker.C:
			sample, err := deps.counters.Read(ctx)
			if err != nil {
				current.LastError = err.Error()
				current.LastReason = "traffic counter read failed; monitoring will retry"
				logger.Warn("counter read failed", "error", err)
			} else {
				if previousCounter != nil {
					download, upload, ok := counters.Throughput(*previousCounter, sample)
					if ok {
						current.Download.ThroughputMbps = download
						current.Upload.ThroughputMbps = upload
						current.LastError = ""
						current.LastReason = "read-only traffic and latency monitoring"
					} else {
						current.LastReason = "interface counter reset detected; rate sample skipped"
					}
				}
				previousCounter = &sample
			}
			rates, limiterErr := deps.limiters.Rates(ctx)
			if limiterErr != nil {
				current.LastError = limiterErr.Error()
				current.LastReason = "limiter rate read failed; monitoring will retry"
				logger.Warn("limiter rate read failed", "error", limiterErr)
			} else {
				downloadRate, downloadFound := rates[cfg.Download.Pipe]
				uploadRate, uploadFound := rates[cfg.Upload.Pipe]
				if !downloadFound || !uploadFound {
					current.LastError = fmt.Sprintf("configured pipes not found in dummynet: download=%d upload=%d", cfg.Download.Pipe, cfg.Upload.Pipe)
					current.LastReason = "configured limiter pipe missing"
					logger.Warn("configured limiter pipe missing", "download_found", downloadFound, "upload_found", uploadFound)
				} else {
					current.Download.CurrentMbps = downloadRate
					current.Upload.CurrentMbps = uploadRate
				}
			}
			if err == nil && limiterErr == nil {
				if _, downloadFound := rates[cfg.Download.Pipe]; downloadFound {
					if _, uploadFound := rates[cfg.Upload.Pipe]; uploadFound {
						current.LastError = ""
					}
				}
			}
			if err := write(); err != nil {
				return fmt.Errorf("publish counter status: %w", err)
			}

		case <-probeTicker.C:
			launchProbe()

		case result := <-probeResults:
			probeInFlight = false
			tracker := trackers[result.address]
			if result.err != nil {
				tracker.Failure(result.err.Error())
				logger.Debug("reflector probe failed", "reflector", result.address, "error", result.err)
			} else {
				tracker.Success(result.rtt, result.at)
			}
			if err := write(); err != nil {
				return fmt.Errorf("publish probe status: %w", err)
			}
		}
	}
}

func initialStatus(cfg config.Config, version string, startedAt time.Time, mode string) status.Status {
	return status.Status{
		Version:      version,
		StartedAt:    startedAt,
		Mode:         mode,
		WANInterface: cfg.WANInterface,
		Download: status.Direction{
			Pipe:         cfg.Download.Pipe,
			MinimumMbps:  cfg.Download.Minimum,
			BaselineMbps: cfg.Download.Baseline,
			MaximumMbps:  cfg.Download.Maximum,
			CurrentMbps:  0,
			ProposedMbps: 0,
			State:        "initializing",
			Action:       "none",
			Reason:       "waiting for live limiter rate",
		},
		Upload: status.Direction{
			Pipe:         cfg.Upload.Pipe,
			MinimumMbps:  cfg.Upload.Minimum,
			BaselineMbps: cfg.Upload.Baseline,
			MaximumMbps:  cfg.Upload.Maximum,
			CurrentMbps:  0,
			ProposedMbps: 0,
			State:        "initializing",
			Action:       "none",
			Reason:       "waiting for live limiter rate",
		},
		LastReason: "waiting for traffic and latency samples",
	}
}

func updateControllerSimulation(current *status.Status, cfg config.Config) {
	if current.HealthyReflector == 0 {
		setWaitingForLatency(&current.Download)
		setWaitingForLatency(&current.Upload)
		return
	}

	download := controller.Evaluate(cfg.Download, current.Download.CurrentMbps, current.Download.ThroughputMbps, current.DelayDeltaMs, cfg.LatencyThreshold)
	current.Download.State = download.State
	current.Download.Action = download.Action
	current.Download.ProposedMbps = download.ProposedMbps
	current.Download.Reason = download.Reason

	upload := controller.Evaluate(cfg.Upload, current.Upload.CurrentMbps, current.Upload.ThroughputMbps, current.DelayDeltaMs, cfg.LatencyThreshold)
	current.Upload.State = upload.State
	current.Upload.Action = upload.Action
	current.Upload.ProposedMbps = upload.ProposedMbps
	current.Upload.Reason = upload.Reason
}

func setWaitingForLatency(direction *status.Direction) {
	direction.ProposedMbps = direction.CurrentMbps
	if direction.CurrentMbps <= 0 {
		direction.State = controller.StateInitializing
		direction.ProposedMbps = 0
	} else {
		direction.State = controller.StateHold
	}
	direction.Action = controller.ActionNone
	direction.Reason = "waiting for healthy latency reflector"
}

func updateLatencyStatus(current *status.Status, order []string, trackers map[string]*latency.Reflector, now time.Time, staleAfter time.Duration) {
	aggregate := latency.Summarize(trackers, now, staleAfter)
	current.CurrentRTTMs = aggregate.CurrentMs
	current.BaselineRTTMs = aggregate.BaselineMs
	current.DelayDeltaMs = aggregate.DeltaMs
	current.HealthyReflector = aggregate.Healthy
	current.Reflectors = make([]status.Reflector, 0, len(order))
	for _, address := range order {
		tracker := trackers[address]
		var lastSeen *time.Time
		if tracker.Initialized {
			value := tracker.LastSeen
			lastSeen = &value
		}
		current.Reflectors = append(current.Reflectors, status.Reflector{
			Address:    address,
			Healthy:    tracker.Healthy(now, staleAfter),
			CurrentMs:  tracker.CurrentMs,
			BaselineMs: tracker.BaselineMs,
			DeltaMs:    tracker.DeltaMs(),
			LastSeen:   lastSeen,
			LastError:  tracker.LastError,
		})
	}
}
