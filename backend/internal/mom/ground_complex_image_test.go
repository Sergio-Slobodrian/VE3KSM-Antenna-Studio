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

func TestComplexSkinDepth_PerfectConductor(t *testing.T) {
	// For very high conductivity, |εc| is large and skin depth → 0.
	k0 := 2 * math.Pi * 14e6 / C0
	epsilonC := ComplexPermittivity(13, 1e6, 2*math.Pi*14e6)
	d := ComplexSkinDepth(k0, epsilonC)
	if cmplx.Abs(d) > 0.01 {
		t.Errorf("skin depth for near-perfect conductor should be tiny, got |d| = %e", cmplx.Abs(d))
	}
	t.Logf("high-σ skin depth: %v (|d| = %e m)", d, cmplx.Abs(d))
}

func TestComplexSkinDepth_TypicalGround(t *testing.T) {
	// Average ground: σ=0.005, εr=13, at 14 MHz.
	// Expected skin depth ≈ 1-2 metres.
	freq := 14e6
	omega := 2 * math.Pi * freq
	k0 := omega / C0
	epsilonC := ComplexPermittivity(13, 0.005, omega)
	d := ComplexSkinDepth(k0, epsilonC)

	// |d| should be on the order of metres, not millimetres or kilometres.
	absD := cmplx.Abs(d)
	if absD < 0.1 || absD > 50 {
		t.Errorf("skin depth for average ground at 14 MHz should be ~metres, got |d| = %f m", absD)
	}
	t.Logf("average ground skin depth at 14 MHz: %v (|d| = %.3f m)", d, absD)
}

func TestComplexImage_PenetrationDepthSign(t *testing.T) {
	// Verify that for typical lossy ground the penetration depth (real part
	// of skin depth) is positive, meaning the effective image is pushed
	// deeper below the surface.
	freq := 14e6
	omega := 2 * math.Pi * freq
	k0 := omega / C0
	epsilonC := ComplexPermittivity(13, 0.005, omega)
	d := ComplexSkinDepth(k0, epsilonC)

	if real(d) <= 0 {
		t.Errorf("penetration depth should be positive for lossy ground, got Re(d) = %f", real(d))
	}
	t.Logf("skin depth = %v, penetration depth = %.3f m", d, real(d))
}

func TestGroundPropagationConst(t *testing.T) {
	// For lossy ground, α = Re(γ) should be positive (attenuation).
	freq := 14e6
	omega := 2 * math.Pi * freq
	k0 := omega / C0
	epsilonC := ComplexPermittivity(13, 0.005, omega)
	gamma := GroundPropagationConst(k0, epsilonC)

	if real(gamma) <= 0 {
		t.Errorf("attenuation constant α should be positive, got Re(γ) = %f", real(gamma))
	}
	t.Logf("γ_g = %v, α = %.4f Np/m", gamma, real(gamma))
}

// TestComplexImage_HighConductivity verifies that for very high
// conductivity the complex-image model converges to the perfect-
// ground result (skin depth → 0, scale → 1, R_eff → +1/-1).
func TestComplexImage_HighConductivity(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4

	// Quarter-wave vertical monopole with base at z=0 (ground-connected)
	// and very high conductivity ground.
	inputHighSigma := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: 0,
			X2: 0, Y2: 0, Z2: halfL,
			Radius: 1e-3, Segments: 21,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 0, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "real", Conductivity: 1e4, Permittivity: 13},
	}

	// Perfect ground reference.
	inputPerfect := inputHighSigma
	inputPerfect.Ground = GroundConfig{Type: "perfect"}

	resReal, err := Simulate(inputHighSigma)
	if err != nil {
		t.Fatalf("high-σ real ground: %v", err)
	}
	resPerfect, err := Simulate(inputPerfect)
	if err != nil {
		t.Fatalf("perfect ground: %v", err)
	}

	// They should agree within ~20% (complex-image method should converge
	// to perfect ground as σ → ∞).
	relR := math.Abs(resReal.Impedance.R-resPerfect.Impedance.R) / (math.Abs(resPerfect.Impedance.R) + 1e-10)
	relX := math.Abs(resReal.Impedance.X-resPerfect.Impedance.X) / (math.Abs(resPerfect.Impedance.X) + 1e-10)
	t.Logf("perfect: Z = %.2f %+.2fj Ω", resPerfect.Impedance.R, resPerfect.Impedance.X)
	t.Logf("high-σ:  Z = %.2f %+.2fj Ω", resReal.Impedance.R, resReal.Impedance.X)
	t.Logf("rel diff: R=%.1f%%, X=%.1f%%", relR*100, relX*100)

	if relR > 0.20 {
		t.Errorf("R mismatch > 20%%: perfect=%.2f, high-σ=%.2f", resPerfect.Impedance.R, resReal.Impedance.R)
	}
	// X can differ more near resonance, allow 30%.
	if relX > 0.30 {
		t.Errorf("X mismatch > 30%%: perfect=%.2f, high-σ=%.2f", resPerfect.Impedance.X, resReal.Impedance.X)
	}
}

// TestComplexImage_LossyGround verifies a monopole over average ground
// gives physically reasonable impedance (R should be positive and in
// the expected range for a quarter-wave vertical).
func TestComplexImage_LossyGround(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq

	input := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: 0,
			X2: 0, Y2: 0, Z2: wavelength / 4,
			Radius: 1e-3, Segments: 21,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 0, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "real", Conductivity: 0.005, Permittivity: 13},
	}

	res, err := Simulate(input)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}

	t.Logf("Z_in = %.2f %+.2fj Ω, SWR = %.2f", res.Impedance.R, res.Impedance.X, res.SWR)

	// A quarter-wave vertical over average ground should have:
	// R ≈ 30-60 Ω (ground loss adds to the radiation resistance)
	// |X| < 100 Ω (near resonance)
	if res.Impedance.R < 5 || res.Impedance.R > 200 {
		t.Errorf("R = %.2f Ω — outside reasonable range [5, 200]", res.Impedance.R)
	}
	if math.Abs(res.Impedance.X) > 200 {
		t.Errorf("|X| = %.2f Ω — too large", math.Abs(res.Impedance.X))
	}
}
