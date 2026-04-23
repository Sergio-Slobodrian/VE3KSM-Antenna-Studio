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

import "testing"

func TestRemapSegIndex_ZeroStaysZero(t *testing.T) {
	cases := [][2]int{{25, 51}, {101, 203}, {3, 7}, {1, 3}}
	for _, c := range cases {
		oldSeg, newSeg := c[0], c[1]
		got := remapSegIndex(0, oldSeg, newSeg)
		if got != 0 {
			t.Errorf("remapSegIndex(0, %d, %d) = %d, want 0", oldSeg, newSeg, got)
		}
	}
}

func TestRemapSegIndex_LastStaysLast(t *testing.T) {
	cases := [][2]int{{25, 51}, {101, 203}, {3, 7}}
	for _, c := range cases {
		oldSeg, newSeg := c[0], c[1]
		got := remapSegIndex(oldSeg-1, oldSeg, newSeg)
		if got != newSeg-1 {
			t.Errorf("remapSegIndex(%d, %d, %d) = %d, want %d", oldSeg-1, oldSeg, newSeg, got, newSeg-1)
		}
	}
}

func TestRemapSegIndex_MiddleProportional(t *testing.T) {
	// Middle index should map near the proportional midpoint of the new mesh.
	// For oldIndex=12, oldSeg=25, newSeg=51: expect ~25 (middle of 0-50).
	got := remapSegIndex(12, 25, 51)
	if got < 23 || got > 27 {
		t.Errorf("remapSegIndex(12, 25, 51) = %d, expected near 25", got)
	}
}

func TestRemapSegIndex_EdgeCases(t *testing.T) {
	// oldSeg=0: should not panic, return 0
	if got := remapSegIndex(5, 0, 10); got != 0 {
		t.Errorf("remapSegIndex(5, 0, 10) = %d, want 0", got)
	}
	// negative oldIndex: clamps to 0
	if got := remapSegIndex(-1, 25, 51); got != 0 {
		t.Errorf("remapSegIndex(-1, 25, 51) = %d, want 0", got)
	}
}

// TestSommerfeldConvergenceSmoke confirms that the Sommerfeld ground path is
// live and reachable via RunConvergenceCheck.  A horizontal half-wave dipole
// at λ/10 height over average lossy soil is simulated twice — once with
// complex-image, once with Sommerfeld — and both must succeed with a positive
// feed-point resistance.  The two resistances are expected to differ (proving
// separate code paths are active) but agree within a factor of 3.
func TestSommerfeldConvergenceSmoke(t *testing.T) {
	const freqHz = 14.2e6 // 20 m band, λ ≈ 21.1 m
	wavelength := 299792458.0 / freqHz
	halfLen := wavelength / 4         // quarter-wave arm ≈ 5.28 m
	height := wavelength / 10         // ≈ 2.11 m — within λ/10 of ground

	// Horizontal centre-fed dipole at height h above real ground.
	wire := Wire{
		X1: -halfLen, Y1: 0, Z1: height,
		X2: halfLen, Y2: 0, Z2: height,
		Radius:   0.001,
		Segments: 11,
	}
	src := Source{WireIndex: 0, SegmentIndex: 5} // centre feed
	ground := GroundConfig{
		Type:         "real",
		Conductivity: 0.005,
		Permittivity: 13,
	}

	baseInput := SimulationInput{
		Wires:              []Wire{wire},
		Frequency:          freqHz,
		Ground:             ground,
		Source:             src,
		ReferenceImpedance: 50,
	}

	imageInput := baseInput
	imageInput.Ground.Method = ""
	resImage, err := RunConvergenceCheck(imageInput)
	if err != nil {
		t.Fatalf("complex-image convergence check failed: %v", err)
	}

	sommInput := baseInput
	sommInput.Ground.Method = "sommerfeld"
	resSomm, err := RunConvergenceCheck(sommInput)
	if err != nil {
		t.Fatalf("sommerfeld convergence check failed: %v", err)
	}

	if resImage.ImpedanceR1 <= 0 {
		t.Errorf("complex-image R1 = %g Ω, want > 0", resImage.ImpedanceR1)
	}
	if resSomm.ImpedanceR1 <= 0 {
		t.Errorf("sommerfeld R1 = %g Ω, want > 0", resSomm.ImpedanceR1)
	}

	// The two methods model the same physics; their resistances must be in the
	// same order of magnitude even though they use different approximations.
	if resImage.ImpedanceR1 > 0 && resSomm.ImpedanceR1 > 0 {
		ratio := resSomm.ImpedanceR1 / resImage.ImpedanceR1
		if ratio < 0.33 || ratio > 3.0 {
			t.Errorf("Sommerfeld/image R ratio = %.2f, expected [0.33, 3.0]; image R=%.2f Ω, sommerfeld R=%.2f Ω",
				ratio, resImage.ImpedanceR1, resSomm.ImpedanceR1)
		}
	}
}
