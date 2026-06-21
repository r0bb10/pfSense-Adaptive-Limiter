package counters

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Sample struct {
	ReceivedBytes    uint64
	TransmittedBytes uint64
	TakenAt          time.Time
}

type CommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

type NetstatReader struct {
	Interface string
	Binary    string
	Runner    CommandRunner
	Now       func() time.Time
}

func NewNetstatReader(interfaceName string) *NetstatReader {
	return &NetstatReader{
		Interface: interfaceName,
		Binary:    "/usr/bin/netstat",
		Runner:    ExecRunner{},
		Now:       time.Now,
	}
}

func (r *NetstatReader) Read(ctx context.Context) (Sample, error) {
	output, err := r.Runner.Output(ctx, r.Binary, "-I", r.Interface, "-b", "-n")
	if err != nil {
		return Sample{}, fmt.Errorf("read %s counters: %w", r.Interface, err)
	}
	rx, tx, err := ParseNetstat(output, r.Interface)
	if err != nil {
		return Sample{}, err
	}
	return Sample{ReceivedBytes: rx, TransmittedBytes: tx, TakenAt: r.Now()}, nil
}

func ParseNetstat(output []byte, interfaceName string) (uint64, uint64, error) {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return 0, 0, errors.New("netstat output contains no interface rows")
	}

	header := strings.Fields(lines[0])
	nameColumn := fieldIndex(header, "Name")
	rxColumn := fieldIndex(header, "Ibytes")
	txColumn := fieldIndex(header, "Obytes")
	if nameColumn < 0 || rxColumn < 0 || txColumn < 0 {
		return 0, 0, fmt.Errorf("netstat output is missing Name, Ibytes, or Obytes columns: %q", lines[0])
	}

	maxColumn := max(nameColumn, rxColumn, txColumn)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) <= maxColumn || fields[nameColumn] != interfaceName {
			continue
		}
		rx, rxErr := strconv.ParseUint(fields[rxColumn], 10, 64)
		tx, txErr := strconv.ParseUint(fields[txColumn], 10, 64)
		if rxErr != nil || txErr != nil {
			continue
		}
		return rx, tx, nil
	}
	return 0, 0, fmt.Errorf("no numeric counters found for interface %q", interfaceName)
}

func Throughput(previous, current Sample) (downloadMbps, uploadMbps float64, ok bool) {
	seconds := current.TakenAt.Sub(previous.TakenAt).Seconds()
	if seconds <= 0 || current.ReceivedBytes < previous.ReceivedBytes || current.TransmittedBytes < previous.TransmittedBytes {
		return 0, 0, false
	}
	downloadMbps = float64(current.ReceivedBytes-previous.ReceivedBytes) * 8 / seconds / 1_000_000
	uploadMbps = float64(current.TransmittedBytes-previous.TransmittedBytes) * 8 / seconds / 1_000_000
	return downloadMbps, uploadMbps, true
}

func fieldIndex(fields []string, wanted string) int {
	for index, field := range fields {
		if field == wanted {
			return index
		}
	}
	return -1
}
