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

// chooseAnchors picks a sensible anchor count for a sweep of nSteps
// points.  Heuristic: ceil(sqrt(nSteps * 2)), capped at nSteps and
// floored at 8.  For 200 steps -> 20 anchors; for 500 -> ~32.
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
		anchors = chooseAnchors(steps)
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
			"sweep interpolated from %d full MoM solves at uniformly-spaced anchors (%.3f MHz step) using natural cubic spline.  Set mode=exact to force a full solve at every point",
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
