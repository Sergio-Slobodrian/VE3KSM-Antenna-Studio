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
	"runtime"
	"sync"

	"gonum.org/v1/gonum/mat"
)

// Electromagnetic constants used throughout the MoM solver.
const (
	Mu0  = 4.0 * math.Pi * 1e-7  // permeability of free space, μ₀ = 4π×10⁻⁷ (H/m)
	C0   = 299792458.0            // speed of light in vacuum (m/s)
	Eps0 = 1.0 / (Mu0 * C0 * C0) // permittivity of free space, ε₀ = 1/(μ₀c²) ≈ 8.854×10⁻¹² (F/m)
	Eta0 = Mu0 * C0               // intrinsic impedance of free space, η₀ = μ₀c ≈ 376.73 (Ω)
)

// BuildZMatrix assembles the N x N complex impedance matrix using the legacy
// Pocklington kernel with pulse basis functions.
//
// NOTE: This function is no longer used in the main simulation path, which has
// been upgraded to triangle (rooftop) basis functions via buildTriangleZMatrix
// in solver.go. It is retained here because other code references the
// electromagnetic constants defined in this file, and for potential future use
// with alternative formulations.
//
// Each element Z[i][j] represents the voltage induced on segment i due to a
// unit current on segment j:
//
//	Z[i][j] = jωμ₀/(4π) · ∫_i ∫_j K(s,s') ds ds'
//
// where K is the Pocklington kernel. Self-terms (i==j) use the reduced kernel
// (thin-wire approximation) with the wire radius offset. The matrix fill is
// parallelized across runtime.NumCPU() goroutine workers.
func BuildZMatrix(segments []Segment, k float64, omega float64) *mat.CDense {
	n := len(segments)
	Z := mat.NewCDense(n, n, nil)

	// EFIE prefactor: jωμ₀/(4π). The kernel ψ = exp(-jkR)/R omits the 1/(4π)
	// normalization, so it is included here instead.
	prefactor := complex(0, omega*Mu0/(4.0*math.Pi))

	// Parallel worker pool to fill the N² matrix entries concurrently
	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}

	type job struct {
		i, j int
	}

	jobs := make(chan job, 256)
	var wg sync.WaitGroup
	var mu sync.Mutex // protects CDense.Set which is not guaranteed thread-safe

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for jb := range jobs {
				i, j := jb.i, jb.j
				// Use reduced kernel (radius offset) for self-terms to regularize 1/R singularity
				reduced := (i == j)
				kernel := PocklingtonKernel(k, segments[i], segments[j], reduced)
				val := prefactor * kernel

				mu.Lock()
				Z.Set(i, j, val)
				mu.Unlock()
			}
		}()
	}

	// Enqueue all N² matrix entries
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			jobs <- job{i, j}
		}
	}
	close(jobs)
	wg.Wait()

	return Z
}
