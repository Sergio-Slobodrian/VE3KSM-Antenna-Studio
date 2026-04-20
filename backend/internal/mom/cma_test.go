// Copyright 2026 Sergio Slobodrian
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mom

import (
	"math"
	"testing"
)

// TestCMA_HalfWaveDipole runs CMA on a half-wave dipole and checks that:
//  1. The dominant mode has MS close to 1 (resonant) near the half-wave frequency.
//  2. Modal significance values are in [0, 1].
//  3. Characteristic angles are in a valid range.
//  4. Modes are sorted by decreasing modal significance.
func TestCMA_HalfWaveDipole(t *testing.T) {
	// 14 MHz half-wave dipole (~10.71 m, 21 segments)
	freq := 14.0e6
	halfLen := 0.5 * C0 / freq / 2.0 // quarter-wave each side

	input := SimulationInput{
		Wires: []Wire{
			{X1: 0, Y1: 0, Z1: -halfLen, X2: 0, Y2: 0, Z2: halfLen, Radius: 0.001, Segments: 21},
		},
		Frequency: freq,
		Ground:    GroundConfig{Type: "free_space"},
		Source:    Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
	}

	result, err := SimulateCMA(input)
	if err != nil {
		t.Fatalf("SimulateCMA failed: %v", err)
	}

	if result.NumModes == 0 {
		t.Fatal("expected at least one mode")
	}

	if len(result.Modes) != result.NumModes {
		t.Errorf("mode count mismatch: len=%d, NumModes=%d", len(result.Modes), result.NumModes)
	}

	// Check that FreqMHz was set
	expectedMHz := freq / 1e6
	if math.Abs(result.FreqMHz-expectedMHz) > 0.001 {
		t.Errorf("FreqMHz = %f, want %f", result.FreqMHz, expectedMHz)
	}

	// Dominant mode should have high modal significance (resonant dipole)
	mode1 := result.Modes[0]
	if mode1.ModalSignificance < 0.5 {
		t.Errorf("dominant mode MS = %f, expected > 0.5 for resonant dipole", mode1.ModalSignificance)
	}

	// Verify all modes have valid MS and alpha
	for i, m := range result.Modes {
		if m.ModalSignificance < 0 || m.ModalSignificance > 1.0+1e-10 {
			t.Errorf("mode %d: MS=%f out of [0,1]", i+1, m.ModalSignificance)
		}
		if m.CharacteristicAngle < 0 || m.CharacteristicAngle > 360 {
			t.Errorf("mode %d: alpha=%f out of [0,360]", i+1, m.CharacteristicAngle)
		}
		// Check current magnitudes
		if len(m.CurrentMagnitudes) == 0 {
			t.Errorf("mode %d: no current magnitudes", i+1)
		}
		// Peak should be normalized to 1.0
		peak := 0.0
		for _, v := range m.CurrentMagnitudes {
			if v > peak {
				peak = v
			}
		}
		if peak > 0 && math.Abs(peak-1.0) > 1e-10 {
			t.Errorf("mode %d: peak current magnitude = %f, want 1.0", i+1, peak)
		}
	}

	// Verify sorted by decreasing MS
	for i := 1; i < len(result.Modes); i++ {
		if result.Modes[i].ModalSignificance > result.Modes[i-1].ModalSignificance+1e-12 {
			t.Errorf("modes not sorted: MS[%d]=%f > MS[%d]=%f",
				i, result.Modes[i].ModalSignificance,
				i-1, result.Modes[i-1].ModalSignificance)
		}
	}

	// Re-index should be 1-based contiguous
	for i, m := range result.Modes {
		if m.Index != i+1 {
			t.Errorf("mode %d: Index=%d, want %d", i, m.Index, i+1)
		}
	}

	t.Logf("CMA found %d modes; top 3 MS: %.4f, %.4f, %.4f",
		result.NumModes,
		result.Modes[0].ModalSignificance,
		safeMS(result.Modes, 1),
		safeMS(result.Modes, 2))
}

// TestCMA_ResonantAngle checks that the dominant mode of a resonant dipole
// has characteristic angle near 180° (resonance condition: λ ≈ 0 → α ≈ 180°).
func TestCMA_ResonantAngle(t *testing.T) {
	freq := 14.0e6
	halfLen := 0.5 * C0 / freq / 2.0

	input := SimulationInput{
		Wires: []Wire{
			{X1: 0, Y1: 0, Z1: -halfLen, X2: 0, Y2: 0, Z2: halfLen, Radius: 0.001, Segments: 21},
		},
		Frequency: freq,
		Ground:    GroundConfig{Type: "free_space"},
		Source:    Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
	}

	result, err := SimulateCMA(input)
	if err != nil {
		t.Fatalf("SimulateCMA failed: %v", err)
	}

	// The dominant mode of a resonant dipole should have α near 180°
	mode1 := result.Modes[0]
	if math.Abs(mode1.CharacteristicAngle-180.0) > 30.0 {
		t.Errorf("dominant mode characteristic angle = %.1f°, expected near 180° for resonant dipole",
			mode1.CharacteristicAngle)
	}

	t.Logf("Dominant mode: λ=%.4f, MS=%.4f, α=%.1f°",
		mode1.Eigenvalue, mode1.ModalSignificance, mode1.CharacteristicAngle)
}

func safeMS(modes []CMAMode, idx int) float64 {
	if idx < len(modes) {
		return modes[idx].ModalSignificance
	}
	return 0
}
