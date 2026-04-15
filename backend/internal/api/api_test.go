package api

import (
	"testing"

	"antenna-studio/backend/internal/mom"
)

// validRequest returns a SimulateRequest that passes all validation.
func validRequest() SimulateRequest {
	return SimulateRequest{
		Wires:        []WireDTO{{X1: 0, Y1: 0, Z1: 0, X2: 0, Y2: 0, Z2: 1, Radius: 0.001, Segments: 11}},
		FrequencyMHz: 14.0,
		Ground:       GroundDTO{Type: "free_space"},
		Source:       SourceDTO{WireIndex: 0, SegmentIndex: 5, Voltage: 1.0},
	}
}

// ---------------------------------------------------------------------------
// SimulateRequest.Validate()
// ---------------------------------------------------------------------------

func TestValidate_ValidRequest(t *testing.T) {
	r := validRequest()
	if err := r.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidate_EmptyWires(t *testing.T) {
	r := validRequest()
	r.Wires = nil
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty wires")
	}
}

func TestValidate_ZeroFrequency(t *testing.T) {
	r := validRequest()
	r.FrequencyMHz = 0
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for zero frequency")
	}
}

func TestValidate_ZeroLengthWire(t *testing.T) {
	r := validRequest()
	r.Wires[0] = WireDTO{X1: 1, Y1: 2, Z1: 3, X2: 1, Y2: 2, Z2: 3, Radius: 0.001, Segments: 11}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for zero-length wire")
	}
}

func TestValidate_NegativeRadius(t *testing.T) {
	r := validRequest()
	r.Wires[0].Radius = -0.001
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for negative radius")
	}
}

func TestValidate_InvalidGroundType(t *testing.T) {
	r := validRequest()
	r.Ground.Type = "wet_sand"
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for invalid ground type")
	}
}

func TestValidate_RealGroundWithoutConductivity(t *testing.T) {
	r := validRequest()
	r.Ground = GroundDTO{Type: "real", Conductivity: 0, Permittivity: 13.0}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for real ground without conductivity")
	}
}

func TestValidate_SourceWireIndexOutOfRange(t *testing.T) {
	r := validRequest()
	r.Source.WireIndex = 5
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for source wire_index out of range")
	}
}

func TestValidate_SourceSegmentIndexOutOfRange(t *testing.T) {
	r := validRequest()
	r.Source.SegmentIndex = 99
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for source segment_index out of range")
	}
}

func TestValidate_EmptyGroundDefaultsToFreeSpace(t *testing.T) {
	r := validRequest()
	r.Ground.Type = ""
	if err := r.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if r.Ground.Type != "free_space" {
		t.Fatalf("expected ground type to default to free_space, got %q", r.Ground.Type)
	}
}

// ---------------------------------------------------------------------------
// SweepRequest.ToSimulateRequest()
// ---------------------------------------------------------------------------

func TestToSimulateRequest_CopiesFields(t *testing.T) {
	sr := SweepRequest{
		Wires:     []WireDTO{{X1: 0, Y1: 0, Z1: 0, X2: 0, Y2: 0, Z2: 2, Radius: 0.002, Segments: 21}},
		Ground:    GroundDTO{Type: "perfect"},
		Source:    SourceDTO{WireIndex: 0, SegmentIndex: 10, Voltage: 1.5},
		FreqStart: 7.0,
		FreqEnd:   21.0,
		FreqSteps: 50,
	}

	r := sr.ToSimulateRequest()

	if len(r.Wires) != len(sr.Wires) {
		t.Fatalf("wires length mismatch: got %d, want %d", len(r.Wires), len(sr.Wires))
	}
	if r.Wires[0].Z2 != 2 {
		t.Fatalf("wire Z2 mismatch: got %f, want 2", r.Wires[0].Z2)
	}
	if r.Ground.Type != "perfect" {
		t.Fatalf("ground type mismatch: got %q, want %q", r.Ground.Type, "perfect")
	}
	if r.Source.SegmentIndex != 10 {
		t.Fatalf("source segment_index mismatch: got %d, want 10", r.Source.SegmentIndex)
	}
	if r.FrequencyMHz != 7.0 {
		t.Fatalf("FrequencyMHz should be FreqStart (7.0), got %f", r.FrequencyMHz)
	}
}

// ---------------------------------------------------------------------------
// SolverResultToResponse()
// ---------------------------------------------------------------------------

func TestSolverResultToResponse(t *testing.T) {
	sr := &mom.SolverResult{
		Impedance: mom.ComplexImpedance{R: 73.0, X: 42.5},
		SWR:       1.85,
		GainDBi:   2.15,
		Pattern: []mom.PatternPoint{
			{ThetaDeg: 0, PhiDeg: 0, GainDB: 2.15},
			{ThetaDeg: 90, PhiDeg: 0, GainDB: -3.0},
		},
		Currents: []mom.CurrentEntry{
			{SegmentIndex: 0, Magnitude: 0.01, PhaseDeg: 0},
			{SegmentIndex: 1, Magnitude: 0.02, PhaseDeg: 45.0},
		},
	}

	resp := SolverResultToResponse(sr)

	if resp.Impedance.R != 73.0 || resp.Impedance.X != 42.5 {
		t.Fatalf("impedance mismatch: got R=%f X=%f", resp.Impedance.R, resp.Impedance.X)
	}
	if resp.SWR != 1.85 {
		t.Fatalf("SWR mismatch: got %f, want 1.85", resp.SWR)
	}
	if resp.GainDBi != 2.15 {
		t.Fatalf("GainDBi mismatch: got %f, want 2.15", resp.GainDBi)
	}

	if len(resp.Pattern) != 2 {
		t.Fatalf("pattern length mismatch: got %d, want 2", len(resp.Pattern))
	}
	if resp.Pattern[0].Theta != 0 || resp.Pattern[0].Phi != 0 || resp.Pattern[0].GainDB != 2.15 {
		t.Fatalf("pattern[0] mismatch: %+v", resp.Pattern[0])
	}
	if resp.Pattern[1].Theta != 90 {
		t.Fatalf("pattern[1].Theta mismatch: got %f, want 90", resp.Pattern[1].Theta)
	}

	if len(resp.Currents) != 2 {
		t.Fatalf("currents length mismatch: got %d, want 2", len(resp.Currents))
	}
	if resp.Currents[0].Segment != 0 || resp.Currents[0].Magnitude != 0.01 || resp.Currents[0].Phase != 0 {
		t.Fatalf("currents[0] mismatch: %+v", resp.Currents[0])
	}
	if resp.Currents[1].Segment != 1 || resp.Currents[1].Magnitude != 0.02 || resp.Currents[1].Phase != 45.0 {
		t.Fatalf("currents[1] mismatch: %+v", resp.Currents[1])
	}
}

// ---------------------------------------------------------------------------
// SweepResultToResponse()
// ---------------------------------------------------------------------------

func TestSweepResultToResponse(t *testing.T) {
	sr := &mom.SweepResult{
		Frequencies: []float64{7.0, 14.0, 21.0},
		SWR:         []float64{3.0, 1.1, 2.5},
		Impedance: []mom.ComplexImpedance{
			{R: 20, X: -30},
			{R: 50, X: 1},
			{R: 100, X: 50},
		},
	}

	resp := SweepResultToResponse(sr)

	if len(resp.Frequencies) != 3 {
		t.Fatalf("frequencies length mismatch: got %d, want 3", len(resp.Frequencies))
	}
	if resp.Frequencies[1] != 14.0 {
		t.Fatalf("frequencies[1] mismatch: got %f, want 14.0", resp.Frequencies[1])
	}

	if len(resp.SWR) != 3 {
		t.Fatalf("SWR length mismatch: got %d, want 3", len(resp.SWR))
	}
	if resp.SWR[0] != 3.0 || resp.SWR[1] != 1.1 || resp.SWR[2] != 2.5 {
		t.Fatalf("SWR mismatch: got %v", resp.SWR)
	}

	if len(resp.Impedance) != 3 {
		t.Fatalf("impedance length mismatch: got %d, want 3", len(resp.Impedance))
	}
	if resp.Impedance[0].R != 20 || resp.Impedance[0].X != -30 {
		t.Fatalf("impedance[0] mismatch: %+v", resp.Impedance[0])
	}
	if resp.Impedance[2].R != 100 || resp.Impedance[2].X != 50 {
		t.Fatalf("impedance[2] mismatch: %+v", resp.Impedance[2])
	}
}
