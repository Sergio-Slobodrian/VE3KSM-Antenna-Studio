package mom

import (
	"fmt"
	"math"
)

// MaterialName identifies a conductor by short tag.  Empty / unset is
// treated as MaterialPEC (perfect electric conductor), which preserves
// the loss-free behaviour that pre-existed Item 4.
type MaterialName string

const (
	MaterialPEC      MaterialName = ""              // perfect conductor (loss-free, default)
	MaterialCopper   MaterialName = "copper"        // σ = 5.80e7 S/m, μ_r = 1
	MaterialAluminum MaterialName = "aluminum"      // σ = 3.50e7 S/m, μ_r = 1
	MaterialBrass    MaterialName = "brass"         // σ = 1.59e7 S/m, μ_r = 1
	MaterialSteel    MaterialName = "steel"         // σ = 1.45e6 S/m, μ_r ≈ 1000 (plain steel)
	MaterialStainless MaterialName = "stainless"    // σ = 1.10e6 S/m, μ_r ≈ 1
	MaterialSilver   MaterialName = "silver"        // σ = 6.30e7 S/m, μ_r = 1
	MaterialGold     MaterialName = "gold"          // σ = 4.10e7 S/m, μ_r = 1
)

// Material describes the bulk properties of a conductor needed for
// surface-impedance / skin-effect loss calculations on round wires.
//
//   - Sigma is the bulk conductivity in S/m.
//   - MuR is the relative permeability (1 for non-ferrous metals,
//     ~1000 for plain carbon steel).
//
// The thin-wire MoM solver applies an incremental impedance per
// segment of Z_loss(ω) = R_s · ℓ / (2πa), where R_s = √(πfμ/σ) is the
// surface resistance and ℓ/(2πa) converts from "ohms per square" to
// "ohms per segment" assuming current is uniformly distributed around
// the wire's circumference (a thin-wire-approximation consistent
// idealisation; matches NEC-2's "wire loading per unit length").
type Material struct {
	Sigma float64 // bulk conductivity (S/m)
	MuR   float64 // relative permeability (dimensionless)
}

// MaterialLibrary is the lookup table used by the API and the solver.
// Custom materials can be added by callers or registered at startup.
var MaterialLibrary = map[MaterialName]Material{
	MaterialPEC:       {Sigma: math.Inf(1), MuR: 1},
	MaterialCopper:    {Sigma: 5.80e7, MuR: 1},
	MaterialAluminum:  {Sigma: 3.50e7, MuR: 1},
	MaterialBrass:     {Sigma: 1.59e7, MuR: 1},
	MaterialSteel:     {Sigma: 1.45e6, MuR: 1000},
	MaterialStainless: {Sigma: 1.10e6, MuR: 1},
	MaterialSilver:    {Sigma: 6.30e7, MuR: 1},
	MaterialGold:      {Sigma: 4.10e7, MuR: 1},
}

// LookupMaterial resolves a name to its bulk properties.  Unknown
// names return (Material{}, false); the empty string maps to PEC.
func LookupMaterial(name MaterialName) (Material, bool) {
	if name == "" {
		return MaterialLibrary[MaterialPEC], true
	}
	m, ok := MaterialLibrary[name]
	return m, ok
}

// SkinDepth returns the classical skin depth δ = 1 / √(πfμσ) in metres
// for a material at frequency f (Hz).  Returns +Inf for PEC (σ = ∞).
func SkinDepth(mat Material, freqHz float64) float64 {
	if math.IsInf(mat.Sigma, 1) {
		return math.Inf(1)
	}
	mu := Mu0 * mat.MuR
	return 1.0 / math.Sqrt(math.Pi*freqHz*mu*mat.Sigma)
}

// SurfaceResistance returns the surface resistance R_s in Ω/□ (ohms
// per square) at frequency f.  R_s = √(πfμ/σ).  Returns 0 for PEC.
func SurfaceResistance(mat Material, freqHz float64) float64 {
	if math.IsInf(mat.Sigma, 1) {
		return 0
	}
	mu := Mu0 * mat.MuR
	return math.Sqrt(math.Pi * freqHz * mu / mat.Sigma)
}

// SegmentLossOhms returns the additional series resistance contributed
// by the skin-effect on a single round-wire segment.
//
//	Z_loss = R_s · ℓ / (2π a)
//
// ℓ is the segment length, a is the wire radius.  This treats the
// segment as a uniform conductor of cross-sectional perimeter 2πa with
// surface resistance R_s, which is the standard NEC-style loading
// term.  For PEC this returns 0.
func SegmentLossOhms(mat Material, freqHz, segLength, radius float64) float64 {
	rs := SurfaceResistance(mat, freqHz)
	if rs == 0 || radius <= 0 || segLength <= 0 {
		return 0
	}
	return rs * segLength / (2.0 * math.Pi * radius)
}

// applyMaterialLoss walks every wire / basis pair and adds the skin-
// effect resistance onto the Z-matrix diagonal at every basis function.
// It is the bulk-conductor analogue of applyLoads: a per-segment
// resistance distributed over the basis support.
//
// The triangle basis spans two adjacent segments; we charge the basis
// with half of each adjacent segment's loss term.  This matches NEC's
// LD card "wire conductivity" treatment (NEC-2 manual §III).
//
// Returns an error if a wire references an unknown material name.
func applyMaterialLoss(zmat zMatSetter, wires []Wire, segments []Segment,
	wireSegOffsets, wireSegCounts, wireBasisOffsets []int,
	freqHz float64, lossPerBasis []float64) error {

	for wi, w := range wires {
		if w.Material == "" || w.Material == MaterialPEC {
			continue
		}
		mat, ok := LookupMaterial(w.Material)
		if !ok {
			return fmt.Errorf("wire %d: unknown material %q", wi, w.Material)
		}
		segOff := wireSegOffsets[wi]
		nSeg := wireSegCounts[wi]
		basisOff := wireBasisOffsets[wi]
		// nSeg-1 interior basis nodes; basis k spans segments k and k+1.
		for k := 0; k < nSeg-1; k++ {
			seg1 := segments[segOff+k]
			seg2 := segments[segOff+k+1]
			len1 := 2 * seg1.HalfLength
			len2 := 2 * seg2.HalfLength
			// Distribute half of each adjacent segment's loss onto
			// this basis: matches NEC-2's LD-card treatment.
			lossR := 0.5*SegmentLossOhms(mat, freqHz, len1, w.Radius) +
				0.5*SegmentLossOhms(mat, freqHz, len2, w.Radius)
			if lossR == 0 {
				continue
			}
			b := basisOff + k
			zmat.Add(b, b, complex(lossR, 0))
			if lossPerBasis != nil && b < len(lossPerBasis) {
				lossPerBasis[b] += lossR
			}
		}
	}
	return nil
}

// zMatSetter is a tiny shim over gonum's CDense to keep applyMaterialLoss
// focused on physics, not container API.  The solver passes a closure
// or wrapper that performs Z[i,j] += v.
type zMatSetter interface {
	Add(i, j int, v complex128)
}
