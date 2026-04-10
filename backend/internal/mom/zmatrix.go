package mom

import (
	"math"
	"runtime"
	"sync"

	"gonum.org/v1/gonum/mat"
)

// Electromagnetic constants
const (
	Mu0  = 4.0 * math.Pi * 1e-7 // permeability of free space (H/m)
	C0   = 299792458.0           // speed of light (m/s)
	Eps0 = 1.0 / (Mu0 * C0 * C0) // permittivity of free space (F/m)
	Eta0 = Mu0 * C0              // free space impedance ~376.73 ohms
)

// BuildZMatrix assembles the N x N complex impedance matrix for the given segments.
// k is the wavenumber (2*pi*f/c), omega is the angular frequency (2*pi*f).
//
// Each element Z[i][j] is computed as:
//
//	Z[i][j] = j*omega*mu0/(4*pi) * integral_i integral_j K(s,s') ds ds'
//
// where K is the Pocklington kernel. The prefactor j*omega*mu0/(4*pi) is applied here.
// Self-terms (i==j) use the reduced kernel with higher quadrature order.
// Off-diagonal terms use standard quadrature.
//
// Computation is parallelized using a goroutine worker pool with runtime.NumCPU workers.
func BuildZMatrix(segments []Segment, k float64, omega float64) *mat.CDense {
	n := len(segments)
	Z := mat.NewCDense(n, n, nil)

	// Prefactor: j * omega * mu0 / (4*pi)
	// The kernel uses ψ = exp(-jkR)/R (without the 1/(4π) factor).
	prefactor := complex(0, omega*Mu0/(4.0*math.Pi))

	// Worker pool for parallel impedance matrix fill
	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}

	type job struct {
		i, j int
	}

	jobs := make(chan job, 256)
	var wg sync.WaitGroup

	// Mutex for CDense.Set since it's not documented as thread-safe
	var mu sync.Mutex

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for jb := range jobs {
				i, j := jb.i, jb.j
				reduced := (i == j)
				kernel := PocklingtonKernel(k, segments[i], segments[j], reduced)
				val := prefactor * kernel

				mu.Lock()
				Z.Set(i, j, val)
				mu.Unlock()
			}
		}()
	}

	// Enqueue all matrix entries
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			jobs <- job{i, j}
		}
	}
	close(jobs)
	wg.Wait()

	return Z
}
