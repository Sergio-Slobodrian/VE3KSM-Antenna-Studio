package mom

import (
	"fmt"
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/mat"
)

// LoadImpedance returns the complex impedance Z_load(ω) of a lumped R/L/C
// load at angular frequency omega (rad/s).  Topology selects series or
// parallel combination of the three components.  A zero-valued component
// is omitted from the sum (series) or admittance sum (parallel), so a
// single non-zero field models a pure resistor, inductor, or capacitor.
//
// Returns an error if the topology is unknown or — for the parallel case
// — if all three components are zero (degenerate, undefined impedance).
func LoadImpedance(load Load, omega float64) (complex128, error) {
	switch load.Topology {
	case LoadSeriesRLC, "":
		// Empty topology defaults to series so that {R: 50} alone is
		// still a useful "50-ohm terminator" without extra ceremony.
		var z complex128
		if load.R != 0 {
			z += complex(load.R, 0)
		}
		if load.L != 0 {
			z += complex(0, omega*load.L)
		}
		if load.C != 0 {
			z += 1.0 / complex(0, omega*load.C)
		}
		return z, nil

	case LoadParallelRLC:
		var y complex128
		nonZero := 0
		if load.R != 0 {
			y += complex(1.0/load.R, 0)
			nonZero++
		}
		if load.L != 0 {
			y += 1.0 / complex(0, omega*load.L)
			nonZero++
		}
		if load.C != 0 {
			y += complex(0, omega*load.C)
			nonZero++
		}
		if nonZero == 0 {
			return 0, fmt.Errorf("parallel_rlc load has no components (R=L=C=0)")
		}
		if cmplx.Abs(y) < 1e-30 {
			return 0, fmt.Errorf("parallel_rlc load admittance vanishes at omega=%g", omega)
		}
		return 1.0 / y, nil

	default:
		return 0, fmt.Errorf("unknown load topology %q", load.Topology)
	}
}

// applyLoads injects each load's complex impedance onto the Z-matrix
// diagonal at the basis function nearest its (wire, segment) position,
// which is the same mapping used by the voltage source.
//
// Mathematically: with delta-gap-style lumped loads and the MoM matrix
// equation Z·I = V, adding Z_load to the diagonal element Z[m,m] is
// equivalent to writing Kirchhoff's voltage law around basis m as
// V_m = (Z_radiation,m + Z_load,m)·I_m + ∑_{n≠m} Z[m,n]·I_n.
//
// This is the standard NEC LD-card treatment and is exact for lumped
// elements that are small relative to the segment length.
func applyLoads(Z *mat.CDense, loads []Load, omega float64,
	wires []Wire, wireSegOffsets, wireSegCounts, wireBasisOffsets []int,
	lossPerBasis []float64) error {

	for li, ld := range loads {
		basisIdx, err := resolveLoadBasis(ld, wires, wireSegCounts, wireBasisOffsets)
		if err != nil {
			return fmt.Errorf("load %d: %w", li, err)
		}
		zLoad, err := LoadImpedance(ld, omega)
		if err != nil {
			return fmt.Errorf("load %d: %w", li, err)
		}
		// Skip the no-op case where the load contributes nothing (all
		// fields zero in series mode) — common for partially-filled
		// requests during UI editing.
		if zLoad == 0 {
			continue
		}
		// Sanity guard against catastrophic numerical values that would
		// destroy the LU factorisation.
		if math.IsNaN(real(zLoad)) || math.IsNaN(imag(zLoad)) ||
			math.IsInf(real(zLoad), 0) || math.IsInf(imag(zLoad), 0) {
			return fmt.Errorf("load %d: non-finite impedance %v", li, zLoad)
		}

		cur := Z.At(basisIdx, basisIdx)
		Z.Set(basisIdx, basisIdx, cur+zLoad)
		if lossPerBasis != nil && basisIdx < len(lossPerBasis) {
			lossPerBasis[basisIdx] += real(zLoad)
		}
	}
	_ = wireSegOffsets // kept in signature for symmetry with resolveFeedBasis
	return nil
}

// resolveLoadBasis maps a (wire, segment) load specification to the global
// basis index using the same nearest-interior-node rule as the source.
// This keeps load and source placement intuitive and consistent: asking
// for "segment 5" puts both at the same junction.
func resolveLoadBasis(ld Load, wires []Wire, wireSegCounts, wireBasisOffsets []int) (int, error) {
	if ld.WireIndex < 0 || ld.WireIndex >= len(wires) {
		return 0, fmt.Errorf("wire_index %d out of range [0, %d)", ld.WireIndex, len(wires))
	}
	nSeg := wireSegCounts[ld.WireIndex]
	if nSeg < 2 {
		return 0, fmt.Errorf("wire %d has %d segments; need ≥2 for a load", ld.WireIndex, nSeg)
	}
	segIdx := ld.SegmentIndex
	if segIdx < 0 || segIdx >= nSeg {
		return 0, fmt.Errorf("segment_index %d out of range [0, %d)", segIdx, nSeg)
	}
	nodeIdx := segIdx + 1
	if nodeIdx < 1 {
		nodeIdx = 1
	}
	if nodeIdx > nSeg-1 {
		nodeIdx = nSeg - 1
	}
	return wireBasisOffsets[ld.WireIndex] + (nodeIdx - 1), nil
}
