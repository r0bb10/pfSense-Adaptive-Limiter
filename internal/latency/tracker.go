package latency

import (
	"math"
	"slices"
	"time"
)

const (
	baselineRiseAlpha = 0.001
	baselineFallAlpha = 0.2
	failureThreshold  = 3
)

type Reflector struct {
	Address             string
	BaselineMs          float64
	CurrentMs           float64
	LastSeen            time.Time
	ConsecutiveFailures int
	LastError           string
	Initialized         bool
}

func (r *Reflector) Success(rtt time.Duration, at time.Time) {
	current := float64(rtt) / float64(time.Millisecond)
	if !r.Initialized {
		r.BaselineMs = current
		r.Initialized = true
	} else {
		alpha := baselineRiseAlpha
		if current < r.BaselineMs {
			alpha = baselineFallAlpha
		}
		r.BaselineMs += alpha * (current - r.BaselineMs)
	}
	r.CurrentMs = current
	r.LastSeen = at
	r.ConsecutiveFailures = 0
	r.LastError = ""
}

func (r *Reflector) Failure(message string) {
	r.ConsecutiveFailures++
	r.LastError = message
}

func (r Reflector) Healthy(now time.Time, staleAfter time.Duration) bool {
	return r.Initialized && r.ConsecutiveFailures < failureThreshold && now.Sub(r.LastSeen) <= staleAfter
}

func (r Reflector) DeltaMs() float64 {
	return math.Max(0, r.CurrentMs-r.BaselineMs)
}

type Aggregate struct {
	CurrentMs  float64
	BaselineMs float64
	DeltaMs    float64
	Healthy    int
}

func Summarize(reflectors map[string]*Reflector, now time.Time, staleAfter time.Duration) Aggregate {
	currents := make([]float64, 0, len(reflectors))
	baselines := make([]float64, 0, len(reflectors))
	deltas := make([]float64, 0, len(reflectors))
	for _, reflector := range reflectors {
		if !reflector.Healthy(now, staleAfter) {
			continue
		}
		currents = append(currents, reflector.CurrentMs)
		baselines = append(baselines, reflector.BaselineMs)
		deltas = append(deltas, reflector.DeltaMs())
	}
	return Aggregate{
		CurrentMs:  median(currents),
		BaselineMs: median(baselines),
		DeltaMs:    median(deltas),
		Healthy:    len(currents),
	}
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyOfValues := slices.Clone(values)
	slices.Sort(copyOfValues)
	middle := len(copyOfValues) / 2
	if len(copyOfValues)%2 == 1 {
		return copyOfValues[middle]
	}
	return (copyOfValues[middle-1] + copyOfValues[middle]) / 2
}
