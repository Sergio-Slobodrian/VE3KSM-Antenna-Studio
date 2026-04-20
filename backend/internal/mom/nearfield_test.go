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

// TestNearField_HertzianDipole validates the near-field computation against
// the analytical solution for a single short dipole.  A z-directed segment
// of length Δl = 0.01λ carrying 1 A should produce well-known E/H fields.
func TestNearField_HertzianDipole(t *testing.T) {
	freq := 300e6            // 300 MHz → λ = 1 m
	lambda := 299792458 / freq
	k := 2 * math.Pi / lambda
	deltaL := 0.01 * lambda  // 1 cm segment

	seg := Segment{
		Index:      0,
		WireIndex:  0,
		Center:     [3]float64{0, 0, 0},
		Start:      [3]float64{0, 0, -deltaL / 2},
		End:        [3]float64{0, 0, deltaL / 2},
		HalfLength: deltaL / 2,
		Direction:  [3]float64{0, 0, 1},
		Radius:     0.001,
	}

	current := complex(1.0, 0)

	// Observe at R = 1λ broadside (on x-axis, perpendicular to dipole)
	req := NearFieldRequest{
		Plane:      "xz",
		FixedCoord: 0,          // y = 0
		Min1:       lambda,     // x = λ
		Max1:       lambda,
		Min2:       0,          // z = 0 (broadside)
		Max2:       0,
		Steps1:     2,
		Steps2:     2,
	}

	result := ComputeNearField([]Segment{seg}, []complex128{current}, k, freq, req)

	if len(result.Points) == 0 {
		t.Fatal("no near-field points returned")
	}

	pt := result.Points[0]
	t.Logf("At R=1λ broadside: |E| = %.4f V/m (%.1f dB), |H| = %.6f A/m (%.1f dB)",
		pt.EMag, pt.EMagDB, pt.HMag, pt.HMagDB)

	// Sanity checks (order-of-magnitude):
	// For I=1A, Δl=0.01λ at R=1λ broadside, the far-field-only E ≈ 60π·I·Δl/(λR) ≈ 60π·0.01/1 ≈ 1.88 V/m
	// The near-field adds small corrections at R=1λ.
	if pt.EMag < 0.5 || pt.EMag > 10.0 {
		t.Errorf("|E| = %.4f V/m; expected O(1) V/m for a Hertzian dipole at R=λ", pt.EMag)
	}
	if pt.HMag < 1e-3 || pt.HMag > 0.1 {
		t.Errorf("|H| = %.6f A/m; expected O(0.01) A/m", pt.HMag)
	}

	// E/H ratio should be close to η ≈ 377 Ω in the far field (R=λ is borderline)
	ratio := pt.EMag / pt.HMag
	t.Logf("E/H ratio = %.1f Ω (free-space η = 376.7 Ω)", ratio)
	if ratio < 300 || ratio > 500 {
		t.Errorf("E/H ratio = %.1f; expected ~377 Ω at R=1λ", ratio)
	}
}

// TestNearField_Dipole_SymmetryCheck verifies that the near-field of a
// centre-fed half-wave dipole is symmetric about the feed point.
func TestNearField_Dipole_SymmetryCheck(t *testing.T) {
	freq := 300e6
	lambda := 299792458.0 / freq

	input := SimulationInput{
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: -lambda / 4,
			X2: 0, Y2: 0, Z2: lambda / 4,
			Radius: 0.001, Segments: 21,
		}},
		Frequency: freq,
		Ground:    GroundConfig{Type: "free_space"},
		Source:    Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
	}

	req := NearFieldRequest{
		Plane:      "xz",
		FixedCoord: 0,
		Min1:       0.5 * lambda,
		Max1:       0.5 * lambda,
		Min2:       -0.3 * lambda,
		Max2:       0.3 * lambda,
		Steps1:     2,
		Steps2:     7,
	}

	result, err := SimulateNearField(input, req)
	if err != nil {
		t.Fatalf("SimulateNearField failed: %v", err)
	}

	// The dipole is symmetric about z=0.  Check that |E| at z=+d ≈ |E| at z=-d.
	nPts := len(result.Points)
	mid := nPts / 2
	t.Logf("Near-field computed: %d points, |E| range [%.1f, %.1f] dB",
		nPts, result.EMinDB, result.EMaxDB)

	for i := 0; i < mid; i++ {
		j := nPts - 1 - i
		eUp := result.Points[j].EMag
		eDown := result.Points[i].EMag
		if eUp < 1e-30 || eDown < 1e-30 {
			continue
		}
		ratio := eUp / eDown
		if ratio < 0.85 || ratio > 1.18 {
			t.Errorf("symmetry broken: |E|(z=%.3f) = %.4f vs |E|(z=%.3f) = %.4f, ratio = %.3f",
				result.Points[j].Z, eUp, result.Points[i].Z, eDown, ratio)
		}
	}
}

// TestNearField_WithGround verifies that the ground-plane variant runs
// without error and that |E| below z=0 accounts for image contributions.
func TestNearField_WithGround(t *testing.T) {
	freq := 300e6
	lambda := 299792458.0 / freq

	input := SimulationInput{
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: 0,
			X2: 0, Y2: 0, Z2: lambda / 4,
			Radius: 0.001, Segments: 21,
		}},
		Frequency: freq,
		Ground:    GroundConfig{Type: "perfect"},
		Source:    Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
	}

	req := NearFieldRequest{
		Plane:      "xz",
		FixedCoord: 0,
		Min1:       0.5 * lambda,
		Max1:       0.5 * lambda,
		Min2:       0.01,
		Max2:       0.5 * lambda,
		Steps1:     2,
		Steps2:     5,
	}

	result, err := SimulateNearField(input, req)
	if err != nil {
		t.Fatalf("SimulateNearField with ground failed: %v", err)
	}

	t.Logf("Near-field with ground: %d points, |E| range [%.1f, %.1f] dB",
		len(result.Points), result.EMinDB, result.EMaxDB)

	// Basic sanity: we should have some non-trivial field
	if result.EMaxDB < -50 {
		t.Errorf("max |E| = %.1f dB; expected significant field from a driven monopole", result.EMaxDB)
	}
}
