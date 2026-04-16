package mom

import (
	"math"
	"testing"
)

func TestSkinDepth_CopperAt14MHz(t *testing.T) {
	// δ = 1/√(πfμσ).  For copper at 14 MHz: ≈ 17.6 µm.
	mat, _ := LookupMaterial(MaterialCopper)
	d := SkinDepth(mat, 14e6)
	want := 17.6e-6
	if math.Abs(d-want)/want > 0.05 {
		t.Fatalf("Cu skin depth @ 14 MHz: got %.3e m, want ≈ %.3e m", d, want)
	}
}

func TestSurfaceResistance_CopperAt14MHz(t *testing.T) {
	// R_s = √(πfμ/σ).  For copper at 14 MHz: ≈ 9.78e-4 Ω/□.
	mat, _ := LookupMaterial(MaterialCopper)
	rs := SurfaceResistance(mat, 14e6)
	want := 9.78e-4
	if math.Abs(rs-want)/want > 0.05 {
		t.Fatalf("Cu R_s @ 14 MHz: got %.3e Ω/□, want ≈ %.3e Ω/□", rs, want)
	}
}

func TestSkinDepth_PECInfinite(t *testing.T) {
	if d := SkinDepth(Material{Sigma: math.Inf(1), MuR: 1}, 14e6); !math.IsInf(d, 1) {
		t.Fatalf("PEC skin depth should be +Inf, got %v", d)
	}
	if rs := SurfaceResistance(Material{Sigma: math.Inf(1), MuR: 1}, 14e6); rs != 0 {
		t.Fatalf("PEC surface resistance should be 0, got %v", rs)
	}
}

func TestLookupMaterial_EmptyIsPEC(t *testing.T) {
	m, ok := LookupMaterial("")
	if !ok {
		t.Fatal("empty material should resolve to PEC")
	}
	if !math.IsInf(m.Sigma, 1) {
		t.Fatalf("PEC should have infinite sigma, got %v", m.Sigma)
	}
}

func TestLookupMaterial_Unknown(t *testing.T) {
	if _, ok := LookupMaterial("unobtainium"); ok {
		t.Fatal("unknown material should return ok=false")
	}
}

// End-to-end: a 14-MHz dipole made of steel (high resistivity) should
// have a measurably higher feed-point resistance than a copper dipole
// of identical geometry, because skin-effect surface resistance adds
// linearly to the radiation resistance.
func TestSimulate_MaterialRaisesResistance(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	build := func(mat MaterialName) SimulationInput {
		return SimulationInput{
			Frequency: freq,
			Wires: []Wire{{
				X1: 0, Y1: -halfL, Z1: 0,
				X2: 0, Y2: halfL, Z2: 0,
				Radius:   1e-3,
				Segments: 21,
				Material: mat,
			}},
			Source: Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
			Ground: GroundConfig{Type: "free_space"},
		}
	}
	pec, err := Simulate(build(MaterialPEC))
	if err != nil {
		t.Fatalf("PEC simulate: %v", err)
	}
	cu, err := Simulate(build(MaterialCopper))
	if err != nil {
		t.Fatalf("Cu simulate: %v", err)
	}
	steel, err := Simulate(build(MaterialSteel))
	if err != nil {
		t.Fatalf("steel simulate: %v", err)
	}
	// Copper barely changes the impedance of a 1 mm wire at 14 MHz
	// (R_s ≈ 1 mΩ/□, segment loss ~1 mΩ each).  Steel has μ_r ≈ 1000,
	// so R_s is ~26× higher than copper and the impedance shift is
	// readily measurable.
	dCu := cu.Impedance.R - pec.Impedance.R
	dSt := steel.Impedance.R - pec.Impedance.R
	if dCu < 0 {
		t.Fatalf("Cu should not reduce R: ΔR=%v", dCu)
	}
	if dSt <= dCu {
		t.Fatalf("steel should add more loss than Cu: ΔR_steel=%v, ΔR_Cu=%v", dSt, dCu)
	}
	t.Logf("PEC R=%.3f, Cu R=%.3f (ΔR=%.3f), steel R=%.3f (ΔR=%.3f)",
		pec.Impedance.R, cu.Impedance.R, dCu, steel.Impedance.R, dSt)
}

// Material loss should pull radiation efficiency below 1.
func TestSimulate_MaterialReducesEfficiency(t *testing.T) {
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
			Material: MaterialSteel,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}
	res, err := Simulate(geom)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if res.Metrics.RadiationEfficiency >= 1.0 {
		t.Fatalf("steel dipole efficiency should be < 1, got %v", res.Metrics.RadiationEfficiency)
	}
	if res.Metrics.RadiationEfficiency <= 0 {
		t.Fatalf("efficiency should still be positive, got %v", res.Metrics.RadiationEfficiency)
	}
}
