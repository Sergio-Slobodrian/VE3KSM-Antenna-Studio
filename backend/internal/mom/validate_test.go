package mom

import (
	"strings"
	"testing"
)

func hasCode(ws []Warning, code string) bool {
	for _, w := range ws {
		if w.Code == code {
			return true
		}
	}
	return false
}

func severityOf(ws []Warning, code string) WarnSeverity {
	for _, w := range ws {
		if w.Code == code {
			return w.Severity
		}
	}
	return ""
}

// A well-segmented half-wave dipole at 14 MHz should produce no
// warnings whatsoever.
func TestValidate_CleanDipoleNoWarnings(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	wires := []Wire{{
		X1: 0, Y1: -halfL, Z1: 0,
		X2: 0, Y2: halfL, Z2: 0,
		Radius:   1e-3,
		Segments: 21, // segLen ≈ λ/42, well under λ/20
	}}
	ws := ValidateGeometry(wires, freq)
	if len(ws) != 0 {
		t.Fatalf("expected no warnings on clean dipole, got %d: %+v", len(ws), ws)
	}
}

// Under-segmented wire: 3 segments on a half-wave at 14 MHz puts each
// segment at ~λ/6 — way over the λ/10 lower bound.
func TestValidate_SegmentTooLong(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	wires := []Wire{{
		X1: 0, Y1: -halfL, Z1: 0,
		X2: 0, Y2: halfL, Z2: 0,
		Radius:   1e-3,
		Segments: 3,
	}}
	ws := ValidateGeometry(wires, freq)
	if !hasCode(ws, "segment_too_long") {
		t.Fatalf("expected segment_too_long warning, got %+v", ws)
	}
	if severityOf(ws, "segment_too_long") != SeverityError {
		t.Fatalf("segment_too_long should be severity error")
	}
}

// Mid-segmented (between λ/20 and λ/10): warn but don't error.
func TestValidate_SegmentBetweenLambda20And10(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	// 7 segments → segLen ≈ λ/14, between λ/20 and λ/10.
	wires := []Wire{{
		X1: 0, Y1: -halfL, Z1: 0,
		X2: 0, Y2: halfL, Z2: 0,
		Radius:   1e-3,
		Segments: 7,
	}}
	ws := ValidateGeometry(wires, freq)
	if !hasCode(ws, "segment_below_lambda_over_20") {
		t.Fatalf("expected segment_below_lambda_over_20 warning, got %+v", ws)
	}
	if severityOf(ws, "segment_below_lambda_over_20") != SeverityWarn {
		t.Fatal("expected warn severity")
	}
}

// Fat wire: radius too large vs segment length.
func TestValidate_KernelInvalidRadius(t *testing.T) {
	freq := 14e6
	wires := []Wire{{
		X1: 0, Y1: 0, Z1: 0,
		X2: 1, Y2: 0, Z2: 0,
		Radius:   0.6, // ~radius == half segment length even at 1 segment
		Segments: 1,
	}}
	ws := ValidateGeometry(wires, freq)
	if !hasCode(ws, "kernel_invalid_radius") {
		t.Fatalf("expected kernel_invalid_radius, got %+v", ws)
	}
}

// Marginal kernel: 2 ≤ ratio < 8 → warn, not error.
func TestValidate_KernelMarginalRadius(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	wires := []Wire{{
		X1: 0, Y1: 0, Z1: 0,
		X2: wavelength / 2, Y2: 0, Z2: 0,
		Radius:   wavelength / 100, // segLen = λ/40, ratio = (λ/40)/(λ/100) = 2.5 < 8
		Segments: 20,
	}}
	ws := ValidateGeometry(wires, freq)
	if !hasCode(ws, "kernel_marginal_radius") {
		t.Fatalf("expected kernel_marginal_radius, got %+v", ws)
	}
}

// Adjacent wires meeting at a junction with very different segment
// lengths should warn (or error if very different).
func TestValidate_AdjacentSegmentMismatch(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	w1 := Wire{ // 1 m long, 50 segs → segLen = 0.02 m
		X1: 0, Y1: 0, Z1: 0,
		X2: 1, Y2: 0, Z2: 0,
		Radius:   1e-4,
		Segments: 50,
	}
	w2 := Wire{ // shares endpoint (1,0,0); 1 m long, 5 segs → segLen = 0.20 m  (10× larger)
		X1: 1, Y1: 0, Z1: 0,
		X2: 2, Y2: 0, Z2: 0,
		Radius:   1e-4,
		Segments: 5,
	}
	ws := ValidateGeometry([]Wire{w1, w2}, freq)
	if !hasCode(ws, "adjacent_segment_ratio_severe") {
		t.Fatalf("expected adjacent_segment_ratio_severe, got %+v", ws)
	}
	_ = wavelength
}

// Junction radius mismatch.
func TestValidate_JunctionRadiusMismatch(t *testing.T) {
	freq := 14e6
	w1 := Wire{
		X1: 0, Y1: 0, Z1: 0, X2: 1, Y2: 0, Z2: 0,
		Radius: 1e-3, Segments: 20,
	}
	w2 := Wire{
		X1: 1, Y1: 0, Z1: 0, X2: 2, Y2: 0, Z2: 0,
		Radius: 1e-2, // 10× larger
		Segments: 20,
	}
	ws := ValidateGeometry([]Wire{w1, w2}, freq)
	if !hasCode(ws, "junction_radius_mismatch") {
		t.Fatalf("expected junction_radius_mismatch, got %+v", ws)
	}
	for _, w := range ws {
		if w.Code == "junction_radius_mismatch" {
			if !strings.Contains(w.Message, "10.0×") && !strings.Contains(w.Message, "10×") {
				t.Logf("note: message text: %q", w.Message)
			}
		}
	}
}

// Simulate() should attach warnings to the result.
func TestSimulate_AttachesWarnings(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	geom := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: -halfL, Z1: 0,
			X2: 0, Y2: halfL, Z2: 0,
			Radius:   1e-3,
			Segments: 5, // under-segmented to trigger a warning
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 2, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}
	res, err := Simulate(geom)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected at least one warning on a 5-segment dipole at 14 MHz")
	}
	// Should contain a segment-length warning of some kind.
	if !hasCode(res.Warnings, "segment_below_lambda_over_20") &&
		!hasCode(res.Warnings, "segment_too_long") {
		t.Fatalf("expected a segment-length warning, got %+v", res.Warnings)
	}
}

// Sweep ill-conditioning: a 10 m wire with 65 segments at 5 MHz puts
// each segment at ~λ/390 — well below the NEC-2 λ/200 minimum.  The
// validator should flag this.
func TestValidate_SegmentTooShortForFrequency(t *testing.T) {
	wires := []Wire{{
		X1: 0, Y1: 0, Z1: 0,
		X2: 0, Y2: 0, Z2: 10,
		Radius:   1e-3,
		Segments: 65,
	}}
	ws := ValidateGeometry(wires, 5e6) // 5 MHz, λ = 60 m, segLen = 0.154 m
	if !hasCode(ws, "segment_too_short_for_frequency") &&
		!hasCode(ws, "segment_short_for_frequency") {
		t.Fatalf("expected segment-too-short warning at 5 MHz, got %+v", ws)
	}
}

