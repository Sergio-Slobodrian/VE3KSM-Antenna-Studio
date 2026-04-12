package mom

import (
	"fmt"
	"math"
	"math/cmplx"
	"runtime"
	"sync"

	"gonum.org/v1/gonum/mat"
)

// Simulate runs the full MoM simulation using triangle (rooftop) basis functions.
func Simulate(input SimulationInput) (*SolverResult, error) {
	if len(input.Wires) == 0 {
		return nil, fmt.Errorf("no wires provided")
	}
	if input.Frequency <= 0 {
		return nil, fmt.Errorf("frequency must be positive")
	}

	// Step 1: Subdivide all wires into segments
	var allSegments []Segment
	wireSegOffsets := make([]int, len(input.Wires)) // first segment index per wire
	wireSegCounts := make([]int, len(input.Wires))  // number of segments per wire
	for wi, w := range input.Wires {
		wireSegOffsets[wi] = len(allSegments)
		numSeg := w.Segments
		if numSeg < 3 {
			numSeg = 3 // minimum 3 segments for triangle basis (need at least 1 interior node)
		}
		// Ensure odd number for center feed
		if numSeg%2 == 0 {
			numSeg++
		}
		wireSegCounts[wi] = numSeg
		segs := SubdivideWire(wi, w.X1, w.Y1, w.Z1, w.X2, w.Y2, w.Z2, w.Radius, numSeg)
		for j := range segs {
			segs[j].Index = len(allSegments) + j
		}
		allSegments = append(allSegments, segs...)
	}

	nSeg := len(allSegments)
	if nSeg == 0 {
		return nil, fmt.Errorf("no segments generated")
	}

	// Step 2: Build triangle basis functions at interior nodes
	// For wire w with N segments: interior nodes at indices 1..N-1 (between segments)
	var bases []TriangleBasis
	wireBasisOffsets := make([]int, len(input.Wires))

	for wi := range input.Wires {
		wireBasisOffsets[wi] = len(bases)
		off := wireSegOffsets[wi]
		nSeg := wireSegCounts[wi]

		// Interior nodes 1..N-1: full triangle basis (wire tips forced to I=0)
		for ni := 1; ni < nSeg; ni++ {
			segLeft := &allSegments[off+ni-1]
			segRight := &allSegments[off+ni]
			bases = append(bases, TriangleBasis{
				NodeIndex:       len(bases),
				NodePos:         segLeft.End,
				SegLeft:         segLeft,
				SegRight:        segRight,
				ChargeDensLeft:  -1.0 / (2.0 * segLeft.HalfLength),
				ChargeDensRight: 1.0 / (2.0 * segRight.HalfLength),
			})
		}
	}

	nBasis := len(bases)
	if nBasis == 0 {
		return nil, fmt.Errorf("no basis functions (need at least 3 segments per wire)")
	}

	// Step 3: Determine feed basis index
	feedBasis, err := resolveFeedBasis(input.Source, input.Wires, wireSegOffsets, wireSegCounts, wireBasisOffsets)
	if err != nil {
		return nil, err
	}

	voltage := input.Source.Voltage
	if cmplx.Abs(voltage) == 0 {
		voltage = 1 + 0i
	}

	// Step 4: Frequency parameters
	freq := input.Frequency
	omega := 2.0 * math.Pi * freq
	k := omega / C0

	// Step 5: Build impedance matrix using triangle basis
	Z := buildTriangleZMatrix(bases, allSegments, k, omega)

	// Step 5b: Ground plane
	if input.Ground.Type == "perfect" {
		imageSegs := ApplyPerfectGround(allSegments)
		addGroundTriangleBasis(Z, bases, allSegments, imageSegs, k, omega)
	}

	// Step 6: Build voltage vector
	V := make([]complex128, nBasis)
	V[feedBasis] = voltage

	// Step 7: Solve Z·I = V
	I, err := solveComplexLU(Z, V, nBasis)
	if err != nil {
		return nil, fmt.Errorf("solver failed: %w", err)
	}

	// Step 8: Compute feed impedance
	feedCurrent := I[feedBasis]
	var impedance ComplexImpedance
	if cmplx.Abs(feedCurrent) > 1e-30 {
		zIn := voltage / feedCurrent
		impedance = ComplexImpedance{R: real(zIn), X: imag(zIn)}
	}

	// Step 9: Compute SWR (50-ohm reference)
	zComplex := complex(impedance.R, impedance.X)
	gamma := (zComplex - 50) / (zComplex + 50)
	gammaAbs := cmplx.Abs(gamma)
	swr := 1.0
	if gammaAbs < 1.0 {
		swr = (1.0 + gammaAbs) / (1.0 - gammaAbs)
	} else {
		swr = 999.0
	}

	// Step 10: Interpolate segment currents from basis node currents
	segCurrents := interpolateSegmentCurrents(I, bases, allSegments, wireSegOffsets, wireSegCounts)

	// Step 11: Far-field pattern and gain
	// For perfect ground, include image segments in far-field and restrict to upper hemisphere
	var pattern []PatternPoint
	var gainDBi float64
	if input.Ground.Type == "perfect" {
		imageSegs := ApplyPerfectGround(allSegments)
		// Image currents: same magnitude, direction already handled by ApplyPerfectGround
		pattern, gainDBi = ComputeFarFieldWithGround(allSegments, imageSegs, segCurrents, k)
	} else {
		pattern, gainDBi = ComputeFarField(allSegments, segCurrents, k)
	}

	// Step 12: Build output currents
	currents := make([]CurrentEntry, len(allSegments))
	for i, c := range segCurrents {
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

// buildTriangleZMatrix assembles the impedance matrix using triangle basis.
func buildTriangleZMatrix(bases []TriangleBasis, segments []Segment, k, omega float64) *mat.CDense {
	n := len(bases)
	Z := mat.NewCDense(n, n, nil)

	// Prefactors
	// Vector potential: jωμ/(4π)
	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	// Scalar potential: 1/(jωε·4π) = -jωμ/(4πk²)
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}

	type job struct{ i, j int }
	jobs := make(chan job, 256)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for jb := range jobs {
				vecTerm, scaTerm := TriangleKernel(bases[jb.i], bases[jb.j], k, omega, segments)
				val := vecPrefactor*vecTerm + scaPrefactor*scaTerm
				mu.Lock()
				Z.Set(jb.i, jb.j, val)
				mu.Unlock()
			}
		}()
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			jobs <- job{i, j}
		}
	}
	close(jobs)
	wg.Wait()

	return Z
}

// addGroundTriangleBasis adds perfect ground image contributions to the Z matrix.
func addGroundTriangleBasis(Z *mat.CDense, bases []TriangleBasis, realSegs, imageSegs []Segment, k, omega float64) {
	// Build image basis functions corresponding to each real basis
	imageBases := make([]TriangleBasis, len(bases))
	for i, b := range bases {
		var imgLeft, imgRight *Segment
		if b.SegLeft != nil {
			s := imageSegs[b.SegLeft.Index]
			imgLeft = &s
		}
		if b.SegRight != nil {
			s := imageSegs[b.SegRight.Index]
			imgRight = &s
		}
		imageBases[i] = TriangleBasis{
			NodeIndex:       i,
			NodePos:         [3]float64{b.NodePos[0], b.NodePos[1], -b.NodePos[2]},
			SegLeft:         imgLeft,
			SegRight:        imgRight,
			ChargeDensLeft:  b.ChargeDensLeft,
			ChargeDensRight: b.ChargeDensRight,
		}
	}

	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	n := len(bases)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			vecTerm, scaTerm := TriangleKernel(bases[i], imageBases[j], k, omega, nil)
			val := vecPrefactor*vecTerm + scaPrefactor*scaTerm
			old := Z.At(i, j)
			Z.Set(i, j, old+val)
		}
	}
}

// interpolateSegmentCurrents computes the current at each segment center
// by interpolating from the triangle basis node currents.
func interpolateSegmentCurrents(basisCurrents []complex128, bases []TriangleBasis, segments []Segment, wireSegOffsets, wireSegCounts []int) []complex128 {
	segI := make([]complex128, len(segments))

	for _, b := range bases {
		idx := b.NodeIndex
		if idx >= len(basisCurrents) {
			continue
		}
		Ib := basisCurrents[idx]

		// On the left segment: φ(center) = (center - start) / Δl
		// center of segment is at midpoint, so φ(center) = 0.5
		if b.SegLeft != nil {
			segI[b.SegLeft.Index] += Ib * 0.5
		}
		// On the right segment: φ(center) = 0.5
		if b.SegRight != nil {
			segI[b.SegRight.Index] += Ib * 0.5
		}
	}

	return segI
}

// resolveFeedBasis converts wire/segment source indices to a basis function index.
// Interior nodes are numbered 1..N-1 for a wire with N segments, mapped to
// basis indices starting at wireBasisOffsets[wire]. Segment segIdx maps to the
// nearest interior node: node max(1, segIdx) for base feed, min(N-1, segIdx+1) for tip.
func resolveFeedBasis(src Source, wires []Wire, wireSegOffsets, wireSegCounts []int, wireBasisOffsets []int) (int, error) {
	if src.WireIndex < 0 || src.WireIndex >= len(wires) {
		return 0, fmt.Errorf("source wire_index %d out of range", src.WireIndex)
	}
	nSeg := wireSegCounts[src.WireIndex]
	segIdx := src.SegmentIndex
	if segIdx < 0 || segIdx >= nSeg {
		return 0, fmt.Errorf("source segment_index %d out of range [0, %d)", segIdx, nSeg)
	}

	// Interior node closest to segment segIdx.
	// Node i (1-based) is between segments i-1 and i.
	// Segment segIdx is bounded by node segIdx (start) and node segIdx+1 (end).
	// Use the node at the higher index (closer to segment center for interior segments).
	nodeIdx := segIdx + 1
	if nodeIdx < 1 {
		nodeIdx = 1
	}
	if nodeIdx > nSeg-1 {
		nodeIdx = nSeg - 1
	}

	// Basis index: node 1 = basis offset+0, node 2 = basis offset+1, etc.
	basisIdx := wireBasisOffsets[src.WireIndex] + (nodeIdx - 1)
	return basisIdx, nil
}

// Sweep runs the MoM solver across a frequency range.
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
		result.Frequencies[i] = freq / 1e6

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

// solveComplexLU solves Z*I = V via the 2N×2N real system.
func solveComplexLU(Z *mat.CDense, V []complex128, n int) ([]complex128, error) {
	A := mat.NewDense(2*n, 2*n, nil)
	b := mat.NewVecDense(2*n, nil)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			z := Z.At(i, j)
			re := real(z)
			im := imag(z)
			A.Set(i, j, re)
			A.Set(i, n+j, -im)
			A.Set(n+i, j, im)
			A.Set(n+i, n+j, re)
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

	I := make([]complex128, n)
	for i := 0; i < n; i++ {
		I[i] = complex(x.AtVec(i), x.AtVec(n+i))
	}
	return I, nil
}
