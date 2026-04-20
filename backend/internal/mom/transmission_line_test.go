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

// A quarter-wave shorted stub at its design frequency presents an open
// circuit at its input: |Z_in| → very large.  Using a 50 Ω Z0, λ/4 at
// 14 MHz is ~5.36 m of lossless line.
func TestTLImpedance_QuarterWaveShortedStubLooksOpen(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	tl := TransmissionLine{
		A:              TLEnd{WireIndex: 0, SegmentIndex: 0},
		B:              TLEnd{WireIndex: TLEndShorted, SegmentIndex: 0},
		Z0:             50,
		Length:         wavelength / 4,
		VelocityFactor: 1.0,
	}
	z11, z12, err := TLImpedanceMatrix(tl, 2*math.Pi*freq)
	if err != nil {
		t.Fatal(err)
	}
	if z12 != 0 {
		t.Fatalf("stub Z12 should be 0 (one-port), got %v", z12)
	}
	mag := cmplx.Abs(z11)
	if mag < 1e6 {
		t.Fatalf("λ/4 shorted stub should look ~open at f0, |Z| got %.3g Ω", mag)
	}
}

// A half-wave shorted stub at f0 looks like a short: |Z_in| → 0.
func TestTLImpedance_HalfWaveShortedStubLooksShort(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	tl := TransmissionLine{
		A:              TLEnd{WireIndex: 0, SegmentIndex: 0},
		B:              TLEnd{WireIndex: TLEndShorted, SegmentIndex: 0},
		Z0:             50,
		Length:         wavelength / 2,
		VelocityFactor: 1.0,
	}
	z11, _, err := TLImpedanceMatrix(tl, 2*math.Pi*freq)
	if err != nil {
		t.Fatal(err)
	}
	mag := cmplx.Abs(z11)
	if mag > 1e-3 {
		t.Fatalf("λ/2 shorted stub should look ~short at f0, |Z| got %.3g Ω", mag)
	}
}

// An eighth-wave shorted stub (electrical length 45°) on a 50 Ω line
// presents a positive imaginary input impedance equal to Z0 = +j50.
func TestTLImpedance_EighthWaveStubInductive(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	tl := TransmissionLine{
		A:              TLEnd{WireIndex: 0, SegmentIndex: 0},
		B:              TLEnd{WireIndex: TLEndShorted, SegmentIndex: 0},
		Z0:             50,
		Length:         wavelength / 8,
		VelocityFactor: 1.0,
	}
	z11, _, err := TLImpedanceMatrix(tl, 2*math.Pi*freq)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(real(z11)) > 1e-9 {
		t.Fatalf("lossless λ/8 stub should be purely reactive, Re(Z) = %v", real(z11))
	}
	if math.Abs(imag(z11)-50) > 1e-6 {
		t.Fatalf("lossless λ/8 shorted stub: expected +j50, got %v", z11)
	}
}

// Velocity factor < 1 stretches the electrical length: a stub with
// physical length λ_air/4 but VF=0.66 looks much shorter than λ/4
// electrically and is no longer near resonance.
func TestTLImpedance_VelocityFactorShortensElectricalLength(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	long := TransmissionLine{
		A:              TLEnd{WireIndex: 0, SegmentIndex: 0},
		B:              TLEnd{WireIndex: TLEndShorted, SegmentIndex: 0},
		Z0:             50,
		Length:         wavelength / 4,
		VelocityFactor: 1.0,
	}
	withVF := long
	withVF.VelocityFactor = 0.66
	zLong, _, _ := TLImpedanceMatrix(long, 2*math.Pi*freq)
	zVF, _, _ := TLImpedanceMatrix(withVF, 2*math.Pi*freq)
	if cmplx.Abs(zLong) <= cmplx.Abs(zVF) {
		t.Fatalf("VF=1 (resonant) should yield larger |Z| than VF=0.66; got %v vs %v",
			cmplx.Abs(zLong), cmplx.Abs(zVF))
	}
}

// A two-port (non-stub) λ/4 line transforms 100 Ω to 25 Ω when Z0 = 50:
// Z_in = Z0² / Z_load.  We can't test that directly without a load, but
// we can verify the symmetric Z11 = Z22 = 0 (-jZ0·cot(π/2)) and
// |Z12| = Z0 / |sin(π/2)| = Z0 properties.
func TestTLImpedance_QuarterWaveTwoPortIdentities(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	tl := TransmissionLine{
		A:              TLEnd{WireIndex: 0, SegmentIndex: 0},
		B:              TLEnd{WireIndex: 1, SegmentIndex: 0},
		Z0:             50,
		Length:         wavelength / 4,
		VelocityFactor: 1.0,
	}
	z11, z12, err := TLImpedanceMatrix(tl, 2*math.Pi*freq)
	if err != nil {
		t.Fatal(err)
	}
	// Lossless λ/4 → Z11 = -j·Z0·cot(π/2) = 0 (small imag).
	if cmplx.Abs(z11) > 1e-6 {
		t.Fatalf("λ/4 lossless: Z11 ≈ 0 expected, got %v", z11)
	}
	// |Z12| = Z0
	if math.Abs(cmplx.Abs(z12)-50) > 1e-6 {
		t.Fatalf("λ/4 lossless: |Z12| should be Z0=50, got %v", cmplx.Abs(z12))
	}
}

// Lossy line should produce a Z11 with non-zero real part (resistive).
func TestTLImpedance_LossyLineHasResistivePart(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	tl := TransmissionLine{
		A:              TLEnd{WireIndex: 0, SegmentIndex: 0},
		B:              TLEnd{WireIndex: TLEndShorted, SegmentIndex: 0},
		Z0:             50,
		Length:         wavelength / 4,
		VelocityFactor: 1.0,
		LossDbPerM:     1.0, // very lossy
	}
	z11, _, err := TLImpedanceMatrix(tl, 2*math.Pi*freq)
	if err != nil {
		t.Fatal(err)
	}
	if real(z11) <= 0 {
		t.Fatalf("lossy stub should have Re(Z11) > 0, got %v", z11)
	}
}

// End-to-end: shorted λ/4 stub at the feed of a dipole should massively
// increase the apparent input impedance (the stub looks open in
// parallel with the dipole's ~73 Ω feed impedance).
func TestSimulate_TLStubAtFeedRaisesImpedance(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	build := func(withStub bool) SimulationInput {
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
		if withStub {
			geom.TransmissionLines = []TransmissionLine{{
				A:              TLEnd{WireIndex: 0, SegmentIndex: 10},
				B:              TLEnd{WireIndex: TLEndShorted, SegmentIndex: 0},
				Z0:             50,
				Length:         wavelength / 4,
				VelocityFactor: 1.0,
			}}
		}
		return geom
	}
	base, err := Simulate(build(false))
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	withStub, err := Simulate(build(true))
	if err != nil {
		t.Fatalf("with stub: %v", err)
	}
	// The λ/4 shorted stub looks open at the feed, so it should add a
	// very large series impedance and the magnitude of Z_in should jump
	// by orders of magnitude versus the bare dipole.
	baseMag := math.Hypot(base.Impedance.R, base.Impedance.X)
	stubMag := math.Hypot(withStub.Impedance.R, withStub.Impedance.X)
	if stubMag <= baseMag*5 {
		t.Fatalf("λ/4 shorted stub should dominate Z: |Z_base|=%.2f, |Z_stub|=%.2f", baseMag, stubMag)
	}
}

// Regression: the lossless λ/4 shorted stub formerly returned NaN
// because cmplx.Tanh(jπ/2) overflows.  TLImpedanceMatrix now clamps to
// a large finite value so the linear solver remains stable.
func TestTLImpedance_QuarterWaveStubFiniteAfterClamp(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	tl := TransmissionLine{
		A:              TLEnd{WireIndex: 0, SegmentIndex: 0},
		B:              TLEnd{WireIndex: TLEndShorted, SegmentIndex: 0},
		Z0:             50,
		Length:         wavelength / 4,
		VelocityFactor: 1.0,
	}
	z11, _, err := TLImpedanceMatrix(tl, 2*math.Pi*freq)
	if err != nil {
		t.Fatal(err)
	}
	if math.IsNaN(real(z11)) || math.IsNaN(imag(z11)) ||
		math.IsInf(real(z11), 0) || math.IsInf(imag(z11), 0) {
		t.Fatalf("Z11 should be finite after clamp, got %v", z11)
	}
	if cmplx.Abs(z11) < 1e6 {
		t.Fatalf("clamped Z11 should still be very large, got |Z|=%g", cmplx.Abs(z11))
	}
}

