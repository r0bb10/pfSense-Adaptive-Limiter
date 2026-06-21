package dummynet

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var pipeLine = regexp.MustCompile(`^\s*0*([0-9]+):\s+([0-9]+(?:\.[0-9]+)?)\s+([KMG]?bit/s)\b`)

type CommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

type Reader struct {
	Binary string
	Runner CommandRunner
}

func NewReader() *Reader {
	return &Reader{Binary: "/sbin/dnctl", Runner: ExecRunner{}}
}

func (r *Reader) Rates(ctx context.Context) (map[int]float64, error) {
	output, err := r.Runner.Output(ctx, r.Binary, "pipe", "show")
	if err != nil {
		return nil, fmt.Errorf("read dummynet pipes: %w", err)
	}
	return ParsePipeRates(output)
}

func ParsePipeRates(output []byte) (map[int]float64, error) {
	rates := make(map[int]float64)
	for _, line := range strings.Split(string(output), "\n") {
		matches := pipeLine.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		pipe, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}
		rate, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			continue
		}
		switch matches[3] {
		case "Kbit/s":
			rate /= 1000
		case "Gbit/s":
			rate *= 1000
		case "Mbit/s":
		case "bit/s":
			rate /= 1_000_000
		default:
			continue
		}
		rates[pipe] = rate
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("dnctl output contains no pipe rates")
	}
	return rates, nil
}
