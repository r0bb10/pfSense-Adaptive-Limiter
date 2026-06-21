package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/r0bb10/pfsense-adaptive-limiter/internal/config"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/status"
)

func Run(ctx context.Context, cfg config.Config, version string, logger *slog.Logger) error {
	startedAt := time.Now().UTC()
	mode := "monitor"
	if !cfg.MonitorOnly {
		// Active control is intentionally unavailable until the actuator milestone.
		mode = "monitor"
		logger.Warn("active mode requested but not implemented; remaining in monitor mode")
	}

	current := status.Status{
		Version:      version,
		StartedAt:    startedAt,
		Mode:         mode,
		WANInterface: cfg.WANInterface,
		Download: status.Direction{
			Pipe:         cfg.Download.Pipe,
			MinimumMbps:  cfg.Download.Minimum,
			BaselineMbps: cfg.Download.Baseline,
			MaximumMbps:  cfg.Download.Maximum,
			CurrentMbps:  cfg.Download.Baseline,
			State:        "initializing",
		},
		Upload: status.Direction{
			Pipe:         cfg.Upload.Pipe,
			MinimumMbps:  cfg.Upload.Minimum,
			BaselineMbps: cfg.Upload.Baseline,
			MaximumMbps:  cfg.Upload.Maximum,
			CurrentMbps:  cfg.Upload.Baseline,
			State:        "initializing",
		},
		LastReason: "milestone 1 service scaffold; monitoring arrives in milestone 2",
	}

	write := func() error {
		current.UpdatedAt = time.Now().UTC()
		return status.WriteAtomic(cfg.StatusPath, current)
	}
	if err := write(); err != nil {
		return err
	}

	logger.Info("service started", "version", version, "interface", cfg.WANInterface, "mode", mode)
	ticker := time.NewTicker(cfg.SampleInterval.Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("service stopping")
			return nil
		case <-ticker.C:
			if err := write(); err != nil {
				return fmt.Errorf("refresh status: %w", err)
			}
		}
	}
}
