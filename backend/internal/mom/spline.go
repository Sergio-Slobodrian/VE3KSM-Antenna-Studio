package mom

import "fmt"

// Spline is a monotone-preserving piecewise cubic Hermite interpolator
// (PCHIP, Fritsch-Carlson 1980).  Unlike a natural cubic spline it does
// not overshoot around rapid swings -- so a frequency-vs-impedance sweep
// with a sharp anti-resonance will produce a clean curve without the
// oscillatory "sawtooth" artefacts a smoother but unconstrained spline
// can introduce.
type Spline struct {
	x, y, d []float64 // knots, values, slopes at each knot
}

// NewSpline returns a PCHIP interpolant through the given points.
// Requires len(x) >= 2 with strictly increasing x.
func NewSpline(x, y []float64) (*Spline, error) {
	n := len(x)
	if n < 2 {
		return nil, fmt.Errorf("spline needs at least 2 points, got %d", n)
	}
	if len(y) != n {
		return nil, fmt.Errorf("x and y must have the same length")
	}
	for i := 1; i < n; i++ {
		if x[i] <= x[i-1] {
			return nil, fmt.Errorf("x must be strictly increasing at index %d", i)
		}
	}
	d := make([]float64, n)

	if n == 2 {
		s := (y[1] - y[0]) / (x[1] - x[0])
		d[0], d[1] = s, s
		return &Spline{x: x, y: y, d: d}, nil
	}

	// Secant slopes between consecutive knots.
	h := make([]float64, n-1)
	m := make([]float64, n-1)
	for i := 0; i < n-1; i++ {
		h[i] = x[i+1] - x[i]
		m[i] = (y[i+1] - y[i]) / h[i]
	}

	// Fritsch-Carlson interior slopes: weighted harmonic mean when both
	// secant slopes have the same sign, zero otherwise (preserves
	// monotonicity).
	for i := 1; i < n-1; i++ {
		if m[i-1]*m[i] <= 0 {
			d[i] = 0
			continue
		}
		w1 := 2*h[i] + h[i-1]
		w2 := h[i] + 2*h[i-1]
		d[i] = (w1 + w2) / (w1/m[i-1] + w2/m[i])
	}

	// One-sided endpoint slopes (Fritsch-Butland 1984), clipped to keep
	// the endpoint segment monotone.
	d[0] = pchipEdge(m[0], m[1], h[0], h[1])
	d[n-1] = pchipEdge(m[n-2], m[n-3], h[n-2], h[n-3])

	return &Spline{x: x, y: y, d: d}, nil
}

func pchipEdge(m0, m1, h0, h1 float64) float64 {
	d := ((2*h0+h1)*m0 - h0*m1) / (h0 + h1)
	// Clip per Fritsch-Carlson to keep the boundary segment monotone.
	if d*m0 <= 0 {
		return 0
	}
	if m0*m1 <= 0 && abs(d) > 3*abs(m0) {
		return 3 * m0
	}
	return d
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// Eval returns the spline value at xq.  Out-of-range queries clamp to
// the endpoint value (constant extrapolation).
func (s *Spline) Eval(xq float64) float64 {
	n := len(s.x)
	if xq <= s.x[0] {
		return s.y[0]
	}
	if xq >= s.x[n-1] {
		return s.y[n-1]
	}
	lo, hi := 0, n-1
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if s.x[mid] > xq {
			hi = mid
		} else {
			lo = mid
		}
	}
	h := s.x[hi] - s.x[lo]
	t := (xq - s.x[lo]) / h
	t2 := t * t
	t3 := t2 * t
	// Hermite basis functions.
	h00 := 2*t3 - 3*t2 + 1
	h10 := t3 - 2*t2 + t
	h01 := -2*t3 + 3*t2
	h11 := t3 - t2
	return h00*s.y[lo] + h10*h*s.d[lo] + h01*s.y[hi] + h11*h*s.d[hi]
}
