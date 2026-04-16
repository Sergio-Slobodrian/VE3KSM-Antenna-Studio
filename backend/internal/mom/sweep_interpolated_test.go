package mom

import (
	"math"
	"testing"
)

func TestSpline_PassesThroughKnots(t *testing.T) {
	xs := []float64{0, 1, 2, 4, 7}
	ys := []float64{0, 1, 0, -1, 2}
	s, err := NewSpline(xs, ys)
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range xs {
		got := s.Eval(x)
		if math.Abs(got-ys[i]) > 1e-9 {
			t.Errorf("knot %d: spline(%v)=%v, want %v", i, x, got, ys[i])
		}
	}
}

func TestSpline_LinearOnTwoPoints(t *testing.T) {
	s, _ := NewSpline([]float64{0, 10}, []float64{0, 100})
	if v := s.Eval(5); math.Abs(v-50) > 1e-9 {
		t.Fatalf("midpoint of two-point spline: got %v, want 50", v)
	}
}

func TestSpline_RejectsNonMonotonic(t *testing.T) {
	if _, err := NewSpline([]float64{0, 2, 1}, []float64{0, 0, 0}); err == nil {
		t.Fatal("expected error for non-monotonic x")
	}
}

func TestChooseAnchors(t *testing.T) {
	tests := []struct{ steps, want int }{
		{200, 20},
		{500, 32},
		{32, 8},
		{8, 8},
		{100, 15},
	}
	for _, c := range tests {
		got := chooseAnchors(c.steps)
		if got != c.want {
			t.Errorf("chooseAnchors(%d) = %d, want %d", c.steps, got, c.want)
		}
	}
}

// End-to-end: an interpolated sweep on a smooth dipole impedance curve
// should match the exact sweep within a tight tolerance everywhere.
func TestSweep_InterpolatedMatchesExact(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	geom := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: -halfL, Z1: 0,
			X2: 0, Y2: halfL, Z2: 0,
			Radius: 1e-3, Segments: 11,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}
	exact, err := SweepWithOptions(geom, 12e6, 16e6, 41, SweepOptions{Mode: SweepModeExact})
	if err != nil {
		t.Fatalf("exact sweep: %v", err)
	}
	interp, err := SweepWithOptions(geom, 12e6, 16e6, 41, SweepOptions{Mode: SweepModeInterpolated, Anchors: 9})
	if err != nil {
		t.Fatalf("interpolated sweep: %v", err)
	}
	if len(exact.SWR) != len(interp.SWR) {
		t.Fatalf("length mismatch: exact %d, interp %d", len(exact.SWR), len(interp.SWR))
	}
	// Allow ~5 ohm mean drift on R/X for a smooth dipole curve fitted
	// with 9 anchors over 41 points.  Should be much better in practice.
	var maxR, maxX float64
	for i := range exact.Impedance {
		dR := math.Abs(exact.Impedance[i].R - interp.Impedance[i].R)
		dX := math.Abs(exact.Impedance[i].X - interp.Impedance[i].X)
		if dR > maxR {
			maxR = dR
		}
		if dX > maxX {
			maxX = dX
		}
	}
	if maxR > 5 {
		t.Errorf("max ΔR between interpolated and exact: %.3f ohm (want < 5)", maxR)
	}
	if maxX > 5 {
		t.Errorf("max ΔX: %.3f ohm (want < 5)", maxX)
	}
	t.Logf("max drift R = %.3f, X = %.3f", maxR, maxX)
}

func TestSweep_AutoModePicksInterpolatedForLargeSweeps(t *testing.T) {
	geom := SimulationInput{
		Frequency: 14e6,
		Wires: []Wire{{
			X1: 0, Y1: -5, Z1: 0, X2: 0, Y2: 5, Z2: 0,
			Radius: 1e-3, Segments: 11,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 5, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}
	res, err := Sweep(geom, 13e6, 15e6, 50) // 50 steps > threshold of 32
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	hasInterp := false
	for _, w := range res.Warnings {
		if w.Code == "sweep_interpolated" {
			hasInterp = true
		}
	}
	if !hasInterp {
		t.Fatal("expected sweep_interpolated info note for large sweep")
	}
}
