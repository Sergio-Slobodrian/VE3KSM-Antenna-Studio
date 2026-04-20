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
	"math/cmplx"
	"testing"
)

// closeC returns true when two complex numbers agree to within tol in both
// the real and imaginary parts.  Used in lieu of cmplx.Abs(a-b) so that a
// small imaginary error in a near-real value still fails as expected.
func closeC(a, b complex128, tol float64) bool {
	return math.Abs(real(a)-real(b)) <= tol && math.Abs(imag(a)-imag(b)) <= tol
}

func TestLoadImpedance_PureResistor(t *testing.T) {
	// 50 Ω terminator at any frequency should look like 50 + 0j.
	z, err := LoadImpedance(Load{Topology: LoadSeriesRLC, R: 50}, 2*math.Pi*14e6)
	if err != nil {
		t.Fatal(err)
	}
	if !closeC(z, 50+0i, 1e-9) {
		t.Fatalf("pure 50 Ω terminator: got %v, want 50+0i", z)
	}
}

func TestLoadImpedance_PureInductor(t *testing.T) {
	// 1 µH at 14 MHz: X_L = 2πfL = 2π·14e6·1e-6 ≈ 87.96 Ω
	omega := 2 * math.Pi * 14e6
	z, err := LoadImpedance(Load{Topology: LoadSeriesRLC, L: 1e-6}, omega)
	if err != nil {
		t.Fatal(err)
	}
	want := complex(0, omega*1e-6)
	if !closeC(z, want, 1e-9) {
		t.Fatalf("1 µH @ 14 MHz: got %v, want %v", z, want)
	}
}

func TestLoadImpedance_PureCapacitor(t *testing.T) {
	// 100 pF at 14 MHz: X_C = -1/(2πfC) ≈ -113.7 Ω
	omega := 2 * math.Pi * 14e6
	z, err := LoadImpedance(Load{Topology: LoadSeriesRLC, C: 100e-12}, omega)
	if err != nil {
		t.Fatal(err)
	}
	want := complex(0, -1/(omega*100e-12))
	if !closeC(z, want, 1e-9) {
		t.Fatalf("100 pF @ 14 MHz: got %v, want %v", z, want)
	}
}

func TestLoadImpedance_SeriesResonance(t *testing.T) {
	// At series-LC resonance the reactances cancel, leaving Z = R.
	// Choose L = 1 µH, C = 100 pF → ω0 = 1/√(LC) ≈ 1.0e8 rad/s.
	L, C, R := 1e-6, 100e-12, 12.0
	omega0 := 1 / math.Sqrt(L*C)
	z, err := LoadImpedance(
		Load{Topology: LoadSeriesRLC, R: R, L: L, C: C},
		omega0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !closeC(z, complex(R, 0), 1e-3) {
		t.Fatalf("series LC at resonance: got %v, want %v+0i", z, R)
	}
}

func TestLoadImpedance_ParallelResonance(t *testing.T) {
	// Parallel LC at ω0 has |Z| → ∞ in the loss-free limit; with a
	// parallel R the magnitude collapses to R.  Use the same L, C as
	// above so ω0 is identical.
	L, C, R := 1e-6, 100e-12, 5000.0
	omega0 := 1 / math.Sqrt(L*C)
	z, err := LoadImpedance(
		Load{Topology: LoadParallelRLC, R: R, L: L, C: C},
		omega0,
	)
	if err != nil {
		t.Fatal(err)
	}
	// At resonance Y = 1/R + 0 (the L and C admittances cancel), so Z = R.
	if !closeC(z, complex(R, 0), 1e-3) {
		t.Fatalf("parallel RLC at resonance: got %v, want %v+0i", z, R)
	}
}

func TestLoadImpedance_ParallelAllZero(t *testing.T) {
	_, err := LoadImpedance(Load{Topology: LoadParallelRLC}, 1e8)
	if err == nil {
		t.Fatal("expected error for parallel_rlc with R=L=C=0")
	}
}

func TestLoadImpedance_UnknownTopology(t *testing.T) {
	_, err := LoadImpedance(Load{Topology: "weird"}, 1e8)
	if err == nil {
		t.Fatal("expected error for unknown topology")
	}
}

// TestSimulate_PureResistorChangesImpedance checks the end-to-end behaviour
// of the lumped-load injection: adding a 50 Ω resistor in series with a
// dipole at its feed segment should bump the real part of the input
// impedance by very nearly 50 Ω (the load lives on the same basis function
// as the source, so it appears in series with the radiation resistance).
func TestSimulate_PureResistorChangesImpedance(t *testing.T) {
	// Reuse the existing half-wave dipole geometry from solver_test.go
	// shape: a centre-fed λ/2 dipole at 14 MHz.
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4

	geom := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: -halfL, Z1: 0,
			X2: 0, Y2: halfL, Z2: 0,
			Radius:   1e-3,
			Segments: 21,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}

	// Baseline (no load).
	base, err := Simulate(geom)
	if err != nil {
		t.Fatalf("baseline simulate: %v", err)
	}

	// Add a 50 Ω series resistor at the feed segment.
	geom.Loads = []Load{{
		WireIndex:    0,
		SegmentIndex: 10,
		Topology:     LoadSeriesRLC,
		R:            50,
	}}
	loaded, err := Simulate(geom)
	if err != nil {
		t.Fatalf("loaded simulate: %v", err)
	}

	dR := loaded.Impedance.R - base.Impedance.R
	dX := loaded.Impedance.X - base.Impedance.X
	// The load injects 50 Ω onto the feed-basis diagonal.  In the
	// idealised limit ΔR = 50 Ω and ΔX = 0; numerically the basis-
	// function-spread coupling means we should still see ΔR within
	// ±15 % of 50 Ω with a 21-segment discretisation.
	if math.Abs(dR-50) > 7.5 {
		t.Fatalf("expected ΔR ≈ 50 Ω, got %.3f Ω (base R=%.2f, loaded R=%.2f)",
			dR, base.Impedance.R, loaded.Impedance.R)
	}
	if math.Abs(dX) > 7.5 {
		t.Fatalf("expected ΔX ≈ 0, got %.3f Ω (base X=%.2f, loaded X=%.2f)",
			dX, base.Impedance.X, loaded.Impedance.X)
	}
	t.Logf("ΔR=%.2f Ω, ΔX=%.2f Ω (base Z=%.2f%+.2fj, loaded Z=%.2f%+.2fj)",
		dR, dX, base.Impedance.R, base.Impedance.X,
		loaded.Impedance.R, loaded.Impedance.X)
}

// TestSimulate_LoadOutOfRange verifies the solver surfaces a clear error
// when a load points at a non-existent wire or segment.
func TestSimulate_LoadOutOfRange(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	geom := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: -halfL, Z1: 0,
			X2: 0, Y2: halfL, Z2: 0,
			Radius:   1e-3,
			Segments: 11,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
		Loads: []Load{{
			WireIndex:    1, // out of range
			SegmentIndex: 0,
			Topology:     LoadSeriesRLC,
			R:            50,
		}},
	}
	_, err := Simulate(geom)
	if err == nil {
		t.Fatal("expected error for out-of-range load wire_index")
	}
}

// Sanity check the helper: cmplx.Abs of a freshly-built impedance should
// be deterministic and non-zero, guarding against future refactors.
func TestLoadImpedance_FiniteAtNonResonance(t *testing.T) {
	z, err := LoadImpedance(
		Load{Topology: LoadSeriesRLC, R: 10, L: 1e-6, C: 100e-12},
		2*math.Pi*5e6,
	)
	if err != nil {
		t.Fatal(err)
	}
	if cmplx.Abs(z) == 0 {
		t.Fatal("series RLC away from resonance should have non-zero |Z|")
	}
}
