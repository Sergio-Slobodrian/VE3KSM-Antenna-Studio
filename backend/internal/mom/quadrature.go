package mom

import (
	"math"
	"sync"
)

// glCache stores precomputed Gauss-Legendre nodes and weights keyed by order n.
var glCache sync.Map

type glData struct {
	nodes   []float64
	weights []float64
}

// GaussLegendre returns n-point Gauss-Legendre quadrature nodes and weights on [-1, 1].
// Uses the standard recurrence relation to find roots of Legendre polynomials via
// Newton's method. Results are cached for repeated calls with the same n.
func GaussLegendre(n int) (nodes []float64, weights []float64) {
	if val, ok := glCache.Load(n); ok {
		d := val.(*glData)
		return d.nodes, d.weights
	}

	nodes = make([]float64, n)
	weights = make([]float64, n)

	// Compute roots of the n-th Legendre polynomial using Newton's method
	m := (n + 1) / 2 // number of positive roots (exploiting symmetry)

	for i := 0; i < m; i++ {
		// Initial guess using the Chebyshev approximation
		x := math.Cos(math.Pi * (float64(i) + 0.75) / (float64(n) + 0.5))

		for iter := 0; iter < 200; iter++ {
			// Evaluate P_n(x) and P_{n-1}(x) via recurrence:
			// (j+1)*P_{j+1}(x) = (2j+1)*x*P_j(x) - j*P_{j-1}(x)
			p0 := 1.0 // P_0
			p1 := x   // P_1
			for j := 2; j <= n; j++ {
				p2 := ((2.0*float64(j)-1.0)*x*p1 - (float64(j)-1.0)*p0) / float64(j)
				p0 = p1
				p1 = p2
			}
			// p1 = P_n(x), p0 = P_{n-1}(x)

			// Derivative: P_n'(x) = n*(x*P_n(x) - P_{n-1}(x)) / (x^2 - 1)
			dp := float64(n) * (x*p1 - p0) / (x*x - 1.0)

			dx := -p1 / dp
			x += dx
			if math.Abs(dx) < 1e-15 {
				break
			}
		}

		// Weight: w_i = 2 / ((1 - x_i^2) * [P_n'(x_i)]^2)
		p0 := 1.0
		p1 := x
		for j := 2; j <= n; j++ {
			p2 := ((2.0*float64(j)-1.0)*x*p1 - (float64(j)-1.0)*p0) / float64(j)
			p0 = p1
			p1 = p2
		}
		dp := float64(n) * (x*p1 - p0) / (x*x - 1.0)
		w := 2.0 / ((1.0 - x*x) * dp * dp)

		// Place roots symmetrically
		nodes[i] = -x
		nodes[n-1-i] = x
		weights[i] = w
		weights[n-1-i] = w
	}

	glCache.Store(n, &glData{nodes: nodes, weights: weights})
	return nodes, weights
}
