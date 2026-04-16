package nec2

import (
	"bytes"
	"strings"
	"testing"

	"antenna-studio/backend/internal/mom"
)

func TestParse_HalfWaveDipole(t *testing.T) {
	src := `CM Dipole at 14 MHz
CE
GW 1 21 0 -5.355 0 0 5.355 0 0.001
GE 0
EX 0 1 11 0 1 0
FR 0 1 0 0 14.0
EN
`
	f, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	g, err := ToGeometry(f)
	if err != nil {
		t.Fatalf("toGeometry: %v", err)
	}
	if len(g.Input.Wires) != 1 {
		t.Fatalf("want 1 wire, got %d", len(g.Input.Wires))
	}
	w := g.Input.Wires[0]
	if w.Segments != 21 {
		t.Errorf("segments: want 21, got %d", w.Segments)
	}
	if w.Y1 >= 0 || w.Y2 <= 0 {
		t.Errorf("dipole should straddle y=0, got y1=%v y2=%v", w.Y1, w.Y2)
	}
	if g.Input.Source.WireIndex != 0 || g.Input.Source.SegmentIndex != 10 {
		t.Errorf("source: want (0, 10), got (%d, %d)", g.Input.Source.WireIndex, g.Input.Source.SegmentIndex)
	}
	if g.Input.Frequency != 14e6 {
		t.Errorf("freq: want 14e6, got %v", g.Input.Frequency)
	}
}

func TestParse_FreqSweep(t *testing.T) {
	src := `CE
GW 1 11 0 -5 0 0 5 0 0.001
GE 0
FR 0 21 0 0 13.0 0.1
EN
`
	f, _ := Parse(strings.NewReader(src))
	g, _ := ToGeometry(f)
	if g.FreqSteps != 21 {
		t.Errorf("steps: want 21, got %d", g.FreqSteps)
	}
	if g.FreqStartHz != 13e6 {
		t.Errorf("start: want 13e6, got %v", g.FreqStartHz)
	}
	if g.FreqEndHz != 15e6 { // 13 + 20*0.1 = 15
		t.Errorf("end: want 15e6, got %v", g.FreqEndHz)
	}
}

func TestParse_LumpedLoadAndTL(t *testing.T) {
	src := `CE
GW 1 11 0 0 0 0 0 5 0.001
GW 2 11 1 0 0 1 0 5 0.001
GE 0
EX 0 1 6 0 1 0
LD 0 1 6 6 50 0 0
TL 1 6 -1 0 50 1.34
EN
`
	f, _ := Parse(strings.NewReader(src))
	g, err := ToGeometry(f)
	if err != nil {
		t.Fatalf("toGeometry: %v", err)
	}
	if len(g.Input.Loads) != 1 {
		t.Fatalf("want 1 load, got %d", len(g.Input.Loads))
	}
	if g.Input.Loads[0].R != 50 {
		t.Errorf("load R: want 50, got %v", g.Input.Loads[0].R)
	}
	if len(g.Input.TransmissionLines) != 1 {
		t.Fatalf("want 1 TL, got %d", len(g.Input.TransmissionLines))
	}
	tl := g.Input.TransmissionLines[0]
	if tl.B.WireIndex != mom.TLEndShorted {
		t.Errorf("B end should be shorted (-1), got %d", tl.B.WireIndex)
	}
	if tl.Z0 != 50 || tl.Length != 1.34 {
		t.Errorf("TL Z0/length: got %v / %v", tl.Z0, tl.Length)
	}
}

func TestWrite_Dipole(t *testing.T) {
	in := mom.SimulationInput{
		Wires: []mom.Wire{{
			X1: 0, Y1: -5, Z1: 0, X2: 0, Y2: 5, Z2: 0,
			Radius: 0.001, Segments: 11, Material: mom.MaterialCopper,
		}},
		Source: mom.Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground: mom.GroundConfig{Type: "free_space"},
	}
	var buf bytes.Buffer
	if err := Write(&buf, FromInput(in), WriteOptions{
		FreqStartMHz: 14, FreqStepMHz: 0, FreqSteps: 1,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "GW 1 11") {
		t.Errorf("missing GW: %s", out)
	}
	if !strings.Contains(out, "EX 0 1 6 0 1") {
		t.Errorf("missing EX: %s", out)
	}
	if !strings.Contains(out, "LD 5 1 0 0 5.8e+07") && !strings.Contains(out, "LD 5 1 0 0 58000000") {
		t.Errorf("missing LD 5 (Cu sigma): %s", out)
	}
	if !strings.Contains(out, "FR 0 1 0 0 14") {
		t.Errorf("missing FR: %s", out)
	}
}

func TestRoundTrip_Dipole(t *testing.T) {
	in := mom.SimulationInput{
		Wires: []mom.Wire{{
			X1: 0, Y1: -5.355, Z1: 0, X2: 0, Y2: 5.355, Z2: 0,
			Radius: 0.001, Segments: 21,
		}},
		Source: mom.Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
		Ground: mom.GroundConfig{Type: "free_space"},
	}
	var buf bytes.Buffer
	if err := Write(&buf, FromInput(in), WriteOptions{
		FreqStartMHz: 14, FreqSteps: 1,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := Parse(&buf)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	g, err := ToGeometry(f)
	if err != nil {
		t.Fatalf("re-toGeometry: %v", err)
	}
	if len(g.Input.Wires) != 1 {
		t.Fatalf("round-trip wires: want 1, got %d", len(g.Input.Wires))
	}
	w := g.Input.Wires[0]
	if w.Segments != 21 {
		t.Errorf("segments: want 21, got %d", w.Segments)
	}
}
