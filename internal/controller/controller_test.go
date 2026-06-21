package controller

import (
	"testing"

	"github.com/r0bb10/pfsense-adaptive-limiter/internal/config"
)

func TestEvaluateClampsLiveRateAboveMaximum(t *testing.T) {
	decision := Evaluate(direction(), 2500, 100, 0, 15)
	if decision.State != StateHold || decision.Action != ActionClampToMaximum || decision.ProposedMbps != 2000 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestEvaluateDecreasesOnLoadedLatency(t *testing.T) {
	decision := Evaluate(direction(), 1800, 1200, 22, 15)
	if decision.State != StateCongested || decision.Action != ActionDecrease || decision.ProposedMbps != 1620 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestEvaluateIncreasesOnHealthySaturation(t *testing.T) {
	decision := Evaluate(direction(), 1800, 1700, 3, 15)
	if decision.State != StateHealthyLoad || decision.Action != ActionIncrease || decision.ProposedMbps != 1854 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestEvaluateMovesIdleTowardBaseline(t *testing.T) {
	decision := Evaluate(direction(), 1500, 10, 1, 15)
	if decision.State != StateIdle || decision.Action != ActionReturnToBaseline || decision.ProposedMbps != 1506 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestEvaluateHoldsWhenLatencyIsHighButDirectionIdle(t *testing.T) {
	decision := Evaluate(direction(), 1800, 20, 30, 15)
	if decision.State != StateIdle || decision.Action != ActionNone || decision.ProposedMbps != 1800 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func direction() config.Direction {
	return config.Direction{Minimum: 500, Baseline: 1800, Maximum: 2000}
}
