package mom

import (
	"math"
	"math/cmplx"
)

// DefaultReferenceImpedance is the reference impedance used for reflection
// coefficient and VSWR calculations when the request does not specify one.
// 50 Ω is the de-facto standard for amateur and commercial RF gear.
const DefaultReferenceImpedance = 50.0

// ReflectionCoefficient returns the complex voltage reflection coefficient
// Γ = (Z - Z₀) / (Z + Z₀) for an impedance Z relative to a real reference
// impedance Z₀.  Γ is the basis for Smith-chart plotting and VSWR.
//
// If Z₀ ≤ 0 the function silently substitutes DefaultReferenceImpedance,
// which mirrors how the rest of the solver treats unset values.
func ReflectionCoefficient(z ComplexImpedance, z0 float64) complex128 {
	if z0 <= 0 {
		z0 = DefaultReferenceImpedance
	}
	zc := complex(z.R, z.X)
	z0c := complex(z0, 0)
	return (zc - z0c) / (zc + z0c)
}

// VSWRFromGamma returns the voltage standing wave ratio derived from the
// magnitude of a reflection coefficient.  |Γ| ≥ 1 is treated as total
// reflection and clamped to a finite cap (999) so plotting code never
// has to handle infinity or NaN.
func VSWRFromGamma(gamma complex128) float64 {
	g := cmplx.Abs(gamma)
	if g >= 1.0 {
		return 999.0
	}
	swr := (1.0 + g) / (1.0 - g)
	// Numerically clamp results that would overflow plotting / display.
	// Practically anything above ~999 is "no useful match" anyway.
	if swr > 999.0 || math.IsNaN(swr) || math.IsInf(swr, 0) {
		return 999.0
	}
	return swr
}

// VSWRAt returns the VSWR of an impedance Z relative to the reference Z₀.
// Equivalent to VSWRFromGamma(ReflectionCoefficient(z, z0)).
func VSWRAt(z ComplexImpedance, z0 float64) float64 {
	return VSWRFromGamma(ReflectionCoefficient(z, z0))
}

// ReflectionRI is the rectangular form of a complex reflection coefficient
// for transport over the JSON API.  A separate type avoids the JSON
// encoding loss that would otherwise come from passing a Go complex128.
type ReflectionRI struct {
	Re float64 `json:"re"`
	Im float64 `json:"im"`
}

// ToReflectionRI converts a Go complex128 into the API-friendly
// rectangular representation.
func ToReflectionRI(g complex128) ReflectionRI {
	return ReflectionRI{Re: real(g), Im: imag(g)}
}
