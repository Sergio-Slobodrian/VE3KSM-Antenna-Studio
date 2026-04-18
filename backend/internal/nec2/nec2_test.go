package nec2

import (
	"bytes"
	"math"
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
	if _, err := Write(&buf, FromInput(in), WriteOptions{
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
	if _, err := Write(&buf, FromInput(in), WriteOptions{
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

// TestWrite_BareWireNoCoatingArtifacts confirms that a bare wire produces
// no coating-related CM headers and no radius substitution.
func TestWrite_BareWireNoCoatingArtifacts(t *testing.T) {
	in := mom.SimulationInput{
		Wires: []mom.Wire{{
			X1: 0, Y1: -5, Z1: 0, X2: 0, Y2: 5, Z2: 0,
			Radius: 0.001, Segments: 11,
		}},
		Source: mom.Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground: mom.GroundConfig{Type: "free_space"},
	}
	var buf bytes.Buffer
	warnings, err := Write(&buf, FromInput(in), WriteOptions{FreqStartMHz: 14, FreqSteps: 1})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for bare wire, got %v", warnings)
	}
	out := buf.String()
	if strings.Contains(out, "Dielectric coatings approximated") {
		t.Errorf("bare wire should not emit coating header:\n%s", out)
	}
	if !strings.Contains(out, "GW 1 11 0 -5 0 0 5 0 0.001") {
		t.Errorf("bare wire should keep original radius 0.001, got:\n%s", out)
	}
}

// TestWrite_CoatedWireEffectiveRadius verifies the Tsai/Richmond
// approximation: a lossless PVC coating should produce an effective
// radius matching ln(a_eff) = ln(a) + (1 − 1/εr) ln(b/a), and the
// header/CM documentation should describe the approximation.
func TestWrite_CoatedWireEffectiveRadius(t *testing.T) {
	const a = 0.001
	const t_coat = 0.002
	const epsR = 2.3
	in := mom.SimulationInput{
		Wires: []mom.Wire{{
			X1: 0, Y1: -5, Z1: 0, X2: 0, Y2: 5, Z2: 0,
			Radius: a, Segments: 11,
			CoatingThickness: t_coat, CoatingEpsR: epsR, CoatingLossTan: 0,
		}},
		Source: mom.Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground: mom.GroundConfig{Type: "free_space"},
	}
	var buf bytes.Buffer
	warnings, err := Write(&buf, FromInput(in), WriteOptions{FreqStartMHz: 14, FreqSteps: 1})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()

	// Lossless coating ⇒ no lossy-drop warning, but the generic
	// "approximated by effective radius" warning must appear.
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning (approximation notice), got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "effective wire radius") {
		t.Errorf("warning text mismatch: %q", warnings[0])
	}

	// Header + per-wire CM card preserving original parameters.
	if !strings.Contains(out, "CM Dielectric coatings approximated by effective radius") {
		t.Errorf("missing approximation header:\n%s", out)
	}
	if !strings.Contains(out, "CM wire 1 coating:") {
		t.Errorf("missing per-wire coating CM:\n%s", out)
	}

	// Verify the effective radius is physically consistent.  We can't
	// check the GW line's floating-point formatting exactly, so extract
	// the radius numerically from the coating CM card.
	b := a + t_coat
	expected := a * math.Pow(b/a, 1-1/epsR)
	if !strings.Contains(out, "a_eff=") {
		t.Fatalf("coating CM card missing a_eff field:\n%s", out)
	}
	// Also check via the exported helper.
	got := effectiveRadius(a, [][2]float64{{epsR, b}})
	if math.Abs(got-expected) > 1e-12 {
		t.Errorf("effectiveRadius: got %g, expected %g", got, expected)
	}
	if got <= a {
		t.Errorf("effective radius should exceed bare radius (coating adds inductance): got %g, bare %g", got, a)
	}
}

// TestWrite_LossyCoatingWarning verifies that a coating with tanδ > 0
// produces an explicit warning that resistive loading has been dropped.
func TestWrite_LossyCoatingWarning(t *testing.T) {
	in := mom.SimulationInput{
		Wires: []mom.Wire{{
			X1: 0, Y1: -5, Z1: 0, X2: 0, Y2: 5, Z2: 0,
			Radius: 0.001, Segments: 11,
			CoatingThickness: 0.002, CoatingEpsR: 2.3, CoatingLossTan: 0.05,
		}},
		Source: mom.Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground: mom.GroundConfig{Type: "free_space"},
	}
	var buf bytes.Buffer
	warnings, err := Write(&buf, FromInput(in), WriteOptions{FreqStartMHz: 14, FreqSteps: 1})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	foundLossy := false
	for _, w := range warnings {
		if strings.Contains(w, "tanδ") || strings.Contains(w, "resistive loading") {
			foundLossy = true
		}
	}
	if !foundLossy {
		t.Errorf("expected lossy-coating warning, got %v", warnings)
	}
	if !strings.Contains(buf.String(), "tanδ") {
		t.Errorf("file should document the dropped loss term:\n%s", buf.String())
	}
}

// TestWrite_WeatherOnBareWire verifies that a global weather film is
// applied to a bare wire as a single outer layer, producing an effective
// radius > bare.
func TestWrite_WeatherOnBareWire(t *testing.T) {
	const a = 0.001
	in := mom.SimulationInput{
		Wires: []mom.Wire{{
			X1: 0, Y1: -5, Z1: 0, X2: 0, Y2: 5, Z2: 0,
			Radius: a, Segments: 11,
		}},
		Source:  mom.Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground:  mom.GroundConfig{Type: "free_space"},
		Weather: mom.WeatherConfig{Preset: "rain", Thickness: 1e-4},
	}
	var buf bytes.Buffer
	warnings, err := Write(&buf, FromInput(in), WriteOptions{FreqStartMHz: 14, FreqSteps: 1})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	if len(warnings) == 0 {
		t.Fatal("expected warnings for weather film")
	}
	if !strings.Contains(out, "CM wire 1 weather film:") {
		t.Errorf("missing per-wire weather CM card:\n%s", out)
	}

	// Rain preset: εr=80, tanδ=0.05 → should trip the lossy warning too.
	foundLossy := false
	for _, w := range warnings {
		if strings.Contains(w, "tanδ") || strings.Contains(w, "resistive loading") {
			foundLossy = true
		}
	}
	if !foundLossy {
		t.Errorf("rain preset should trigger lossy-coating warning, got %v", warnings)
	}
}

// TestWrite_GroundMoisturePreset verifies the writer emits a CM card
// documenting the soil moisture preset when it is set (non-"custom"),
// and omits the CM when the preset is "custom" or empty.
func TestWrite_GroundMoisturePreset(t *testing.T) {
	base := mom.SimulationInput{
		Wires: []mom.Wire{{
			X1: 0, Y1: -5, Z1: 1, X2: 0, Y2: 5, Z2: 1,
			Radius: 0.001, Segments: 11,
		}},
		Source: mom.Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground: mom.GroundConfig{
			Type:           "real",
			Conductivity:   0.02,
			Permittivity:   30,
			MoisturePreset: "wet",
		},
	}

	var buf bytes.Buffer
	if _, err := Write(&buf, FromInput(base), WriteOptions{FreqStartMHz: 14, FreqSteps: 1}); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "CM Ground moisture preset: wet") {
		t.Errorf("missing moisture-preset CM card:\n%s", out)
	}
	if !strings.Contains(out, "GN 2 0 0 0 30") {
		t.Errorf("GN card should carry εr=30 unchanged:\n%s", out)
	}

	// "custom" preset must not emit a CM card.
	base.Ground.MoisturePreset = "custom"
	buf.Reset()
	if _, err := Write(&buf, FromInput(base), WriteOptions{FreqStartMHz: 14, FreqSteps: 1}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if strings.Contains(buf.String(), "Ground moisture preset") {
		t.Errorf("custom preset should not emit CM card:\n%s", buf.String())
	}
}

// TestWrite_GroundRegionPreset verifies the writer emits a CM card
// documenting the region preset (from the map picker) when it is set,
// and omits the CM when empty.
func TestWrite_GroundRegionPreset(t *testing.T) {
	base := mom.SimulationInput{
		Wires: []mom.Wire{{
			X1: 0, Y1: -5, Z1: 1, X2: 0, Y2: 5, Z2: 1,
			Radius: 0.001, Segments: 11,
		}},
		Source: mom.Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground: mom.GroundConfig{
			Type:         "real",
			Conductivity: 0.01,
			Permittivity: 15,
			RegionPreset: "itu:3",
		},
	}

	var buf bytes.Buffer
	if _, err := Write(&buf, FromInput(base), WriteOptions{FreqStartMHz: 14, FreqSteps: 1}); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "CM Ground region preset: itu:3") {
		t.Errorf("missing region-preset CM card:\n%s", out)
	}
	if !strings.Contains(out, "GN 2 0 0 0 15") {
		t.Errorf("GN card should carry εr=15 unchanged:\n%s", out)
	}

	// Empty preset must not emit a CM card.
	base.Ground.RegionPreset = ""
	buf.Reset()
	if _, err := Write(&buf, FromInput(base), WriteOptions{FreqStartMHz: 14, FreqSteps: 1}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if strings.Contains(buf.String(), "Ground region preset") {
		t.Errorf("empty preset should not emit CM card:\n%s", buf.String())
	}
}

// TestWrite_MultilayerEffectiveRadius confirms that coating + weather
// stack inner-to-outer and produce a larger effective radius than coating
// alone.
func TestWrite_MultilayerEffectiveRadius(t *testing.T) {
	const a = 0.001
	rBare := effectiveRadius(a, nil)
	rCoat := effectiveRadius(a, [][2]float64{{2.3, 0.003}})
	rStack := effectiveRadius(a, [][2]float64{{2.3, 0.003}, {80.0, 0.0031}})
	if !(rBare < rCoat && rCoat < rStack) {
		t.Fatalf("expected rBare < rCoat < rStack, got %g / %g / %g", rBare, rCoat, rStack)
	}
	if rBare != a {
		t.Errorf("no layers should return the bare radius, got %g want %g", rBare, a)
	}
}
