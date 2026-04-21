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

// Package mom provides the convergence reporter: re-run at 2x segmentation
// and report relative change in driving-point impedance, SWR, and gain.
// A small delta indicates the user's mesh is adequately resolved.
package mom

import (
	"fmt"
	"math"
)

// ConvergenceResult holds the 1x and 2x simulation outcomes and the
// computed deltas that tell the user whether their segmentation has converged.
type ConvergenceResult struct {
	// Impedance at original segmentation
	ImpedanceR1 float64 `json:"impedance_r_1x"`
	ImpedanceX1 float64 `json:"impedance_x_1x"`
	SWR1        float64 `json:"swr_1x"`
	GainDBi1    float64 `json:"gain_dbi_1x"`
	Segments1   int     `json:"total_segments_1x"`

	// Impedance at doubled segmentation
	ImpedanceR2 float64 `json:"impedance_r_2x"`
	ImpedanceX2 float64 `json:"impedance_x_2x"`
	SWR2        float64 `json:"swr_2x"`
	GainDBi2    float64 `json:"gain_dbi_2x"`
	Segments2   int     `json:"total_segments_2x"`

	// Relative changes (percent)
	DeltaRPct    float64 `json:"delta_r_pct"`
	DeltaXPct    float64 `json:"delta_x_pct"`
	DeltaZMagPct float64 `json:"delta_z_mag_pct"`
	DeltaSWRPct  float64 `json:"delta_swr_pct"`
	DeltaGainDb  float64 `json:"delta_gain_db"`

	// Overall verdict
	Converged bool   `json:"converged"`
	Verdict   string `json:"verdict"`
}

// solverSegCount mirrors the odd-count adjustment in solver.go so that
// convergence can report the segments actually used, not the raw input.
func solverSegCount(requested int) int {
	n := requested
	if n < 3 {
		n = 3
	}
	if n%2 == 0 {
		n++
	}
	return n
}

// remapSegIndex scales a segment index from an old mesh to a new mesh,
// preserving the physical position along the wire.
//
//	newIndex ≈ (oldIndex + 0.5) * newSeg / oldSeg
//
// This is exact for true doubling (newSeg = 2*oldSeg) and correct for any
// other ratio, including when a cap prevents a full 2× increase.
func remapSegIndex(oldIndex, oldSeg, newSeg int) int {
	if oldSeg <= 0 {
		return 0
	}
	idx := int((float64(oldIndex)+0.5)*float64(newSeg)/float64(oldSeg))
	if idx >= newSeg {
		idx = newSeg - 1
	}
	if idx < 0 {
		idx = 0
	}
	return idx
}

// RunConvergenceCheck runs the simulation at the user's segmentation (1x)
// and then at 2x segmentation, reporting the relative impedance change.
// The threshold for "converged" is < 2% change in |Z| magnitude.
func RunConvergenceCheck(input SimulationInput) (*ConvergenceResult, error) {
	if len(input.Wires) == 0 {
		return nil, fmt.Errorf("no wires provided")
	}

	// --- 1x run (original segments) ---
	res1, err := Simulate(input)
	if err != nil {
		return nil, fmt.Errorf("1x simulation failed: %w", err)
	}

	// --- Build 2x input ---
	// maxSegPerWire is generous enough to let the 2x test run at any
	// practical mesh density.  The old cap of 200 caused the 2x run to
	// have fewer segments than the 1x run when the user had > 100
	// segments/wire, producing misleading "worse convergence" results.
	const maxSegPerWire = 2000

	input2 := input
	input2.Wires = make([]Wire, len(input.Wires))
	copy(input2.Wires, input.Wires)

	totalSeg1 := 0
	totalSeg2 := 0
	invalidCap := false // true when cap prevents a proper 2x comparison

	for i := range input2.Wires {
		actual1 := solverSegCount(input2.Wires[i].Segments) // what solver actually uses
		totalSeg1 += actual1

		doubled := actual1 * 2
		if doubled > maxSegPerWire {
			doubled = maxSegPerWire
		}
		// Ensure odd count (solver requirement) without going below actual1.
		if doubled%2 == 0 {
			doubled++
		}
		if doubled <= actual1 {
			// Cap prevents any increase — comparison would be invalid.
			invalidCap = true
		}
		input2.Wires[i].Segments = doubled
		totalSeg2 += doubled
	}

	if invalidCap {
		// Cannot produce a meaningful 2x comparison at this resolution.
		r1, x1 := res1.Impedance.R, res1.Impedance.X
		return &ConvergenceResult{
			ImpedanceR1: r1,
			ImpedanceX1: x1,
			SWR1:        res1.SWR,
			GainDBi1:    res1.GainDBi,
			Segments1:   totalSeg1,
			Segments2:   totalSeg1, // unchanged — no 2x run
			Converged:   true,      // assume converged at high resolution
			Verdict:     "Maximum test resolution reached — segment count is already very high. Results are assumed well converged.",
		}, nil
	}

	// Remap source segment index proportionally to the new mesh.
	// Using (oldIdx + 0.5) * newSeg / oldSeg preserves the physical
	// location on the wire regardless of the actual doubling ratio.
	srcWire := input.Source.WireIndex
	if srcWire >= 0 && srcWire < len(input.Wires) {
		oldSeg := solverSegCount(input.Wires[srcWire].Segments)
		newSeg := input2.Wires[srcWire].Segments
		input2.Source.SegmentIndex = remapSegIndex(input.Source.SegmentIndex, oldSeg, newSeg)
	}

	// Remap load segment indices
	if len(input2.Loads) > 0 {
		input2.Loads = make([]Load, len(input.Loads))
		copy(input2.Loads, input.Loads)
		for i := range input2.Loads {
			wi := input2.Loads[i].WireIndex
			if wi >= 0 && wi < len(input2.Wires) {
				oldSeg := solverSegCount(input.Wires[wi].Segments)
				newSeg := input2.Wires[wi].Segments
				input2.Loads[i].SegmentIndex = remapSegIndex(input.Loads[i].SegmentIndex, oldSeg, newSeg)
			}
		}
	}

	// Remap transmission line segment indices
	if len(input2.TransmissionLines) > 0 {
		input2.TransmissionLines = make([]TransmissionLine, len(input.TransmissionLines))
		copy(input2.TransmissionLines, input.TransmissionLines)
		for i := range input2.TransmissionLines {
			tl := &input2.TransmissionLines[i]
			orig := &input.TransmissionLines[i]
			if tl.A.WireIndex >= 0 && tl.A.WireIndex < len(input2.Wires) {
				oldSeg := solverSegCount(input.Wires[tl.A.WireIndex].Segments)
				newSeg := input2.Wires[tl.A.WireIndex].Segments
				tl.A.SegmentIndex = remapSegIndex(orig.A.SegmentIndex, oldSeg, newSeg)
			}
			if tl.B.WireIndex >= 0 && tl.B.WireIndex < len(input2.Wires) {
				oldSeg := solverSegCount(input.Wires[tl.B.WireIndex].Segments)
				newSeg := input2.Wires[tl.B.WireIndex].Segments
				tl.B.SegmentIndex = remapSegIndex(orig.B.SegmentIndex, oldSeg, newSeg)
			}
		}
	}

	res2, err := Simulate(input2)
	if err != nil {
		return nil, fmt.Errorf("2x simulation failed: %w", err)
	}

	// --- Compute deltas ---
	r1, x1 := res1.Impedance.R, res1.Impedance.X
	r2, x2 := res2.Impedance.R, res2.Impedance.X

	zmag1 := math.Sqrt(r1*r1 + x1*x1)
	zmag2 := math.Sqrt(r2*r2 + x2*x2)

	deltaR := relPct(r1, r2)
	deltaX := relPctAbs(x1, x2, zmag1) // use |Z| as reference for X to avoid div-by-zero when X≈0
	deltaZMag := relPct(zmag1, zmag2)
	deltaSWR := relPct(res1.SWR, res2.SWR)
	deltaGain := res2.GainDBi - res1.GainDBi

	converged := math.Abs(deltaZMag) < 2.0

	verdict := ""
	switch {
	case math.Abs(deltaZMag) < 1.0:
		verdict = "Excellent convergence — impedance change < 1%. Your segmentation is well resolved."
	case math.Abs(deltaZMag) < 2.0:
		verdict = "Good convergence — impedance change < 2%. Results are reliable for most purposes."
	case math.Abs(deltaZMag) < 5.0:
		verdict = "Marginal convergence — impedance change 2–5%. Consider increasing segments for higher accuracy."
	default:
		verdict = "Poor convergence — impedance change > 5%. Increase segment count significantly for reliable results."
	}

	return &ConvergenceResult{
		ImpedanceR1: r1,
		ImpedanceX1: x1,
		SWR1:        res1.SWR,
		GainDBi1:    res1.GainDBi,
		Segments1:   totalSeg1,

		ImpedanceR2: r2,
		ImpedanceX2: x2,
		SWR2:        res2.SWR,
		GainDBi2:    res2.GainDBi,
		Segments2:   totalSeg2,

		DeltaRPct:    deltaR,
		DeltaXPct:    deltaX,
		DeltaZMagPct: deltaZMag,
		DeltaSWRPct:  deltaSWR,
		DeltaGainDb:  deltaGain,

		Converged: converged,
		Verdict:   verdict,
	}, nil
}

// relPct returns the relative percentage change from a to b: 100*(b-a)/|a|.
// Returns 0 if a is zero (no meaningful reference).
func relPct(a, b float64) float64 {
	if math.Abs(a) < 1e-30 {
		return 0
	}
	return 100.0 * (b - a) / math.Abs(a)
}

// relPctAbs returns the relative percentage change using a custom reference magnitude.
// Useful for reactance where the value can be near zero while |Z| is not.
func relPctAbs(a, b, ref float64) float64 {
	if ref < 1e-30 {
		return 0
	}
	return 100.0 * (b - a) / ref
}
