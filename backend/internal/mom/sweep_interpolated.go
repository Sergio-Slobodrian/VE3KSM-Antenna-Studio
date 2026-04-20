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
	"fmt"
	"math"
)

// SweepMode picks how a frequency sweep is computed.
type SweepMode string

const (
	// SweepModeExact runs a full MoM solve at every frequency point.
	// Slowest but most accurate; necessary near sharp resonances or
	// for very small step sizes.
	SweepModeExact SweepMode = "exact"

	// SweepModeInterpolated runs a full solve only at a small number of
	// "anchor" frequencies and cubic-spline interpolates R(f), X(f)
	// for the remaining points.  Typical 10-50x faster than exact for
	// long sweeps; accuracy is excellent away from resonances and good
	// near them when the anchor density is sufficient.
	SweepModeInterpolated SweepMode = "interpolated"

	// SweepModeAuto picks Interpolated when the requested step count
	// exceeds InterpolationThreshold and Exact otherwise.  This is the
	// default when no mode is supplied on a request.
	SweepModeAuto SweepMode = ""
)

// InterpolationThreshold is the step count above which SweepModeAuto
// switches from exact to interpolated.
const InterpolationThreshold = 32

// SweepOptions tunes the behaviour of Sweep.
type SweepOptions struct {
	Mode    SweepMode
	Anchors int // when Mode is interpolated; 0 = pick automatically
}

// chooseAnchors picks a sensible anchor count for a sweep.
// The basic heuristic is ceil(sqrt(nSteps * 2)), but when the
// frequency ratio is large (wide sweep) the impedance of even a
// simple wire can cycle through many resonances.  Each half-wave
// resonance needs ≈8 anchors to capture the R+X peak, so we also
// compute a physics-aware floor from the electrical length of the
// longest wire at the highest frequency.  The result is the larger
// of the two estimates, capped at nSteps and floored at 8.
func chooseAnchors(nSteps int) int {
	a := int(math.Ceil(math.Sqrt(float64(nSteps) * 2)))
	if a < 8 {
		a = 8
	}
	if a > nSteps {
		a = nSteps
	}
	return a
}

// chooseAnchorsPhysics is like chooseAnchors but also considers the
// antenna's electrical extent to ensure enough anchors per resonance
// cycle.  freqStartHz..freqEndHz is the sweep range and maxWireLen
// is the length of the longest wire in metres.
func chooseAnchorsPhysics(nSteps int, freqStartHz, freqEndHz, maxWireLen float64) int {
	// Base heuristic
	a := chooseAnchors(nSteps)

	// Physics-aware: estimate the number of half-wave resonances in
	// the sweep band.  Each resonance swings R from ~50 Ω to ~kΩ and
	// back, so we want ≥8 anchors per cycle to keep interpolation
	// error small.
	if maxWireLen > 0 && freqStartHz > 0 {
		c := 2.998e8
		// Resonances at n·c/(2L).  Count how many fall in [fStart, fEnd].
		resSpacingHz := c / (2.0 * maxWireLen)
		nResonances := (freqEndHz - freqStartHz) / resSpacingHz
		if nResonances < 1 {
			nResonances = 1
		}
		physAnchors := int(math.Ceil(nResonances * 8))
		if physAnchors > a {
			a = physAnchors
		}
	}
	if a > nSteps {
		a = nSteps
	}
	return a
}

// SweepWithOptions is the configurable variant of Sweep.  Sweep itself
// keeps its old signature and calls this with SweepModeAuto / Anchors=0.
func SweepWithOptions(input SimulationInput, freqStartHz, freqEndHz float64, steps int, opts SweepOptions) (*SweepResult, error) {
	if steps < 2 {
		return nil, fmt.Errorf("frequency sweep requires at least 2 steps")
	}

	mode := opts.Mode
	if mode == SweepModeAuto {
		if steps > InterpolationThreshold {
			mode = SweepModeInterpolated
		} else {
			mode = SweepModeExact
		}
	}

	if mode == SweepModeExact {
		return sweepExact(input, freqStartHz, freqEndHz, steps)
	}
	anchors := opts.Anchors
	if anchors <= 0 {
		// Find the longest wire so the anchor heuristic can account
		// for the number of resonance cycles in the sweep band.
		var maxLen float64
		for _, w := range input.Wires {
			dx := w.X2 - w.X1
			dy := w.Y2 - w.Y1
			dz := w.Z2 - w.Z1
			l := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if l > maxLen {
				maxLen = l
			}
		}
		anchors = chooseAnchorsPhysics(steps, freqStartHz, freqEndHz, maxLen)
	}
	if anchors > steps {
		anchors = steps
	}
	return sweepInterpolated(input, freqStartHz, freqEndHz, steps, anchors)
}

// sweepInterpolated runs full Simulate() at nAnchors uniformly-spaced
// frequencies and cubic-spline interpolates R(f), X(f) at the rest.
// Reflection coefficients and SWR are then derived from the
// interpolated Z.  Falls back to exact mode (returns error) if the
// anchor count is >= the step count.
func sweepInterpolated(input SimulationInput, freqStartHz, freqEndHz float64, steps, nAnchors int) (*SweepResult, error) {
	z0 := input.ReferenceImpedance
	if z0 <= 0 {
		z0 = DefaultReferenceImpedance
	}
	result := &SweepResult{
		Frequencies:        make([]float64, steps),
		SWR:                make([]float64, steps),
		Impedance:          make([]ComplexImpedance, steps),
		Reflections:        make([]complex128, steps),
		ReferenceImpedance: z0,
	}

	// Anchor frequencies (linear spacing).
	anchorFreqs := make([]float64, nAnchors)
	anchorR := make([]float64, nAnchors)
	anchorX := make([]float64, nAnchors)
	for i := 0; i < nAnchors; i++ {
		anchorFreqs[i] = freqStartHz + float64(i)*(freqEndHz-freqStartHz)/float64(nAnchors-1)
		stepInput := input
		stepInput.Frequency = anchorFreqs[i]
		res, err := Simulate(stepInput)
		if err != nil {
			return nil, fmt.Errorf("interpolated sweep failed at anchor %.3f MHz: %w", anchorFreqs[i]/1e6, err)
		}
		anchorR[i] = res.Impedance.R
		anchorX[i] = res.Impedance.X
	}

	splineR, err := NewSpline(anchorFreqs, anchorR)
	if err != nil {
		return nil, fmt.Errorf("interpolation spline (R): %w", err)
	}
	splineX, err := NewSpline(anchorFreqs, anchorX)
	if err != nil {
		return nil, fmt.Errorf("interpolation spline (X): %w", err)
	}

	stepHz := (freqEndHz - freqStartHz) / float64(steps-1)
	for i := 0; i < steps; i++ {
		f := freqStartHz + float64(i)*stepHz
		R := splineR.Eval(f)
		X := splineX.Eval(f)
		z := ComplexImpedance{R: R, X: X}
		gamma := ReflectionCoefficient(z, z0)
		result.Frequencies[i] = f / 1e6
		result.Impedance[i] = z
		result.Reflections[i] = gamma
		result.SWR[i] = VSWRFromGamma(gamma)
	}

	// Sweep-range advisory + start/end validation as in sweepExact.
	seen := map[string]bool{}
	for _, w := range ValidateGeometry(input.Wires, freqStartHz) {
		if !seen[w.Code] {
			result.Warnings = append(result.Warnings, w)
			seen[w.Code] = true
		}
	}
	for _, w := range ValidateGeometry(input.Wires, freqEndHz) {
		if !seen[w.Code] {
			result.Warnings = append(result.Warnings, w)
			seen[w.Code] = true
		}
	}
	addSweepRangeAdvisory(&result.Warnings, freqStartHz, freqEndHz)

	// One additional info note so users know the sweep was interpolated.
	result.Warnings = append(result.Warnings, Warning{
		Code:     "sweep_interpolated",
		Severity: SeverityInfo,
		Message: fmt.Sprintf(
			"sweep interpolated from %d full MoM solves at uniformly-spaced anchors (%.3f MHz step) using PCHIP monotone cubic Hermite.  Set mode=exact to force a full solve at every point",
			nAnchors, (freqEndHz-freqStartHz)/float64(nAnchors-1)/1e6),
	})

	return result, nil
}

// addSweepRangeAdvisory appends the sweep_range_unsatisfiable note to
// the warnings slice when the frequency span exceeds 10:1.  Extracted
// from the original sweepExact function so both paths emit the same
// advisory.
func addSweepRangeAdvisory(warnings *[]Warning, freqStartHz, freqEndHz float64) {
	if freqStartHz <= 0 {
		return
	}
	ratio := freqEndHz / freqStartHz
	if ratio <= 10 {
		return
	}
	sev := SeverityInfo
	msg := fmt.Sprintf(
		"sweep ratio %.1f:1 (%.3f-%.3f MHz) is wider than any fixed segment count can fully satisfy; expect impedance drift near each band edge.  Either narrow the sweep or split it into bands and pick segments per band",
		ratio, freqStartHz/1e6, freqEndHz/1e6)
	if ratio > 20 {
		sev = SeverityWarn
		msg = fmt.Sprintf(
			"sweep ratio %.1f:1 (%.3f-%.3f MHz) exceeds 20:1; no fixed segment count satisfies both NEC accuracy bounds.  Results near each extreme will be approximate; split the sweep into bands for trustworthy numbers",
			ratio, freqStartHz/1e6, freqEndHz/1e6)
	}
	*warnings = append(*warnings, Warning{
		Code:     "sweep_range_unsatisfiable",
		Severity: sev,
		Message:  msg,
	})
}
