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
	"math"
	"sync"
)

// glCache stores precomputed Gauss-Legendre nodes and weights keyed by order n.
// A sync.Map is used because the solver's parallel workers may request quadrature
// rules concurrently for different orders.
var glCache sync.Map

// glData holds a cached set of Gauss-Legendre quadrature nodes and weights.
type glData struct {
	nodes   []float64 // quadrature abscissae on [-1, 1]
	weights []float64 // corresponding quadrature weights
}

// GaussLegendre returns n-point Gauss-Legendre quadrature nodes and weights on [-1, 1].
//
// Gauss-Legendre quadrature exactly integrates polynomials of degree up to 2n-1
// and is used throughout the MoM solver to evaluate the double integrals in the
// impedance matrix (Green's function kernels over segment pairs).
//
// The algorithm finds the roots of the n-th Legendre polynomial P_n(x) via
// Newton's method, starting from Chebyshev approximations as initial guesses.
// Only the positive half of the roots are computed; the rest follow from the
// symmetry P_n(-x) = (-1)^n P_n(x). Results are cached in glCache so that
// repeated calls with the same n avoid recomputation.
func GaussLegendre(n int) (nodes []float64, weights []float64) {
	// Return cached result if available
	if val, ok := glCache.Load(n); ok {
		d := val.(*glData)
		return d.nodes, d.weights
	}

	nodes = make([]float64, n)
	weights = make([]float64, n)

	// Only compute ceil(n/2) roots; the rest are placed by symmetry
	m := (n + 1) / 2

	for i := 0; i < m; i++ {
		// Initial guess: Chebyshev approximation to the i-th root of P_n
		x := math.Cos(math.Pi * (float64(i) + 0.75) / (float64(n) + 0.5))

		// Newton iterations to refine root of P_n(x) = 0
		for iter := 0; iter < 200; iter++ {
			// Evaluate P_n(x) via the three-term recurrence relation:
			//   (j+1)*P_{j+1}(x) = (2j+1)*x*P_j(x) - j*P_{j-1}(x)
			// We track two consecutive Legendre polynomials to get both
			// P_n(x) (for the function value) and P_{n-1}(x) (for the derivative).
			p0 := 1.0 // P_0(x)
			p1 := x   // P_1(x)
			for j := 2; j <= n; j++ {
				p2 := ((2.0*float64(j)-1.0)*x*p1 - (float64(j)-1.0)*p0) / float64(j)
				p0 = p1
				p1 = p2
			}
			// After the loop: p1 = P_n(x), p0 = P_{n-1}(x)

			// Derivative formula: P_n'(x) = n*(x*P_n(x) - P_{n-1}(x)) / (x^2 - 1)
			dp := float64(n) * (x*p1 - p0) / (x*x - 1.0)

			// Newton step: x_{k+1} = x_k - P_n(x_k) / P_n'(x_k)
			dx := -p1 / dp
			x += dx
			if math.Abs(dx) < 1e-15 {
				break
			}
		}

		// Compute the quadrature weight at this root.
		// Formula: w_i = 2 / ((1 - x_i^2) * [P_n'(x_i)]^2)
		// We re-evaluate P_n and its derivative at the converged root.
		p0 := 1.0
		p1 := x
		for j := 2; j <= n; j++ {
			p2 := ((2.0*float64(j)-1.0)*x*p1 - (float64(j)-1.0)*p0) / float64(j)
			p0 = p1
			p1 = p2
		}
		dp := float64(n) * (x*p1 - p0) / (x*x - 1.0)
		w := 2.0 / ((1.0 - x*x) * dp * dp)

		// Place roots and weights symmetrically about the origin
		nodes[i] = -x
		nodes[n-1-i] = x
		weights[i] = w
		weights[n-1-i] = w
	}

	glCache.Store(n, &glData{nodes: nodes, weights: weights})
	return nodes, weights
}
