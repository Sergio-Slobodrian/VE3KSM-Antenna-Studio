// ground_complex_image.go implements the complex-image ground model
// for MoM antenna analysis over lossy earth.
//
// The standard Fresnel reflection-coefficient method (ground_real.go)
// approximates the half-space Green's function by a single geometric
// image at depth -z_src, scaled by a Fresnel coefficient evaluated at
// the geometric grazing angle.  This is accurate in the far field but
// breaks down for near-field interactions when source and observer are
// both close to the ground plane.
//
// The complex-image method (Bannister 1986, Lindell 1986) improves
// accuracy by replacing the geometric image depth with a complex depth
// that accounts for wave penetration into the lossy ground:
//
//     z_image_eff = -(z_src + 2·Re(d))
//
// where d = 1 / γ_g is the complex skin depth of the ground and
//
//     γ_g = j·k₀·√εc      (propagation constant in the ground medium)
//     εc  = εr - j·σ/(ω·ε₀) (complex relative permittivity)
//
// The real part of d (the penetration depth) shifts the effective image
// deeper into the ground.  This means the Fresnel reflection coefficient
// is evaluated at a MODIFIED grazing angle:
//
//     ψ_eff = atan2(z_obs + z_src + 2·Re(d), ρ)
//
// rather than the simple geometric angle ψ = atan2(z_obs + z_src, ρ).
// The modified angle is always larger than the geometric angle (steeper),
// which reduces the reflection coefficient magnitude — physically correct
// because waves must travel further through lossy ground to reach the
// deeper effective image.
//
// Additionally, the image contribution is attenuated by the ground loss
// over the extra path length through the lossy medium:
//
//     attenuation = exp(-2·α·Re(d))
//
// where α = Re(γ_g) is the attenuation constant of the ground.
//
// This hybrid approach — modified angle + attenuation factor applied to
// the standard geometric-image kernel — captures the leading Sommerfeld
// near-field correction without requiring complex-valued distances in
// the Green's function evaluation (which would cause numerical overflow).
//
// References:
//   - Bannister (1986) "Applications of complex image theory",
//     Radio Science 21(4):605-616
//   - Lindell & Alanen (1984) "Exact image theory for the Sommerfeld
//     half-space problem", IEEE Trans. AP-32(2):126-133
package mom

import (
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/mat"
)

// ComplexSkinDepth computes the complex skin depth d = 1/γ_g where
// γ_g = j·k₀·√εc is the propagation constant in the ground medium.
//
//	d = 1 / (j·k₀·√εc)
//
// k0 is the free-space wavenumber (rad/m), epsilonC is the complex
// relative permittivity of the ground.
func ComplexSkinDepth(k0 float64, epsilonC complex128) complex128 {
	// γ_g = j * k0 * sqrt(εc)
	gamma := complex(0, k0) * cmplx.Sqrt(epsilonC)
	if cmplx.Abs(gamma) < 1e-30 {
		return complex(1e10, 0)
	}
	return 1.0 / gamma
}

// GroundPropagationConst returns γ_g = j·k₀·√εc for the ground medium.
func GroundPropagationConst(k0 float64, epsilonC complex128) complex128 {
	return complex(0, k0) * cmplx.Sqrt(epsilonC)
}

// addComplexImageGroundBasis adds lossy-ground contributions to the
// Z-matrix using the complex-image method with the half-space Green's
// function approach.
//
// The PEC image kernel (TriangleKernelPerfectGround) provides the base
// image coupling.  On top of this, the complex-image method:
//
//  1. Computes the complex skin depth d = 1/(j·k·√εc).
//  2. Uses a modified grazing angle that includes the penetration depth
//     2·Re(d), giving a steeper effective angle.
//
// The modified grazing angle alone is the leading Bannister correction:
// it pushes the effective image deeper, making the Fresnel coefficient
// closer to the normal-incidence value.  No separate real-valued
// attenuation factor is applied, because the product α·Re(d) → 0.5
// as σ → ∞ (regardless of actual conductivity), which would prevent
// convergence to perfect ground.  The modified angle naturally
// converges to the geometric angle as σ → ∞ (Re(d) → 0).
func addComplexImageGroundBasis(Z *mat.CDense, bases []TriangleBasis, realSegs, imageSegs []Segment, k, omega, sigma, epsilonR float64) {
	epsilonC := ComplexPermittivity(epsilonR, sigma, omega)
	skinDepth := ComplexSkinDepth(k, epsilonC)

	penetrationDepth := real(skinDepth)

	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	n := len(bases)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			vecTerm, scaTerm := TriangleKernelPerfectGround(bases[i], bases[j], k)

			// Modified grazing angle: geometric + 2× penetration depth.
			obsPos := bases[i].NodePos
			srcPos := bases[j].NodePos

			dx := obsPos[0] - srcPos[0]
			dy := obsPos[1] - srcPos[1]
			horizDist := math.Sqrt(dx*dx + dy*dy)
			vertDistEff := math.Abs(obsPos[2]+srcPos[2]) + 2*math.Abs(penetrationDepth)

			psi := math.Atan2(vertDistEff, horizDist)
			if psi < 0.01 {
				psi = 0.01
			}

			rv := FresnelRV(psi, epsilonC)
			rh := FresnelRH(psi, epsilonC)

			vertFracI := basisVerticalFraction(bases[i])
			vertFracJ := basisVerticalFraction(bases[j])
			vertFrac := (vertFracI + vertFracJ) / 2.0
			rEff := complex(vertFrac, 0)*rv + complex(1-vertFrac, 0)*rh

			val := rEff * (vecPrefactor*vecTerm + scaPrefactor*scaTerm)
			old := Z.At(i, j)
			Z.Set(i, j, old+val)
		}
	}
}
