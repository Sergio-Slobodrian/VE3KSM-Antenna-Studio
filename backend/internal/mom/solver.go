package mom

import (
	"fmt"
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/mat"
)

// Simulate runs the full MoM simulation pipeline for a single frequency:
//  1. Subdivide all wires into segments
//  2. Determine the global feed segment index
//  3. Compute k = 2*pi*f/c, omega = 2*pi*f
//  4. Build the impedance matrix Z
//  5. Build the voltage excitation vector V
//  6. Solve Z*I = V via LU decomposition
//  7. Compute feed impedance Z_in = V_source / I_feed
//  8. Compute SWR relative to 50 ohms
//  9. Compute far-field radiation pattern and gain
//  10. Extract segment current magnitudes and phases
func Simulate(input SimulationInput) (*SolverResult, error) {
	if len(input.Wires) == 0 {
		return nil, fmt.Errorf("no wires provided")
	}
	if input.Frequency <= 0 {
		return nil, fmt.Errorf("frequency must be positive")
	}

	// Step 1: Subdivide all wires into segments
	var allSegments []Segment
	wireSegmentOffsets := make([]int, len(input.Wires))
	for wi, w := range input.Wires {
		wireSegmentOffsets[wi] = len(allSegments)
		numSeg := w.Segments
		if numSeg < 1 {
			numSeg = 11
		}
		segs := SubdivideWire(wi, w.X1, w.Y1, w.Z1, w.X2, w.Y2, w.Z2, w.Radius, numSeg)
		for j := range segs {
			segs[j].Index = len(allSegments) + j
		}
		allSegments = append(allSegments, segs...)
	}

	n := len(allSegments)
	if n == 0 {
		return nil, fmt.Errorf("no segments generated")
	}

	// Step 2: Determine global feed segment index
	feedGlobal, err := resolveSourceSegment(input.Source, input.Wires)
	if err != nil {
		return nil, err
	}

	// Resolve voltage
	voltage := input.Source.Voltage
	if cmplx.Abs(voltage) == 0 {
		voltage = 1 + 0i
	}

	// Step 3: Frequency parameters
	freq := input.Frequency
	omega := 2.0 * math.Pi * freq
	k := omega / C0

	// Step 4: Build impedance matrix
	Z := BuildZMatrix(allSegments, k, omega)

	// Step 4b: Apply ground plane contributions
	if input.Ground.Type == "perfect" {
		imageSegs := ApplyPerfectGround(allSegments)
		AddGroundContributions(Z, allSegments, imageSegs, k, omega)
	}

	// Step 5: Build voltage vector (all zeros except at feed)
	V := make([]complex128, n)
	V[feedGlobal] = voltage

	// Step 6: Solve Z*I = V using LU decomposition
	// Convert to 2N x 2N real system since gonum lacks complex LU
	I, err := solveComplexLU(Z, V, n)
	if err != nil {
		return nil, fmt.Errorf("solver failed: %w", err)
	}

	// Step 7: Compute feed impedance
	feedCurrent := I[feedGlobal]
	var impedance ComplexImpedance
	if cmplx.Abs(feedCurrent) > 1e-30 {
		zIn := voltage / feedCurrent
		impedance = ComplexImpedance{R: real(zIn), X: imag(zIn)}
	}

	// Step 8: Compute SWR (50-ohm reference)
	zComplex := complex(impedance.R, impedance.X)
	gamma := (zComplex - 50) / (zComplex + 50)
	gammaAbs := cmplx.Abs(gamma)
	swr := 1.0
	if gammaAbs < 1.0 {
		swr = (1.0 + gammaAbs) / (1.0 - gammaAbs)
	} else {
		swr = 999.0
	}

	// Step 9: Far-field pattern and gain
	pattern, gainDBi := ComputeFarField(allSegments, I, k)

	// Step 10: Extract currents
	currents := make([]CurrentEntry, n)
	for i, c := range I {
		currents[i] = CurrentEntry{
			SegmentIndex: i,
			Magnitude:    cmplx.Abs(c),
			PhaseDeg:     cmplx.Phase(c) * 180.0 / math.Pi,
		}
	}

	return &SolverResult{
		Currents:  currents,
		Impedance: impedance,
		SWR:       swr,
		GainDBi:   gainDBi,
		Pattern:   pattern,
	}, nil
}

// Sweep runs the MoM solver across a frequency range.
// freqStartHz and freqEndHz are in Hz. steps is the number of frequency points.
func Sweep(input SimulationInput, freqStartHz, freqEndHz float64, steps int) (*SweepResult, error) {
	if steps < 2 {
		return nil, fmt.Errorf("frequency sweep requires at least 2 steps")
	}

	result := &SweepResult{
		Frequencies: make([]float64, steps),
		SWR:         make([]float64, steps),
		Impedance:   make([]ComplexImpedance, steps),
	}

	stepSize := (freqEndHz - freqStartHz) / float64(steps-1)

	for i := 0; i < steps; i++ {
		freq := freqStartHz + float64(i)*stepSize
		result.Frequencies[i] = freq / 1e6 // store as MHz

		stepInput := input
		stepInput.Frequency = freq

		res, err := Simulate(stepInput)
		if err != nil {
			return nil, fmt.Errorf("sweep failed at %.3f MHz: %w", freq/1e6, err)
		}

		result.SWR[i] = res.SWR
		result.Impedance[i] = res.Impedance
	}

	return result, nil
}

// resolveSourceSegment converts wire-relative source indices to a global segment index.
func resolveSourceSegment(src Source, wires []Wire) (int, error) {
	if src.WireIndex < 0 || src.WireIndex >= len(wires) {
		return 0, fmt.Errorf("source wire_index %d out of range [0, %d)", src.WireIndex, len(wires))
	}
	w := wires[src.WireIndex]
	numSeg := w.Segments
	if numSeg < 1 {
		numSeg = 11
	}
	if src.SegmentIndex < 0 || src.SegmentIndex >= numSeg {
		return 0, fmt.Errorf("source segment_index %d out of range [0, %d)", src.SegmentIndex, numSeg)
	}

	global := 0
	for i := 0; i < src.WireIndex; i++ {
		ns := wires[i].Segments
		if ns < 1 {
			ns = 11
		}
		global += ns
	}
	global += src.SegmentIndex
	return global, nil
}

// solveComplexLU solves the complex linear system Z*I = V by converting to
// a 2N x 2N real system and using gonum's LU decomposition.
//
// The equivalent real system is:
//
//	[Re(Z)  -Im(Z)] [Re(I)]   [Re(V)]
//	[Im(Z)   Re(Z)] [Im(I)] = [Im(V)]
func solveComplexLU(Z *mat.CDense, V []complex128, n int) ([]complex128, error) {
	A := mat.NewDense(2*n, 2*n, nil)
	b := mat.NewVecDense(2*n, nil)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			z := Z.At(i, j)
			re := real(z)
			im := imag(z)

			A.Set(i, j, re)       // top-left: Re(Z)
			A.Set(i, n+j, -im)    // top-right: -Im(Z)
			A.Set(n+i, j, im)     // bottom-left: Im(Z)
			A.Set(n+i, n+j, re)   // bottom-right: Re(Z)
		}
		b.SetVec(i, real(V[i]))
		b.SetVec(n+i, imag(V[i]))
	}

	var lu mat.LU
	lu.Factorize(A)

	x := mat.NewVecDense(2*n, nil)
	if err := lu.SolveVecTo(x, false, b); err != nil {
		return nil, fmt.Errorf("LU solve failed: %w", err)
	}

	// Reconstruct complex solution
	I := make([]complex128, n)
	for i := 0; i < n; i++ {
		I[i] = complex(x.AtVec(i), x.AtVec(n+i))
	}
	return I, nil
}
