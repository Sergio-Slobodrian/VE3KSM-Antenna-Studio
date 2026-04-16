package match

import (
	"math"
	"math/cmplx"
	"testing"
)

// helper: complex evaluation of "load + L-net + series" to verify a
// network really transforms LoadZ to SourceZ0 at the design freq.
func evalSeriesShunt(load complex128, comps []Component, freqHz float64) complex128 {
	omega := 2 * math.Pi * freqHz
	z := load
	// Walk the network from load toward source.  Components are listed
	// in source-to-load order; reverse iterate.
	for i := len(comps) - 1; i >= 0; i-- {
		c := comps[i]
		var jX complex128
		if c.Kind == "L" {
			jX = complex(0, omega*c.Value)
		} else if c.Kind == "C" {
			if c.Value <= 0 {
				continue
			}
			jX = complex(0, -1/(omega*c.Value))
		}
		switch c.Position {
		case "series":
			z += jX
		case "shunt":
			y := 1/z + 1/jX
			z = 1 / y
		}
	}
	return z
}

func TestL_StepDown_75to50(t *testing.T) {
	r, err := All(Request{LoadR: 75, FreqHz: 14e6, SourceZ0: 50})
	if err != nil {
		t.Fatal(err)
	}
	var L Solution
	for _, s := range r.Solutions {
		if s.Topology == "L" && len(s.Components) > 0 {
			L = s
			break
		}
	}
	if len(L.Components) != 2 {
		t.Fatalf("L solution missing")
	}
	z := evalSeriesShunt(complex(75, 0), L.Components, 14e6)
	if math.Abs(real(z)-50) > 5 || math.Abs(imag(z)) > 10 {
		t.Fatalf("L-network output not near 50+j0: got %v", z)
	}
}

func TestPi_HighRatio(t *testing.T) {
	r, err := All(Request{LoadR: 500, FreqHz: 14e6, SourceZ0: 50, QFactor: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range r.Solutions {
		if s.Topology == "pi" && len(s.Components) == 3 {
			return
		}
	}
	t.Fatal("pi solution missing")
}

func TestT_LowRatio(t *testing.T) {
	r, _ := All(Request{LoadR: 10, FreqHz: 14e6, SourceZ0: 50, QFactor: 8})
	for _, s := range r.Solutions {
		if s.Topology == "T" && len(s.Components) == 3 {
			return
		}
	}
	t.Fatal("T solution missing")
}

func TestGamma_HigherR(t *testing.T) {
	r, _ := All(Request{LoadR: 200, LoadX: 0, FreqHz: 14e6, SourceZ0: 50})
	for _, s := range r.Solutions {
		if s.Topology == "gamma" && len(s.Components) > 0 {
			return
		}
	}
	t.Fatal("gamma solution missing")
}

func TestGamma_RejectedWhenLoadTooLow(t *testing.T) {
	r, _ := All(Request{LoadR: 30, LoadX: 0, FreqHz: 14e6, SourceZ0: 50})
	for _, s := range r.Solutions {
		if s.Topology == "gamma" && len(s.Components) > 0 {
			t.Fatalf("gamma should not produce a solution for R=30 vs Z0=50")
		}
	}
}

func TestBeta_Capacitive(t *testing.T) {
	r, _ := All(Request{LoadR: 25, LoadX: -30, FreqHz: 14e6, SourceZ0: 50})
	for _, s := range r.Solutions {
		if s.Topology == "beta" && len(s.Components) > 0 {
			return
		}
	}
	t.Fatal("beta solution missing for capacitive load")
}

func TestBeta_RejectedWhenInductive(t *testing.T) {
	r, _ := All(Request{LoadR: 25, LoadX: +30, FreqHz: 14e6, SourceZ0: 50})
	for _, s := range r.Solutions {
		if s.Topology == "beta" && len(s.Components) > 0 {
			t.Fatalf("beta should not produce a solution for inductive load")
		}
	}
}

func TestComponentFromX_PositiveIsInductor(t *testing.T) {
	c := componentFromX(50, 14e6, "series", "")
	if c.Kind != "L" {
		t.Fatalf("X=+50 should be L, got %q", c.Kind)
	}
	want := 50 / (2 * math.Pi * 14e6)
	if math.Abs(c.Value-want)/want > 1e-9 {
		t.Fatalf("L value: got %v, want %v", c.Value, want)
	}
}

func TestComponentFromX_NegativeIsCapacitor(t *testing.T) {
	c := componentFromX(-50, 14e6, "shunt", "")
	if c.Kind != "C" {
		t.Fatalf("X=-50 should be C, got %q", c.Kind)
	}
	want := 1 / (2 * math.Pi * 14e6 * 50)
	if math.Abs(c.Value-want)/want > 1e-9 {
		t.Fatalf("C value: got %v, want %v", c.Value, want)
	}
}

// Sanity: All returns one entry per topology even when some are skipped.
func TestAll_AlwaysReturnsAllTopologies(t *testing.T) {
	r, _ := All(Request{LoadR: 30, LoadX: +10, FreqHz: 14e6, SourceZ0: 50})
	if len(r.Solutions) != 5 {
		t.Fatalf("want 5 solution slots, got %d", len(r.Solutions))
	}
	_ = cmplx.Abs(0) // silence import
}
