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
	"encoding/json"
	"math"
	"testing"
)

// Backward compatibility: JSON round-trip of a Wire with no radius_start /
// radius_end fields must produce a zero-valued taper pair that the solver
// treats identically to the uniform-radius path.  Guards against accidental
// taper fallback clobbering Radius.
func TestTaperBackwardCompatJSON(t *testing.T) {
	raw := []byte(`{"x1":0,"y1":0,"z1":-0.25,"x2":0,"y2":0,"z2":0.25,"radius":0.001,"segments":11}`)
	var w Wire
	if err := json.Unmarshal(raw, &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if w.RadiusStart != 0 || w.RadiusEnd != 0 {
		t.Fatalf("expected RadiusStart/RadiusEnd zero when absent from JSON, got %g / %g",
			w.RadiusStart, w.RadiusEnd)
	}
	if w.isTapered() {
		t.Fatalf("uniform wire reported as tapered")
	}
	rS, rE := w.taperRadii()
	if rS != w.Radius || rE != w.Radius {
		t.Fatalf("uniform wire taperRadii returned (%g,%g); want (%g,%g)", rS, rE, w.Radius, w.Radius)
	}
}

// SubdivideWire(..., r, r, n) must produce segments whose Radius field is
// exactly r (bit-identical to the pre-item-19 constant assignment).
func TestSubdivideWireUniformRadiusIsBitIdentical(t *testing.T) {
	r := 0.00375
	segs := SubdivideWire(0, 0, 0, 0, 0, 0, 1.0, r, r, 9)
	if len(segs) != 9 {
		t.Fatalf("expected 9 segments, got %d", len(segs))
	}
	for i, s := range segs {
		if s.Radius != r {
			t.Errorf("segment %d: Radius = %v, want exactly %v (bit-identical)", i, s.Radius, r)
		}
	}
}

// Linear interpolation check: a tapered wire's segment centres should carry
// a radius that matches rStart + tCenter * (rEnd - rStart).
func TestSubdivideWireTaperedInterpolation(t *testing.T) {
	rS, rE := 0.001, 0.004
	n := 8
	segs := SubdivideWire(0, 0, 0, 0, 0, 0, 1.0, rS, rE, n)
	for i, s := range segs {
		tCenter := (float64(i) + 0.5) / float64(n)
		want := rS + tCenter*(rE-rS)
		if math.Abs(s.Radius-want) > 1e-15 {
			t.Errorf("segment %d Radius = %g, want %g", i, s.Radius, want)
		}
	}
}

// A tapered wire collapses cleanly to the uniform result as rEnd → rStart.
// Sweeping the ratio from 1 → 8 must yield continuous Re/Im impedance; a
// jump at the isTapered() fallback boundary would imply a spurious branch
// in the solver path.
func TestTaperSmoothDegenerationToUniform(t *testing.T) {
	freq := 300e6
	lambda := C0 / freq
	half := lambda / 4

	run := func(rS, rE float64) ComplexImpedance {
		in := SimulationInput{
			Wires: []Wire{{
				X1: 0, Y1: 0, Z1: -half,
				X2: 0, Y2: 0, Z2: half,
				Radius:      0.001,
				RadiusStart: rS,
				RadiusEnd:   rE,
				Segments:    21,
			}},
			Frequency: freq,
			Ground:    GroundConfig{Type: "free_space"},
			Source:    Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
		}
		r, err := Simulate(in)
		if err != nil {
			t.Fatalf("Simulate(%v,%v): %v", rS, rE, err)
		}
		return r.Impedance
	}

	// Reference: uniform wire with radius 1 mm, no taper fields.
	base := SimulationInput{
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: -half,
			X2: 0, Y2: 0, Z2: half,
			Radius: 0.001, Segments: 21,
		}},
		Frequency: freq,
		Ground:    GroundConfig{Type: "free_space"},
		Source:    Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
	}
	baseRes, err := Simulate(base)
	if err != nil {
		t.Fatalf("base Simulate: %v", err)
	}

	// Taper with rS == rE == 1 mm must match the uniform reference closely.
	// The taper branch reuses the same Z-matrix arithmetic, so agreement to
	// ~1e-9 is expected.
	eq := run(0.001, 0.001)
	if math.Abs(eq.R-baseRes.Impedance.R) > 1e-6 || math.Abs(eq.X-baseRes.Impedance.X) > 1e-6 {
		t.Fatalf("taper(1mm,1mm) diverges from uniform: taper=%+v uniform=%+v", eq, baseRes.Impedance)
	}

	// Sweep rEnd/rStart from 1 to 8; consecutive samples should be smooth.
	ratios := []float64{1.0, 1.25, 1.5, 2, 3, 4, 6, 8}
	prev := run(0.001, 0.001)
	for _, ratio := range ratios[1:] {
		cur := run(0.001, 0.001*ratio)
		// Impedance should move monotonically but never jump wildly.
		dR := math.Abs(cur.R - prev.R)
		dX := math.Abs(cur.X - prev.X)
		if dR > 30 || dX > 30 {
			t.Errorf("ratio %g: impedance jump too large (ΔR=%g ΔX=%g, %+v → %+v)",
				ratio, dR, dX, prev, cur)
		}
		prev = cur
	}
}

// Coating regression: a tapered wire with tanδ = 0 must still produce a
// purely reactive zBasis contribution (real part ≈ 0). Catches the
// half-and-half split regression if zPul1 and zPul2 evaluation were to
// accidentally couple with w.Radius again.
func TestTaperedCoatingLosslessStaysLossless(t *testing.T) {
	freq := 100e6
	lambda := C0 / freq
	half := lambda / 4

	// Dipole with linear taper 0.5mm → 2mm, PVC-style εr but tanδ=0.
	in := SimulationInput{
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: -half,
			X2: 0, Y2: 0, Z2: half,
			Radius: 0.001, // fallback only — taper dominates
			Segments:    21,
			RadiusStart: 0.0005,
			RadiusEnd:   0.002,
			CoatingThickness: 0.0005,
			CoatingEpsR:      3.5,
			CoatingLossTan:   0, // lossless
		}},
		Frequency: freq,
		Ground:    GroundConfig{Type: "free_space"},
		Source:    Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
	}
	bare := in
	bare.Wires[0].CoatingThickness = 0
	bare.Wires[0].CoatingEpsR = 0

	r1, err := Simulate(in)
	if err != nil {
		t.Fatalf("coated Simulate: %v", err)
	}
	r2, err := Simulate(bare)
	if err != nil {
		t.Fatalf("bare Simulate: %v", err)
	}

	// With tanδ=0 the coating contributes only a reactive (imaginary) term,
	// so any resistance change between the coated and bare dipoles must come
	// from the implicit dielectric-loading shift of the resonance, not from
	// resistive loading. In practice this stays within a few Ω; a broken
	// half-and-half split would double-count real(zBasis) and push ΔR into
	// the tens or hundreds.
	deltaR := r1.Impedance.R - r2.Impedance.R
	if math.Abs(deltaR) > 10.0 {
		t.Errorf("lossless coating added resistance ΔR=%g Ω — expected ~0", deltaR)
	}
}

// Validator: two wires meeting at a common endpoint where the uniform
// Radii match but tapered radii at the junction differ must fire the
// junction_radius_mismatch warning.
func TestValidatorJunctionTaperedMismatch(t *testing.T) {
	wires := []Wire{
		{
			X1: 0, Y1: 0, Z1: 0,
			X2: 0, Y2: 0, Z2: 1,
			Radius:      0.001,
			RadiusStart: 0.0005, // thin at z=0
			RadiusEnd:   0.005,  // fat at z=1
			Segments:    11,
		},
		{
			X1: 0, Y1: 0, Z1: 1,
			X2: 0, Y2: 0, Z2: 2,
			Radius:      0.001, // same uniform Radius as wire 0
			RadiusStart: 0.005, // matches wire[0].RadiusEnd at the shared z=1 junction
			RadiusEnd:   0.0005,
			Segments:    11,
		},
	}
	// Junction radii: wire[0].RadiusEnd = 5mm, wire[1].RadiusStart = 5mm.
	// These match, so NO mismatch should fire even though the wires' uniform
	// Radius fields look identical and no-taper check would also pass.
	ws := ValidateGeometry(wires, 100e6)
	for _, w := range ws {
		if w.Code == "junction_radius_mismatch" {
			t.Errorf("unexpected junction_radius_mismatch with matched taper radii: %s", w.Message)
		}
	}

	// Now mutate wire[1] so its junction-end radius differs.
	wires[1].RadiusStart = 0.0005 // 10× smaller than wire[0].RadiusEnd
	wires[1].RadiusEnd = 0.005
	ws = ValidateGeometry(wires, 100e6)
	found := false
	for _, w := range ws {
		if w.Code == "junction_radius_mismatch" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected junction_radius_mismatch when tapered endpoint radii differ; got %v", ws)
	}
}

// The taper_ratio_high warning fires above 10:1 and not below.
func TestValidatorTaperRatioHigh(t *testing.T) {
	mkWire := func(rS, rE float64) Wire {
		return Wire{
			X1: 0, Y1: 0, Z1: 0,
			X2: 0, Y2: 0, Z2: 1,
			Radius:      0.001,
			RadiusStart: rS,
			RadiusEnd:   rE,
			Segments:    30, // keep segLen/maxR well above the kernel threshold
		}
	}
	hasCode := func(ws []Warning, code string) bool {
		for _, w := range ws {
			if w.Code == code {
				return true
			}
		}
		return false
	}

	// 9.9× — should NOT fire.
	ws := ValidateGeometry([]Wire{mkWire(0.0001, 0.00099)}, 30e6)
	if hasCode(ws, "taper_ratio_high") {
		t.Errorf("taper_ratio_high fired at 9.9×")
	}

	// 15× — should fire.
	ws = ValidateGeometry([]Wire{mkWire(0.0001, 0.0015)}, 30e6)
	if !hasCode(ws, "taper_ratio_high") {
		t.Errorf("taper_ratio_high did not fire at 15×")
	}
}
