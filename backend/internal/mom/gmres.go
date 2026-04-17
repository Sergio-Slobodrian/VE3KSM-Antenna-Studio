// Package mom — GMRES iterative solver for complex linear systems.
//
// This file implements restarted GMRES (Generalised Minimal Residual)
// for solving Z·I = V where Z is a dense complex impedance matrix.
// For large basis counts (N > ~150) the O(N²) iterative solve per
// restart cycle is far cheaper than O(N³) dense LU, especially when
// the system converges in a few dozen iterations.
//
// A diagonal (Jacobi) preconditioner is applied by default: each
// equation is scaled by 1/Z[i][i].  This is cheap and effective for
// MoM impedance matrices where the diagonal self-impedance dominates.
package mom

import (
	"fmt"
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/mat"
)

// GMRESOptions configures the iterative solver.
type GMRESOptions struct {
	// MaxIter is the maximum number of outer restarts × inner Krylov
	// iterations.  0 = use default (N * 3).
	MaxIter int

	// Restart is the Krylov subspace dimension before restart.
	// 0 = use default (min(N, 50)).
	Restart int

	// Tol is the relative residual tolerance ‖r‖/‖b‖.
	// 0 = use default (1e-8).
	Tol float64

	// UsePrecon enables diagonal (Jacobi) preconditioning.
	// Default true when left at zero-value.
	UsePrecon *bool
}

// GMRESResult captures solver convergence metadata for diagnostics.
type GMRESResult struct {
	Iterations   int
	FinalResNorm float64
	Converged    bool
}

// complexVecNorm returns the 2-norm of a complex vector.
func complexVecNorm(v []complex128) float64 {
	var s float64
	for _, c := range v {
		s += real(c)*real(c) + imag(c)*imag(c)
	}
	return math.Sqrt(s)
}

// complexVecScale sets dst[i] = alpha * src[i].
func complexVecScale(dst []complex128, alpha complex128, src []complex128) {
	for i := range src {
		dst[i] = alpha * src[i]
	}
}

// complexVecAxpy computes y[i] += alpha * x[i].
func complexVecAxpy(y []complex128, alpha complex128, x []complex128) {
	for i := range x {
		y[i] += alpha * x[i]
	}
}

// complexDot computes the conjugate dot product: sum(conj(a[i]) * b[i]).
func complexDot(a, b []complex128) complex128 {
	var s complex128
	for i := range a {
		s += cmplx.Conj(a[i]) * b[i]
	}
	return s
}

// matvec computes dst = Z * x for a dense complex matrix Z.
func matvec(Z *mat.CDense, x, dst []complex128) {
	n, _ := Z.Dims()
	for i := 0; i < n; i++ {
		var s complex128
		for j := 0; j < n; j++ {
			s += Z.At(i, j) * x[j]
		}
		dst[i] = s
	}
}

// diagPrecon extracts the diagonal of Z and returns 1/Z[i][i] for each i.
// If any diagonal element is zero (or near-zero), falls back to 1.
func diagPrecon(Z *mat.CDense) []complex128 {
	n, _ := Z.Dims()
	inv := make([]complex128, n)
	for i := 0; i < n; i++ {
		d := Z.At(i, i)
		if cmplx.Abs(d) < 1e-30 {
			inv[i] = 1
		} else {
			inv[i] = 1 / d
		}
	}
	return inv
}

// applyPrecon computes dst[i] = precon[i] * src[i] (left Jacobi preconditioner).
func applyPrecon(dst, precon, src []complex128) {
	for i := range src {
		dst[i] = precon[i] * src[i]
	}
}

// SolveGMRES solves Z·I = V using restarted preconditioned GMRES.
// Returns the solution vector and convergence metadata.
func SolveGMRES(Z *mat.CDense, V []complex128, n int, opts GMRESOptions) ([]complex128, *GMRESResult, error) {
	// ---- Defaults ----
	restart := opts.Restart
	if restart <= 0 {
		restart = n
		if restart > 50 {
			restart = 50
		}
	}
	tol := opts.Tol
	if tol <= 0 {
		tol = 1e-8
	}
	maxIter := opts.MaxIter
	if maxIter <= 0 {
		maxIter = n * 3
		if maxIter < 200 {
			maxIter = 200
		}
	}
	usePrecon := true
	if opts.UsePrecon != nil {
		usePrecon = *opts.UsePrecon
	}

	// ---- Preconditioner ----
	var precon []complex128
	if usePrecon {
		precon = diagPrecon(Z)
	}

	// ---- Initial guess: x = 0 ----
	x := make([]complex128, n)
	r := make([]complex128, n) // residual
	tmp := make([]complex128, n)

	// r = V - Z*x = V (since x = 0)
	copy(r, V)

	bnorm := complexVecNorm(V)
	if bnorm == 0 {
		// Trivial: V = 0 → solution is 0.
		return x, &GMRESResult{Converged: true}, nil
	}

	totalIter := 0
	var finalRes float64

	for outer := 0; outer < maxIter; outer++ {
		// Apply preconditioner to residual if enabled.
		w := make([]complex128, n)
		if usePrecon {
			applyPrecon(w, precon, r)
		} else {
			copy(w, r)
		}

		beta := complexVecNorm(w)
		if beta/bnorm < tol {
			finalRes = beta / bnorm
			return x, &GMRESResult{Iterations: totalIter, FinalResNorm: finalRes, Converged: true}, nil
		}

		// Arnoldi/GMRES inner iteration.
		// V_k: Krylov basis vectors (restart+1 vectors of length n).
		// H: upper Hessenberg matrix ((restart+1) × restart).
		m := restart
		Vk := make([][]complex128, m+1)
		Vk[0] = make([]complex128, n)
		complexVecScale(Vk[0], complex(1.0/beta, 0), w)

		H := make([][]complex128, m+1)
		for i := range H {
			H[i] = make([]complex128, m)
		}

		// Givens rotation coefficients.
		cs := make([]complex128, m)
		sn := make([]complex128, m)
		g := make([]complex128, m+1) // RHS of the least-squares problem
		g[0] = complex(beta, 0)

		j := 0
		for ; j < m; j++ {
			totalIter++
			if totalIter > maxIter {
				break
			}

			// w = Z * Vk[j]
			matvec(Z, Vk[j], tmp)
			if usePrecon {
				applyPrecon(w, precon, tmp)
			} else {
				copy(w, tmp)
			}

			// Modified Gram-Schmidt orthogonalisation.
			for i := 0; i <= j; i++ {
				H[i][j] = complexDot(Vk[i], w)
				complexVecAxpy(w, -H[i][j], Vk[i])
			}
			H[j+1][j] = complex(complexVecNorm(w), 0)
			hjj1 := cmplx.Abs(H[j+1][j])
			if hjj1 < 1e-30 {
				// Lucky breakdown: exact solution found in Krylov subspace.
				j++
				break
			}
			Vk[j+1] = make([]complex128, n)
			complexVecScale(Vk[j+1], 1/H[j+1][j], w)

			// Apply previous Givens rotations to new column of H.
			for i := 0; i < j; i++ {
				temp := cs[i]*H[i][j] + sn[i]*H[i+1][j]
				H[i+1][j] = -cmplx.Conj(sn[i])*H[i][j] + cmplx.Conj(cs[i])*H[i+1][j]
				H[i][j] = temp
			}

			// Compute new Givens rotation to eliminate H[j+1][j].
			a := H[j][j]
			b := H[j+1][j]
			denom := math.Sqrt(cmplx.Abs(a)*cmplx.Abs(a) + cmplx.Abs(b)*cmplx.Abs(b))
			cs[j] = a / complex(denom, 0)
			sn[j] = b / complex(denom, 0)

			H[j][j] = cs[j]*a + sn[j]*b
			H[j+1][j] = 0

			// Apply rotation to g.
			temp := cs[j] * g[j]
			g[j+1] = -cmplx.Conj(sn[j]) * g[j]
			g[j] = temp

			// Check convergence.
			resNorm := cmplx.Abs(g[j+1]) / bnorm
			if resNorm < tol {
				j++
				break
			}
		}

		// Solve the upper triangular system H * y = g.
		k := j // number of columns filled
		y := make([]complex128, k)
		for i := k - 1; i >= 0; i-- {
			y[i] = g[i]
			for l := i + 1; l < k; l++ {
				y[i] -= H[i][l] * y[l]
			}
			y[i] /= H[i][i]
		}

		// Update solution: x = x + Vk * y.
		for i := 0; i < k; i++ {
			complexVecAxpy(x, y[i], Vk[i])
		}

		// Recompute residual: r = V - Z*x.
		matvec(Z, x, tmp)
		for i := 0; i < n; i++ {
			r[i] = V[i] - tmp[i]
		}
		finalRes = complexVecNorm(r) / bnorm
		if finalRes < tol {
			return x, &GMRESResult{Iterations: totalIter, FinalResNorm: finalRes, Converged: true}, nil
		}
	}

	// Did not converge — return the best solution anyway with a warning.
	return x, &GMRESResult{Iterations: totalIter, FinalResNorm: finalRes, Converged: false},
		fmt.Errorf("GMRES did not converge after %d iterations (residual %.2e, tol %.2e)", totalIter, finalRes, tol)
}
