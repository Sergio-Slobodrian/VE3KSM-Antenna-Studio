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

// sommerfeld.go implements the full Sommerfeld half-space integration for
// rigorous treatment of antennas over lossy ground.
//
// The scattered (ground-reflected) part of the MPIE Green's function for a
// horizontal source above a planar lossy earth interface is expressed exactly
// as a semi-infinite integral over the horizontal wavenumber λ:
//
//	I₀(ρ,z) = ∫₀^∞  ½[R_TM(λ)+R_TE(λ)] · (λ/γ₀) · J₀(λρ) · exp(−γ₀z) dλ
//	I₂(ρ,z) = ∫₀^∞  ½[R_TM(λ)−R_TE(λ)] · (λ/γ₀) · J₂(λρ) · exp(−γ₀z) dλ
//	IΦ(ρ,z) = ∫₀^∞   R_TM(λ)            · (λ³/γ₀k₀²) · J₀(λρ) · exp(−γ₀z) dλ
//
// where:
//
//	γ₀(λ) = √(λ²−k₀²)  vertical wavenumber in air  (Re(γ₀) ≥ 0)
//	γ₁(λ) = √(λ²−k₁²)  vertical wavenumber in ground, k₁ = k₀√εc
//	R_TM  = (εc·γ₀ − γ₁) / (εc·γ₀ + γ₁)   TM (vertical-E) reflection coefficient
//	R_TE  = (γ₀ − γ₁)    / (γ₀ + γ₁)       TE (horizontal-E) reflection coefficient
//
// The vector potential scattered kernel for a horizontal source direction ŝ is:
//
//	G_A^s = I₀ + I₂·cos(2φ₀)
//
// where φ₀ is the azimuthal angle between ŝ and the horizontal source–observer
// separation vector.  IΦ gives the scalar potential (charge) scattered kernel.
//
// Numerical integration uses three-interval 32-point Gauss-Legendre quadrature
// on λ ∈ [0, 15·k₀], with a small imaginary path deformation near the branch
// point at λ = k₀ to avoid the square-root singularity.  Results are cached
// by geometry/frequency key so the same (ρ, z_sum) pair is never integrated
// more than once per simulation.
//
// References:
//   - Sommerfeld (1926) Ann. Phys. 81:1135
//   - Michalski & Mosig (1997) IEEE Trans. AP-45(3):508-519
//   - NEC-2 Manual, Burke & Poggio (1981), Sec. III
package mom

import (
	"fmt"
	"math"
	"math/cmplx"
	"sync"
)

// sommerfeldCacheKey uniquely identifies a Sommerfeld integral evaluation.
type sommerfeldCacheKey struct {
	rho, zsum float64
	k0        float64
	sigma     float64
	epsilonR  float64
}

// sommerfeldCacheVal holds all three precomputed integrals for one key.
type sommerfeldCacheVal struct {
	i0, i2, iPhi complex128
}

var sommerfeldCache sync.Map // map[sommerfeldCacheKey]sommerfeldCacheVal

// gamma0 computes the vertical wavenumber in air: γ₀ = √(λ²−k₀²).
// The branch cut is chosen so that Re(γ₀) ≥ 0 (evanescent upward decay).
func gamma0(lambda complex128, k0 float64) complex128 {
	k0c := complex(k0, 0)
	v := lambda*lambda - k0c*k0c
	g := cmplx.Sqrt(v)
	// Ensure Re(γ₀) ≥ 0 for evanescent / outgoing wave convention.
	if real(g) < 0 {
		g = -g
	}
	return g
}

// gamma1 computes the vertical wavenumber in ground: γ₁ = √(λ²−k₁²),
// k₁ = k₀·√εc.  Branch cut: Re(γ₁) ≥ 0.
func gamma1(lambda complex128, k0 float64, epsilonC complex128) complex128 {
	k1sq := complex(k0*k0, 0) * epsilonC
	v := lambda*lambda - k1sq
	g := cmplx.Sqrt(v)
	if real(g) < 0 {
		g = -g
	}
	return g
}

// reflTM returns the TM (vertical, p-polarised) reflection coefficient at
// horizontal wavenumber λ: R_TM = (εc·γ₀ − γ₁) / (εc·γ₀ + γ₁).
func reflTM(lambda complex128, k0 float64, epsilonC complex128) complex128 {
	g0 := gamma0(lambda, k0)
	g1 := gamma1(lambda, k0, epsilonC)
	num := epsilonC*g0 - g1
	den := epsilonC*g0 + g1
	if cmplx.Abs(den) < 1e-30 {
		return 0
	}
	return num / den
}

// reflTE returns the TE (horizontal, s-polarised) reflection coefficient:
// R_TE = (γ₀ − γ₁) / (γ₀ + γ₁).
func reflTE(lambda complex128, k0 float64, epsilonC complex128) complex128 {
	g0 := gamma0(lambda, k0)
	g1 := gamma1(lambda, k0, epsilonC)
	num := g0 - g1
	den := g0 + g1
	if cmplx.Abs(den) < 1e-30 {
		return 0
	}
	return num / den
}

// besselJ0 evaluates the Bessel function J₀(x) for real x ≥ 0 using the
// polynomial approximations from Abramowitz & Stegun §9.4 (max error < 5e-8).
func besselJ0(x float64) float64 {
	if x < 0 {
		x = -x
	}
	if x <= 3.0 {
		t := x * x / 9.0
		return 1 + t*(-2.2499997+t*(1.2656208+t*(-0.3163866+t*(0.0444479+t*(-0.0039444+t*0.0002100)))))
	}
	t := 3.0 / x
	f0 := 0.79788456 + t*(-0.00000077+t*(-0.00552740+t*(-0.00009512+t*(0.00137237+t*(-0.00072805+t*0.00014476)))))
	theta0 := x - 0.78539816 + t*(-0.04166397+t*(-0.00003954+t*(0.00262573+t*(-0.00054125+t*(-0.00029333+t*0.00013558)))))
	return f0 / math.Sqrt(x) * math.Cos(theta0)
}

// besselJ1 evaluates J₁(x) for real x ≥ 0 (A&S §9.4, max error < 5e-8).
func besselJ1(x float64) float64 {
	sign := 1.0
	if x < 0 {
		x = -x
		sign = -1
	}
	var v float64
	if x <= 3.0 {
		t := x * x / 9.0
		v = x * (0.5 + t*(-0.56249985+t*(0.21093573+t*(-0.03954289+t*(0.00443319+t*(-0.00031761+t*0.00001109))))))
	} else {
		t := 3.0 / x
		f1 := 0.79788456 + t*(0.00000156+t*(0.01659667+t*(0.00017105+t*(-0.00249511+t*(0.00113653+t*(-0.00020033))))))
		theta1 := x - 2.35619449 + t*(0.12499612+t*(0.00005650+t*(-0.00637879+t*(0.00074348+t*(0.00079824+t*(-0.00029166))))))
		v = f1 / math.Sqrt(x) * math.Cos(theta1)
	}
	return sign * v
}

// besselJ2 evaluates J₂(x) = (2/x)·J₁(x) − J₀(x).
func besselJ2(x float64) float64 {
	if x == 0 {
		return 0
	}
	return (2.0/x)*besselJ1(x) - besselJ0(x)
}

// sommerfeldIntegrand evaluates the three Sommerfeld integrand kernels at a
// single complex wavenumber λ for geometry (ρ, z_sum).  Returns (f0, f2, fPhi)
// where the prefactor j/(4π) is NOT included (applied by the caller).
func sommerfeldIntegrand(lambda complex128, rho, zsum, k0 float64, epsilonC complex128) (f0, f2, fPhi complex128) {
	g0 := gamma0(lambda, k0)

	// Protect against γ₀ = 0 (exactly at branch point).
	if cmplx.Abs(g0) < 1e-20 {
		return 0, 0, 0
	}

	rTM := reflTM(lambda, k0, epsilonC)
	rTE := reflTE(lambda, k0, epsilonC)

	// exp(−γ₀·z_sum) — decays for evanescent modes (Re(γ₀)·z_sum > 0)
	expFac := cmplx.Exp(-g0 * complex(zsum, 0))

	// Bessel functions at λρ (λ is complex; use real part for J evaluation
	// since Im(λ) is tiny deformation).
	lambdaRho := real(lambda) * rho
	j0 := complex(besselJ0(lambdaRho), 0)
	j2 := complex(besselJ2(lambdaRho), 0)

	lambdaOverG0 := lambda / g0
	lambda3OverG0k02 := lambda * lambda * lambda / (g0 * complex(k0*k0, 0))

	f0 = 0.5 * (rTM + rTE) * lambdaOverG0 * j0 * expFac
	f2 = 0.5 * (rTM - rTE) * lambdaOverG0 * j2 * expFac
	fPhi = rTM * lambda3OverG0k02 * j0 * expFac
	return
}

// integrateSubInterval integrates the three Sommerfeld kernels over a real-λ
// sub-interval [a, b] (with optional imaginary deformation iDelta·k₀) using
// n-point Gauss-Legendre quadrature.
func integrateSubInterval(a, b, iDelta, k0, rho, zsum float64, epsilonC complex128, n int) (s0, s2, sPhi complex128) {
	nodes, weights := GaussLegendre(n)
	mid := 0.5 * (a + b)
	half := 0.5 * (b - a)
	for q, t := range nodes {
		lambdaR := mid + half*t
		lambda := complex(lambdaR, iDelta*k0)
		f0, f2, fPhi := sommerfeldIntegrand(lambda, rho, zsum, k0, epsilonC)
		w := complex(weights[q]*half, 0)
		s0 += w * f0
		s2 += w * f2
		sPhi += w * fPhi
	}
	return
}

// SommerfeldIntegrals computes the three MPIE scattered-kernel integrals
// I₀, I₂, IΦ for horizontal-wire ground coupling.  Results are cached.
//
//	rho   — horizontal source–observer distance (m)
//	zsum  — z_obs + z_src, both heights > 0 (m)
//	k0    — free-space wavenumber (rad/m)
//	sigma — ground conductivity (S/m)
//	epsilonR — relative permittivity of ground
func SommerfeldIntegrals(rho, zsum, k0, sigma, epsilonR float64) (i0, i2, iPhi complex128) {
	// Clamp near-zero separation to avoid integrand singularity; quasi-static
	// regularisation is handled in ground_sommerfeld.go at the kernel level.
	if zsum < 1e-9 {
		zsum = 1e-9
	}

	key := sommerfeldCacheKey{
		rho:      math.Round(rho*1e9) * 1e-9,
		zsum:     math.Round(zsum*1e9) * 1e-9,
		k0:       k0,
		sigma:    sigma,
		epsilonR: epsilonR,
	}
	if v, ok := sommerfeldCache.Load(key); ok {
		cv := v.(sommerfeldCacheVal)
		return cv.i0, cv.i2, cv.iPhi
	}

	epsilonC := ComplexPermittivity(epsilonR, sigma, 2*math.Pi*k0*C0/(2*math.Pi))
	// Re-derive omega from k0: ω = k0·c
	omega := k0 * C0
	epsilonC = ComplexPermittivity(epsilonR, sigma, omega)

	// Three sub-intervals; imaginary deformation δ near the branch point.
	const nPts = 32
	const delta = 0.01 // imaginary shift fraction near branch point

	// [0, 0.8·k₀] — propagating, smooth
	s0a, s2a, sPhia := integrateSubInterval(0, 0.8*k0, 0, k0, rho, zsum, epsilonC, nPts)
	// [0.8·k₀, 1.2·k₀] — straddles branch cut; add imaginary deformation
	s0b, s2b, sPhib := integrateSubInterval(0.8*k0, 1.2*k0, delta, k0, rho, zsum, epsilonC, nPts)
	// [1.2·k₀, 15·k₀] — evanescent, decays quickly for z_sum > 0
	s0c, s2c, sPhic := integrateSubInterval(1.2*k0, 15*k0, 0, k0, rho, zsum, epsilonC, nPts)

	i0 = s0a + s0b + s0c
	i2 = s2a + s2b + s2c
	iPhi = sPhia + sPhib + sPhic

	sommerfeldCache.Store(key, sommerfeldCacheVal{i0, i2, iPhi})
	return
}

// ClearSommerfeldCache discards all cached Sommerfeld integral results.
// Call between simulations at different frequencies or ground parameters.
func ClearSommerfeldCache() {
	sommerfeldCache.Range(func(k, _ any) bool {
		sommerfeldCache.Delete(k)
		return true
	})
}

// SommerfeldCacheKey exported for testing.
func SommerfeldCacheKey(rho, zsum, k0, sigma, epsilonR float64) string {
	return fmt.Sprintf("rho=%.9g zsum=%.9g k0=%.6g sigma=%.6g er=%.6g", rho, zsum, k0, sigma, epsilonR)
}
