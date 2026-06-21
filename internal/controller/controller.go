package controller

import (
	"math"

	"github.com/r0bb10/pfsense-adaptive-limiter/internal/config"
)

const (
	StateInitializing = "initializing"
	StateIdle         = "idle"
	StateHealthyLoad  = "healthy_load"
	StateCongested    = "congested"
	StateHold         = "hold"

	ActionNone             = "none"
	ActionIncrease         = "increase"
	ActionDecrease         = "decrease"
	ActionReturnToBaseline = "return_to_baseline"
	ActionClampToMinimum   = "clamp_to_minimum"
	ActionClampToMaximum   = "clamp_to_maximum"
)

type Decision struct {
	State        string
	Action       string
	ProposedMbps float64
	Reason       string
}

func Evaluate(direction config.Direction, currentMbps, throughputMbps, delayDeltaMs, latencyThresholdMs float64) Decision {
	if currentMbps <= 0 {
		return Decision{State: StateInitializing, Action: ActionNone, ProposedMbps: 0, Reason: "waiting for live limiter rate"}
	}

	if currentMbps < direction.Minimum {
		return Decision{State: StateHold, Action: ActionClampToMinimum, ProposedMbps: direction.Minimum, Reason: "live rate is below configured minimum"}
	}
	if currentMbps > direction.Maximum {
		return Decision{State: StateHold, Action: ActionClampToMaximum, ProposedMbps: direction.Maximum, Reason: "live rate is above configured maximum"}
	}

	utilization := throughputMbps / currentMbps
	if utilization < 0 {
		utilization = 0
	}

	congested := delayDeltaMs >= latencyThresholdMs && utilization >= 0.50
	if congested {
		proposed := clamp(roundMbps(currentMbps*0.90), direction.Minimum, direction.Maximum)
		return Decision{State: StateCongested, Action: ActionDecrease, ProposedMbps: proposed, Reason: "latency threshold exceeded while direction is loaded"}
	}

	idle := utilization < 0.10
	if idle {
		proposed := moveToward(currentMbps, direction.Baseline, 0.02, direction.Minimum, direction.Maximum)
		if proposed != roundMbps(currentMbps) {
			return Decision{State: StateIdle, Action: ActionReturnToBaseline, ProposedMbps: proposed, Reason: "traffic is idle; proposed rate returns toward baseline"}
		}
		return Decision{State: StateIdle, Action: ActionNone, ProposedMbps: roundMbps(currentMbps), Reason: "traffic is idle and rate is at baseline"}
	}

	healthySaturation := utilization >= 0.85 && delayDeltaMs < latencyThresholdMs*0.50
	if healthySaturation {
		proposed := clamp(roundMbps(currentMbps*1.03), direction.Minimum, direction.Maximum)
		if proposed > roundMbps(currentMbps) {
			return Decision{State: StateHealthyLoad, Action: ActionIncrease, ProposedMbps: proposed, Reason: "high utilization with healthy latency"}
		}
		return Decision{State: StateHealthyLoad, Action: ActionNone, ProposedMbps: roundMbps(currentMbps), Reason: "high utilization but already at configured maximum"}
	}

	return Decision{State: StateHold, Action: ActionNone, ProposedMbps: roundMbps(currentMbps), Reason: "no rate change proposed"}
}

func moveToward(current, target, fraction, minimum, maximum float64) float64 {
	if current == target {
		return roundMbps(current)
	}
	step := math.Max(math.Abs(current-target)*fraction, 1)
	if current > target {
		return clamp(roundMbps(math.Max(target, current-step)), minimum, maximum)
	}
	return clamp(roundMbps(math.Min(target, current+step)), minimum, maximum)
}

func clamp(value, minimum, maximum float64) float64 {
	return math.Min(math.Max(value, minimum), maximum)
}

func roundMbps(value float64) float64 {
	return math.Round(value*10) / 10
}
