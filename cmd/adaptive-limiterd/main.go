package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/r0bb10/pfsense-adaptive-limiter/internal/app"
	"github.com/r0bb10/pfsense-adaptive-limiter/internal/config"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "/usr/local/etc/adaptive-limiter/config.json", "configuration file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("adaptive-limiterd %s\n", version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("configuration rejected", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg, version, logger); err != nil {
		logger.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}
