package mom

import (
	"math"
	"math/cmplx"
	"testing"
)

// A 50 Ω resistive load against a 50 Ω reference is a perfect match:
// Γ = 0, VSWR = 1.
func TestReflection_PerfectMatch(t *testing.T) {
	z := ComplexImpedance{R: 50, X: 0}
	g := ReflectionCoefficient(z, 50)
	if cmplx.Abs(g) > 1e-12 {
		t.Fatalf("expected |Γ| ≈ 0 for matched load, got %v", g)
	}
	if vswr := VSWRFromGamma(g); math.Abs(vswr-1) > 1e-12 {
		t.Fatalf("expected VSWR = 1, got %v", vswr)
	}
}

// A 75 Ω resistive load against a 50 Ω reference: Γ = (75-50)/(75+50) = 0.2,
// VSWR = (1 + 0.2)/(1 - 0.2) = 1.5.
func TestReflection_KnownValue(t *testing.T) {
	z := ComplexImpedance{R: 75, X: 0}
	g := ReflectionCoefficient(z, 50)
	if math.Abs(real(g)-0.2) > 1e-12 || math.Abs(imag(g)) > 1e-12 {
		t.Fatalf("Γ for 75 Ω over 50 Ω: got %v, want 0.2+0i", g)
	}
	if vswr := VSWRFromGamma(g); math.Abs(vswr-1.5) > 1e-12 {
		t.Fatalf("VSWR: got %v, want 1.5", vswr)
	}
}

// Same load measured against two different references gives different VSWRs:
// 75 Ω load against a 75 Ω reference is matched (VSWR = 1).
func TestReflection_ReferenceImpedanceMatters(t *testing.T) {
	z := ComplexImpedance{R: 75, X: 0}
	v50 := VSWRAt(z, 50)
	v75 := VSWRAt(z, 75)
	if math.Abs(v50-1.5) > 1e-12 {
		t.Fatalf("VSWR @ 50 Ω: got %v, want 1.5", v50)
	}
	if math.Abs(v75-1) > 1e-12 {
		t.Fatalf("VSWR @ 75 Ω: got %v, want 1.0", v75)
	}
}

// Z₀ = 0 should fall back to the 50 Ω default rather than divide by zero.
func TestReflection_DefaultZ0Fallback(t *testing.T) {
	z := ComplexImpedance{R: 75, X: 0}
	g0 := ReflectionCoefficient(z, 0)
	g50 := ReflectionCoefficient(z, 50)
	if g0 != g50 {
		t.Fatalf("Z₀=0 should equal Z₀=default; got %v vs %v", g0, g50)
	}
}

// Open and short circuits should map to |Γ| = 1 and the VSWR clamp.
func TestReflection_OpenAndShort(t *testing.T) {
	openZ := ComplexImpedance{R: 1e12, X: 0}
	shortZ := ComplexImpedance{R: 0, X: 0}

	gOpen := ReflectionCoefficient(openZ, 50)
	gShort := ReflectionCoefficient(shortZ, 50)

	if math.Abs(cmplx.Abs(gOpen)-1) > 1e-6 {
		t.Fatalf("open: |Γ| should ≈ 1, got %v", cmplx.Abs(gOpen))
	}
	if math.Abs(cmplx.Abs(gShort)-1) > 1e-12 {
		t.Fatalf("short: |Γ| should = 1, got %v", cmplx.Abs(gShort))
	}
	if VSWRFromGamma(gShort) != 999.0 {
		t.Fatalf("short VSWR should clamp to 999, got %v", VSWRFromGamma(gShort))
	}
}

// End-to-end: a half-wave dipole has roughly Z ≈ 73 + j42 Ω in free
// space.  The VSWR vs 50 Ω should be > 1 and the VSWR vs 73 Ω should
// be lower.  Also the ReferenceImpedance field on the result should
// reflect what we passed in.
func TestSimulate_ReferenceImpedancePassThrough(t *testing.T) {
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

	// Default 50 Ω.
	r50, err := Simulate(geom)
	if err != nil {
		t.Fatalf("simulate @ 50Ω: %v", err)
	}
	if math.Abs(r50.ReferenceImpedance-50) > 1e-12 {
		t.Fatalf("default Z₀ should be 50, got %v", r50.ReferenceImpedance)
	}

	// Override to 75 Ω.
	geom.ReferenceImpedance = 75
	r75, err := Simulate(geom)
	if err != nil {
		t.Fatalf("simulate @ 75Ω: %v", err)
	}
	if math.Abs(r75.ReferenceImpedance-75) > 1e-12 {
		t.Fatalf("Z₀ should be 75, got %v", r75.ReferenceImpedance)
	}
	// Cross-check: VSWR computed solver-side at 75Ω matches the helper.
	wantVSWR := VSWRAt(r75.Impedance, 75)
	if math.Abs(r75.SWR-wantVSWR) > 1e-9 {
		t.Fatalf("solver SWR (%v) ≠ helper VSWR (%v)", r75.SWR, wantVSWR)
	}
	// Sanity: the impedance should be the same regardless of Z₀.
	if math.Abs(r50.Impedance.R-r75.Impedance.R) > 1e-6 ||
		math.Abs(r50.Impedance.X-r75.Impedance.X) > 1e-6 {
		t.Fatalf("Z_in must not depend on reference impedance")
	}
	// And the reflection coefficient should differ.
	g50 := r50.Reflection
	g75 := r75.Reflection
	if cmplx.Abs(g50-g75) < 1e-3 {
		t.Fatalf("Γ should change with Z₀: got identical %v vs %v", g50, g75)
	}
}

// Sweep should populate Reflections in lockstep with Impedance and SWR.
func TestSweep_PopulatesReflections(t *testing.T) {
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
	}
	res, err := Sweep(geom, 13e6, 15e6, 5)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(res.Reflections) != len(res.SWR) {
		t.Fatalf("Reflections length %d ≠ SWR length %d", len(res.Reflections), len(res.SWR))
	}
	if res.ReferenceImpedance != 50 {
		t.Fatalf("default sweep Z₀ should be 50, got %v", res.ReferenceImpedance)
	}
	for i := range res.Reflections {
		want := VSWRFromGamma(res.Reflections[i])
		if math.Abs(want-res.SWR[i]) > 1e-9 {
			t.Fatalf("step %d: VSWR(Γ)=%v ≠ stored SWR=%v", i, want, res.SWR[i])
		}
	}
}

// |Γ| just below 1 used to produce huge unclamped VSWR values
// (e.g. 822039 in real sweeps).  Confirm we now cap at 999.
func TestVSWR_NearOneClampsTo999(t *testing.T) {
	// |Γ| = 0.99999 → naive (1+0.99999)/(1-0.99999) ≈ 200000.
	g := complex(0.99999, 0)
	swr := VSWRFromGamma(g)
	if swr != 999.0 {
		t.Fatalf("expected SWR clamp to 999 for |Γ|≈1, got %v", swr)
	}
}

