package mom

import (
	"math"
	"testing"
)

// TestOptimizer_DipoleLengthForSWR verifies that the PSO can tune a dipole's
// half-length to minimise SWR at a target frequency.  A half-wave dipole at
// 14 MHz has optimal SWR near 1:1 when the total length ≈ λ/2 ≈ 10.71 m.
// We start with a deliberately mis-tuned dipole (8 m total) and let the
// optimizer find a better length.
func TestOptimizer_DipoleLengthForSWR(t *testing.T) {
	freq := 14.0e6
	lambda := C0 / freq

	// Start with a short dipole (8 m total, each arm = 4 m)
	req := OptimRequest{
		Input: SimulationInput{
			Wires: []Wire{
				{X1: 0, Y1: 0, Z1: -4.0, X2: 0, Y2: 0, Z2: 4.0, Radius: 0.001, Segments: 21},
			},
			Frequency: freq,
			Ground:    GroundConfig{Type: "free_space"},
			Source:    Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
		},
		Variables: []OptimVariable{
			{Name: "top_z", WireIndex: 0, Field: "z2", Min: 3.0, Max: 7.0},
			{Name: "bot_z", WireIndex: 0, Field: "z1", Min: -7.0, Max: -3.0},
		},
		Goals: []OptimGoal{
			{Metric: "swr", Target: 1.0, Weight: 10.0},
		},
		Particles:  15,
		Iterations: 20,
		Seed:       42,
	}

	result, err := RunOptimizer(req)
	if err != nil {
		t.Fatalf("RunOptimizer failed: %v", err)
	}

	// The optimal total length should be near λ/2
	topZ := result.BestParams["top_z"]
	botZ := result.BestParams["bot_z"]
	totalLen := topZ - botZ
	expectedLen := lambda / 2.0

	t.Logf("Optimized length: %.4f m (expected ~%.4f m)", totalLen, expectedLen)
	t.Logf("Best SWR: %.3f, Best cost: %.4f", result.BestMetrics["swr"], result.BestCost)
	t.Logf("Convergence: %v", result.Convergence)

	// The length should be within 15% of lambda/2
	if math.Abs(totalLen-expectedLen)/expectedLen > 0.15 {
		t.Errorf("total length %.4f m not within 15%% of λ/2 = %.4f m", totalLen, expectedLen)
	}

	// SWR should be better than what we started with (a short dipole has SWR >> 2)
	if result.BestMetrics["swr"] > 3.0 {
		t.Errorf("optimized SWR = %.2f, expected < 3.0", result.BestMetrics["swr"])
	}

	// Convergence should be monotonically non-increasing
	for i := 1; i < len(result.Convergence); i++ {
		if result.Convergence[i] > result.Convergence[i-1]+1e-10 {
			t.Errorf("convergence not monotone at gen %d: %.4f > %.4f",
				i, result.Convergence[i], result.Convergence[i-1])
		}
	}

	// OptimizedWires should be populated
	if len(result.OptimizedWires) != 1 {
		t.Errorf("expected 1 optimized wire, got %d", len(result.OptimizedWires))
	}
}

// TestOptimizer_BandEvaluation checks that band-based optimisation
// runs without error and produces reasonable results.
func TestOptimizer_BandEvaluation(t *testing.T) {
	freq := 14.1e6

	req := OptimRequest{
		Input: SimulationInput{
			Wires: []Wire{
				{X1: 0, Y1: 0, Z1: -5.0, X2: 0, Y2: 0, Z2: 5.0, Radius: 0.001, Segments: 21},
			},
			Frequency: freq,
			Ground:    GroundConfig{Type: "free_space"},
			Source:    Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
		},
		Variables: []OptimVariable{
			{Name: "top_z", WireIndex: 0, Field: "z2", Min: 4.0, Max: 6.5},
		},
		Goals: []OptimGoal{
			{Metric: "swr", Target: 1.0, Weight: 10.0},
		},
		FreqStartHz: 14.0e6,
		FreqEndHz:   14.35e6,
		FreqSteps:   3,
		Particles:   10,
		Iterations:  10,
		Seed:        123,
	}

	result, err := RunOptimizer(req)
	if err != nil {
		t.Fatalf("RunOptimizer (band) failed: %v", err)
	}

	t.Logf("Band optimization: best cost=%.4f, SWR=%.3f",
		result.BestCost, result.BestMetrics["swr"])

	if result.BestCost >= 1e12 {
		t.Error("all evaluations failed (cost = 1e12)")
	}
}

// TestOptimizer_Validation checks error handling for bad inputs.
func TestOptimizer_Validation(t *testing.T) {
	freq := 14.0e6

	// No variables
	_, err := RunOptimizer(OptimRequest{
		Input: SimulationInput{
			Wires:     []Wire{{X1: 0, Y1: 0, Z1: -5, X2: 0, Y2: 0, Z2: 5, Radius: 0.001, Segments: 11}},
			Frequency: freq,
			Ground:    GroundConfig{Type: "free_space"},
			Source:    Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		},
		Variables:  []OptimVariable{},
		Goals:      []OptimGoal{{Metric: "swr", Target: 1.0, Weight: 1.0}},
		Particles:  5,
		Iterations: 5,
	})
	if err == nil {
		t.Error("expected error for empty variables")
	}

	// No goals
	_, err = RunOptimizer(OptimRequest{
		Input: SimulationInput{
			Wires:     []Wire{{X1: 0, Y1: 0, Z1: -5, X2: 0, Y2: 0, Z2: 5, Radius: 0.001, Segments: 11}},
			Frequency: freq,
			Ground:    GroundConfig{Type: "free_space"},
			Source:    Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		},
		Variables:  []OptimVariable{{Name: "z2", WireIndex: 0, Field: "z2", Min: 3, Max: 7}},
		Goals:      []OptimGoal{},
		Particles:  5,
		Iterations: 5,
	})
	if err == nil {
		t.Error("expected error for empty goals")
	}
}

// TestApplyParams checks that parameter application works correctly.
func TestApplyParams(t *testing.T) {
	wires := []Wire{
		{X1: 0, Y1: 0, Z1: -5, X2: 0, Y2: 0, Z2: 5, Radius: 0.001, Segments: 11},
		{X1: 1, Y1: -2, Z1: 10, X2: 1, Y2: 2, Z2: 10, Radius: 0.001, Segments: 11},
	}
	vars := []OptimVariable{
		{Name: "w0_z2", WireIndex: 0, Field: "z2", Min: 3, Max: 7},
		{Name: "w1_y2", WireIndex: 1, Field: "y2", Min: 1, Max: 4},
	}
	params := []float64{6.0, 3.5}

	out := applyParams(wires, vars, params)

	if out[0].Z2 != 6.0 {
		t.Errorf("wire 0 Z2 = %f, want 6.0", out[0].Z2)
	}
	if out[1].Y2 != 3.5 {
		t.Errorf("wire 1 Y2 = %f, want 3.5", out[1].Y2)
	}
	// Original should be unchanged
	if wires[0].Z2 != 5 {
		t.Error("original wires were mutated")
	}
}
