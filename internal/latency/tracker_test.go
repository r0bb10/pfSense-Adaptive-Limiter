package latency

import (
	"math"
	"testing"
	"time"
)

func TestBaselineRespondsSlowlyToLatencyIncrease(t *testing.T) {
	now := time.Unix(100, 0)
	reflector := Reflector{Address: "1.1.1.1"}
	reflector.Success(6*time.Millisecond, now)
	reflector.Success(30*time.Millisecond, now.Add(time.Second))
	if math.Abs(reflector.BaselineMs-6.024) > 0.0001 {
		t.Fatalf("unexpected baseline: %f", reflector.BaselineMs)
	}
	if reflector.DeltaMs() < 23.9 {
		t.Fatalf("congestion was learned into baseline: delta=%f", reflector.DeltaMs())
	}
}

func TestBaselineRespondsQuicklyToLatencyDecrease(t *testing.T) {
	now := time.Unix(100, 0)
	reflector := Reflector{Address: "1.1.1.1"}
	reflector.Success(10*time.Millisecond, now)
	reflector.Success(5*time.Millisecond, now.Add(time.Second))
	if reflector.BaselineMs != 9 {
		t.Fatalf("unexpected baseline: %f", reflector.BaselineMs)
	}
}

func TestSummarizeUsesMedianAndExcludesFailedReflectors(t *testing.T) {
	now := time.Unix(100, 0)
	reflectors := map[string]*Reflector{}
	for index, address := range []string{"1.1.1.1", "9.9.9.9", "8.8.8.8"} {
		reflector := &Reflector{Address: address}
		reflector.Success(time.Duration(6+index)*time.Millisecond, now)
		reflectors[address] = reflector
	}
	for range failureThreshold {
		reflectors["8.8.8.8"].Failure("timeout")
	}
	aggregate := Summarize(reflectors, now, 10*time.Second)
	if aggregate.Healthy != 2 || aggregate.CurrentMs != 6.5 {
		t.Fatalf("unexpected aggregate: %#v", aggregate)
	}
}
