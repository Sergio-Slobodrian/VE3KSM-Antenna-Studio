package mom

import (
	"fmt"
	"math"
	"math/cmplx"
	"runtime"
	"sync"

	"gonum.org/v1/gonum/mat"
)

// Simulate runs the full MoM simulation pipeline using triangle (rooftop) basis
// functions. This is the main entry point for a single-frequency analysis.
//
// The pipeline:
//  1. Subdivide wires into segments (geometry discretization)
//  2. Build triangle basis functions at interior segment junctions
//  3. Resolve the voltage source to a basis function index
//  4. Assemble the impedance matrix Z (MPIE with Gauss-Legendre quadrature)
//  5. Add ground plane image contributions if configured
//  6. Solve the linear system Z·I = V for basis function currents
//  7. Compute feed-point impedance, SWR, segment currents, and far-field pattern
func Simulate(input SimulationInput) (*SolverResult, error) {
	if len(input.Wires) == 0 {
		return nil, fmt.Errorf("no wires provided")
	}
	if input.Frequency <= 0 {
		return nil, fmt.Errorf("frequency must be positive")
	}

	// ---- Step 1: Subdivide all wires into segments ----
	// Each wire is divided into equal-length segments. The segment count is forced
	// to be at least 3 (minimum for one interior node) and odd (so the center
	// segment aligns with a center-fed source).
	var allSegments []Segment
	wireSegOffsets := make([]int, len(input.Wires)) // index of first segment for each wire
	wireSegCounts := make([]int, len(input.Wires))  // number of segments per wire
	for wi, w := range input.Wires {
		wireSegOffsets[wi] = len(allSegments)
		numSeg := w.Segments
		if numSeg < 3 {
			numSeg = 3 // minimum 3 segments: need at least 1 interior node for triangle basis
		}
		if numSeg%2 == 0 {
			numSeg++ // odd count ensures a segment boundary at the wire midpoint for center feed
		}
		wireSegCounts[wi] = numSeg
		segs := SubdivideWire(wi, w.X1, w.Y1, w.Z1, w.X2, w.Y2, w.Z2, w.Radius, numSeg)
		// Assign global indices (SubdivideWire uses local 0-based indices)
		for j := range segs {
			segs[j].Index = len(allSegments) + j
		}
		allSegments = append(allSegments, segs...)
	}

	nSeg := len(allSegments)
	if nSeg == 0 {
		return nil, fmt.Errorf("no segments generated")
	}

	// ---- Step 2: Build triangle basis functions at interior nodes ----
	// A wire with N segments has N-1 interior nodes (junctions between adjacent
	// segments). Each interior node gets a triangle basis function that spans the
	// two segments sharing that node. Wire endpoints have no basis function,
	// which enforces the boundary condition I=0 at open wire ends.
	var bases []TriangleBasis
	wireBasisOffsets := make([]int, len(input.Wires))

	for wi := range input.Wires {
		wireBasisOffsets[wi] = len(bases)
		off := wireSegOffsets[wi]
		nSeg := wireSegCounts[wi]

		for ni := 1; ni < nSeg; ni++ {
			segLeft := &allSegments[off+ni-1]
			segRight := &allSegments[off+ni]
			// Charge density = ±1/Δl where Δl = 2*HalfLength is the full segment length
			bases = append(bases, TriangleBasis{
				NodeIndex:       len(bases),
				NodePos:         segLeft.End, // the junction point between left and right segments
				SegLeft:         segLeft,
				SegRight:        segRight,
				ChargeDensLeft:  -1.0 / (2.0 * segLeft.HalfLength),  // = -1/Δl_left
				ChargeDensRight: 1.0 / (2.0 * segRight.HalfLength),  // = +1/Δl_right
			})
		}
	}

	nBasis := len(bases)
	if nBasis == 0 {
		return nil, fmt.Errorf("no basis functions (need at least 3 segments per wire)")
	}

	// ---- Step 3: Determine which basis function receives the voltage source ----
	feedBasis, err := resolveFeedBasis(input.Source, input.Wires, wireSegOffsets, wireSegCounts, wireBasisOffsets)
	if err != nil {
		return nil, err
	}

	voltage := input.Source.Voltage
	if cmplx.Abs(voltage) == 0 {
		voltage = 1 + 0i // default to 1V source if not specified
	}

	// ---- Step 4: Compute frequency-dependent parameters ----
	freq := input.Frequency
	omega := 2.0 * math.Pi * freq // angular frequency ω (rad/s)
	k := omega / C0               // free-space wavenumber k = ω/c (rad/m)

	// ---- Step 5: Assemble the impedance matrix Z ----
	// Z is nBasis x nBasis, where Z[m][n] is the voltage induced on basis m
	// due to a unit current coefficient on basis n (via the MPIE formulation).
	Z := buildTriangleZMatrix(bases, allSegments, k, omega)

	// Step 5b: Add ground plane contributions via image theory.
	// Both perfect and real ground use geometric image segments; the difference
	// is that real ground scales image contributions by Fresnel reflection
	// coefficients (angle-dependent, lossy) instead of unity.
	switch input.Ground.Type {
	case "perfect":
		imageSegs := ApplyPerfectGround(allSegments)
		addGroundTriangleBasis(Z, bases, allSegments, imageSegs, k, omega)
	case "real":
		imageSegs := ApplyPerfectGround(allSegments)
		addRealGroundTriangleBasis(Z, bases, allSegments, imageSegs, k, omega,
			input.Ground.Conductivity, input.Ground.Permittivity)
	}

	// Step 5c: Inject any lumped R/L/C loads onto the Z-matrix diagonal.
	// Loads are applied AFTER ground contributions so the load impedance
	// adds linearly to whatever the wire's self-radiation impedance is.
	lossPerBasis := make([]float64, nBasis)

	if len(input.Loads) > 0 {
		if err := applyLoads(Z, input.Loads, omega, input.Wires,
			wireSegOffsets, wireSegCounts, wireBasisOffsets, lossPerBasis); err != nil {
			return nil, fmt.Errorf("applying loads: %w", err)
		}
	}

	// Step 5d: Apply skin-effect surface resistance for non-PEC wire
	// materials.  This is the bulk-conductor analogue of lumped loads.
	if err := applyMaterialLoss(cdenseAdder{Z: Z}, input.Wires, allSegments,
		wireSegOffsets, wireSegCounts, wireBasisOffsets, freq, lossPerBasis); err != nil {
		return nil, fmt.Errorf("applying material loss: %w", err)
	}

	// Step 5e: Stamp transmission-line elements (NEC TL cards).  Two-port
	// TLs add to four Z-matrix entries; stubs collapse to a single
	// diagonal stamp.  Resistive parts of Z11/Z22 feed the loss budget.
	if len(input.TransmissionLines) > 0 {
		if err := applyTransmissionLines(Z, input.TransmissionLines, omega,
			input.Wires, wireSegCounts, wireBasisOffsets, lossPerBasis); err != nil {
			return nil, fmt.Errorf("applying transmission lines: %w", err)
		}
	}

	// ---- Step 6: Build the excitation (voltage) vector ----
	// In MoM with delta-gap source model, only the feed basis function has a
	// nonzero voltage; all others are zero (no incident field on the antenna).
	V := make([]complex128, nBasis)
	V[feedBasis] = voltage

	// ---- Step 7: Solve the linear system Z·I = V ----
	// The solution vector I contains the complex current coefficients for each
	// triangle basis function. The system is solved via LU decomposition of the
	// equivalent 2N x 2N real system (see solveComplexLU).
	I, err := solveComplexLU(Z, V, nBasis)
	if err != nil {
		return nil, fmt.Errorf("solver failed: %w", err)
	}

	// ---- Step 8: Compute feed-point impedance ----
	// Z_in = V_feed / I_feed (input impedance seen at the source terminals)
	feedCurrent := I[feedBasis]
	var impedance ComplexImpedance
	if cmplx.Abs(feedCurrent) > 1e-30 {
		zIn := voltage / feedCurrent
		impedance = ComplexImpedance{R: real(zIn), X: imag(zIn)}
	}

	// ---- Step 9: Reflection coefficient and VSWR at the user-supplied
	// reference impedance (defaults to 50 Ω if unset).  We surface both
	// the complex Γ (for Smith-chart plotting) and the scalar VSWR.
	z0 := input.ReferenceImpedance
	if z0 <= 0 {
		z0 = DefaultReferenceImpedance
	}
	gamma := ReflectionCoefficient(impedance, z0)
	swr := VSWRFromGamma(gamma)

	// ---- Step 10: Interpolate segment currents from basis node currents ----
	// The basis function currents are defined at inter-segment nodes; we need
	// the current at each segment center for far-field computation and display.
	segCurrents := interpolateSegmentCurrents(I, bases, allSegments, wireSegOffsets, wireSegCounts)

	// ---- Step 11: Far-field radiation pattern and peak directivity ----
	var pattern []PatternPoint
	var gainDBi float64
	switch input.Ground.Type {
	case "perfect":
		// Perfect ground: image contributions with unity reflection, upper hemisphere only
		imageSegs := ApplyPerfectGround(allSegments)
		pattern, gainDBi = ComputeFarFieldWithGround(allSegments, imageSegs, segCurrents, k)
	case "real":
		// Real ground: image contributions scaled by Fresnel coefficients, upper hemisphere only
		imageSegs := ApplyPerfectGround(allSegments)
		pattern, gainDBi = ComputeFarFieldRealGround(allSegments, imageSegs, segCurrents, k, omega,
			input.Ground.Conductivity, input.Ground.Permittivity)
	default:
		// Free space: full sphere
		pattern, gainDBi = ComputeFarField(allSegments, segCurrents, k)
	}

	// ---- Step 12: Package output ----
	currents := make([]CurrentEntry, len(allSegments))
	for i, c := range segCurrents {
		currents[i] = CurrentEntry{
			SegmentIndex: i,
			Magnitude:    cmplx.Abs(c),
			PhaseDeg:     cmplx.Phase(c) * 180.0 / math.Pi,
		}
	}

	// ---- Step 12b: Headline metrics + 2D polar cuts (post-processing
	// over the existing pattern grid; no additional MoM work required).
	inputPower := FeedInputPower(voltage, feedCurrent)
	lossPower := DissipatedPower(I, lossPerBasis)
	metrics, cuts := ComputeFarFieldMetricsWithLoss(pattern, inputPower, lossPower)

	warnings := ValidateGeometry(input.Wires, freq)
	if input.Ground.Type == "real" {
		for wi, w := range input.Wires {
			if w.Z1 == 0 || w.Z2 == 0 {
				warnings = append(warnings, Warning{
					Code:      "wire_endpoint_on_ground",
					Severity:  SeverityWarn,
					Message:   "wire endpoint sits exactly at z=0; with real ground the Fresnel kernel can become near-singular and produce non-physical impedance.  Lift the endpoint by a few cm or use perfect ground for base-fed verticals",
					WireIndex: wi,
				})
			}
		}
	}
	if impedance.R < 0 {
		// Self-diagnose: re-run with segment counts shifted by +-2 on every
		// wire.  If the perturbed runs give positive R, the failure is most
		// likely a discretisation bandgap at the user’s exact N rather than
		// a real bug or anti-resonance.  Recursion is bounded by skipping the
		// retry inside the perturbed run via the no-retry private flag.
		if !input.skipBandgapRetry {
			perturb := func(delta int) (float64, bool) {
				probe := input
				probe.skipBandgapRetry = true
				probe.Wires = make([]Wire, len(input.Wires))
				copy(probe.Wires, input.Wires)
				for i := range probe.Wires {
					n := probe.Wires[i].Segments + delta
					if n < 3 {
						n = 3
					}
					probe.Wires[i].Segments = n
				}
				res, err := Simulate(probe)
				if err != nil {
					return 0, false
				}
				return res.Impedance.R, true
			}
			rPlus, okPlus := perturb(+2)
			rMinus, okMinus := perturb(-2)
			if okPlus && okMinus && rPlus > 0 && rMinus > 0 {
				warnings = append(warnings, Warning{
					Code:     "discretisation_bandgap",
					Severity: SeverityWarn,
					Message:  fmt.Sprintf("re-running with segments+/-2 produced healthy positive R (%.1f, %.1f Ω); your current N hits a triangle-basis discretisation bandgap at this wavelength.  Try N+/-2 or N+/-5 to escape.", rPlus, rMinus),
				})
			} else if okPlus && rPlus > 0 {
				warnings = append(warnings, Warning{
					Code:     "discretisation_bandgap",
					Severity: SeverityWarn,
					Message:  fmt.Sprintf("re-running with segments+2 produced positive R (%.1f Ω); your current N likely hits a discretisation bandgap.  Try a different N.", rPlus),
				})
			} else if okMinus && rMinus > 0 {
				warnings = append(warnings, Warning{
					Code:     "discretisation_bandgap",
					Severity: SeverityWarn,
					Message:  fmt.Sprintf("re-running with segments-2 produced positive R (%.1f Ω); your current N likely hits a discretisation bandgap.  Try a different N.", rMinus),
				})
			}
		}

		absX := math.Abs(impedance.X)
		absR := math.Abs(impedance.R)
		if absR > 0 && absX/absR > 10 {
			warnings = append(warnings, Warning{
				Code:     "near_anti_resonance",
				Severity: SeverityInfo,
				Message:  fmt.Sprintf("near an anti-resonance: feed-point Z = %.1f %+.1fj Ω with |X|/|R| = %.1f.  At anti-resonances the true |Z| is theoretically infinite and the current at the feed is tiny, so Z = V/I becomes dominated by numerical noise (R can come out small with arbitrary sign).  Shift frequency by 1-3%% to escape the singular point, or accept that |Z| at this exact frequency is very large and the SWR is essentially infinite.", impedance.R, impedance.X, absX/absR),
			})
		} else {
			warnings = append(warnings, Warning{
				Code:     "non_physical_impedance",
				Severity: SeverityError,
				Message:  fmt.Sprintf("solver returned negative real part R = %.2f Ω with |X|/|R| = %.1f; this is unphysical.  Likely causes: (1) a wire endpoint sits exactly on the ground plane (use perfect ground or lift it); (2) Z-matrix ill-conditioning from awkward N at this wavelength.", impedance.R, absX/(absR+1e-30)),
			})
		}
	}

	return &SolverResult{
		Currents:           currents,
		Impedance:          impedance,
		SWR:                swr,
		Reflection:         gamma,
		ReferenceImpedance: z0,
		GainDBi:            gainDBi,
		Pattern:            pattern,
		Metrics:            metrics,
		Cuts:               cuts,
		Warnings:           warnings,
	}, nil
}

// buildTriangleZMatrix assembles the nBasis x nBasis impedance matrix using
// the MPIE formulation with triangle (rooftop) basis functions.
//
// Each matrix element Z[m][n] is composed of two terms from the MPIE:
//
//	Z[m][n] = vecPrefactor * A_mn + scaPrefactor * Φ_mn
//
// where A_mn is the vector potential coupling (current-current interaction)
// and Φ_mn is the scalar potential coupling (charge-charge interaction).
//
// Prefactors:
//   - Vector potential: jωμ₀/(4π)
//   - Scalar potential: 1/(jωε₀·4π) = -jωμ₀/(4πk²)
//     (the identity 1/(jωε₀) = -jωμ₀/k² follows from k² = ω²μ₀ε₀)
//
// Matrix fill is parallelized across runtime.NumCPU() goroutine workers.
func buildTriangleZMatrix(bases []TriangleBasis, segments []Segment, k, omega float64) *mat.CDense {
	n := len(bases)
	Z := mat.NewCDense(n, n, nil)

	// MPIE prefactors (see TriangleKernel for how the integrals are split)
	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))       // jωμ₀/(4π)
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))   // -jωμ₀/(4πk²) = 1/(jωε₀·4π)

	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}

	type job struct{ i, j int }
	jobs := make(chan job, 256)
	var wg sync.WaitGroup
	var mu sync.Mutex // protects Z.Set (gonum CDense is not thread-safe)

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

// addGroundTriangleBasis adds perfect ground plane (PEC at z=0) image
// contributions to the impedance matrix Z using image theory.
//
// For each real basis function, a corresponding image basis is constructed by
// mirroring the segments across z=0 (with direction sign changes per PEC image
// rules — see ApplyPerfectGround). The mutual coupling between each real basis
// (observation) and each image basis (source) is computed and added to the
// existing Z matrix entries. This effectively doubles the number of source
// integrals without increasing the matrix size.
//
// The charge density coefficients are preserved from the real basis because the
// image charge mirrors with the same sign for a PEC ground plane.
func addGroundTriangleBasis(Z *mat.CDense, bases []TriangleBasis, realSegs, imageSegs []Segment, k, omega float64) {
	// Construct image basis functions by mirroring each real basis across z=0
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
			NodePos:         [3]float64{b.NodePos[0], b.NodePos[1], -b.NodePos[2]}, // mirror z
			SegLeft:         imgLeft,
			SegRight:        imgRight,
			ChargeDensLeft:  b.ChargeDensLeft,
			ChargeDensRight: b.ChargeDensRight,
		}
	}

	// Same MPIE prefactors as in buildTriangleZMatrix
	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	// Add image coupling to each Z matrix entry: Z[i][j] += coupling(real_i, image_j)
	n := len(bases)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			// segments arg is nil because image segments are already embedded in imageBases
			vecTerm, scaTerm := TriangleKernel(bases[i], imageBases[j], k, omega, nil)
			val := vecPrefactor*vecTerm + scaPrefactor*scaTerm
			old := Z.At(i, j)
			Z.Set(i, j, old+val)
		}
	}
}

// interpolateSegmentCurrents computes the current at each segment's center
// by evaluating all triangle basis functions at that point.
//
// Each triangle basis contributes to two segments (left and right). At the
// center of each segment, the triangle basis function evaluates to 0.5
// (halfway between 0 at the far end and 1 at the node). The total current at
// a segment center is the sum of contributions from both adjacent basis
// functions (one peaking at each end of the segment).
//
// This produces the physical current distribution needed for far-field
// computation and for reporting segment currents to the user.
func interpolateSegmentCurrents(basisCurrents []complex128, bases []TriangleBasis, segments []Segment, wireSegOffsets, wireSegCounts []int) []complex128 {
	segI := make([]complex128, len(segments))

	for _, b := range bases {
		idx := b.NodeIndex
		if idx >= len(basisCurrents) {
			continue
		}
		Ib := basisCurrents[idx] // complex current coefficient for this basis

		// The triangle basis evaluates to 0.5 at the center of each of its
		// two segments (the midpoint is equidistant from node and far end).
		if b.SegLeft != nil {
			segI[b.SegLeft.Index] += Ib * 0.5
		}
		if b.SegRight != nil {
			segI[b.SegRight.Index] += Ib * 0.5
		}
	}

	return segI
}

// resolveFeedBasis converts user-specified wire/segment source indices into the
// corresponding triangle basis function index in the global basis array.
//
// The mapping works as follows for a wire with N segments:
//   - Segments are numbered 0..N-1
//   - Interior nodes (basis functions) are numbered 1..N-1, where node i sits
//     between segment i-1 and segment i
//   - The basis function closest to segment segIdx is at node segIdx+1,
//     clamped to [1, N-1] to stay within interior nodes
//   - The global basis index is wireBasisOffsets[wire] + (nodeIdx - 1)
//
// This ensures the delta-gap voltage source is applied at the basis function
// nearest to the user-requested segment, which is the standard MoM approach
// for modeling a feed point.
func resolveFeedBasis(src Source, wires []Wire, wireSegOffsets, wireSegCounts []int, wireBasisOffsets []int) (int, error) {
	if src.WireIndex < 0 || src.WireIndex >= len(wires) {
		return 0, fmt.Errorf("source wire_index %d out of range", src.WireIndex)
	}
	nSeg := wireSegCounts[src.WireIndex]
	segIdx := src.SegmentIndex
	if segIdx < 0 || segIdx >= nSeg {
		return 0, fmt.Errorf("source segment_index %d out of range [0, %d)", segIdx, nSeg)
	}

	// Map segment index to the nearest interior node index (1-based).
	// Segment segIdx is bounded by node segIdx (its start) and node segIdx+1 (its end).
	// We pick the higher node to place the source at the segment's end junction.
	nodeIdx := segIdx + 1
	if nodeIdx < 1 {
		nodeIdx = 1 // clamp: first interior node
	}
	if nodeIdx > nSeg-1 {
		nodeIdx = nSeg - 1 // clamp: last interior node
	}

	// Convert 1-based node index to 0-based basis index within this wire,
	// then offset to the global basis array.
	basisIdx := wireBasisOffsets[src.WireIndex] + (nodeIdx - 1)
	return basisIdx, nil
}

// Sweep runs the MoM solver at multiple frequency points across a range,
// producing impedance and SWR vs. frequency data suitable for plotting.
//
// The frequency range [freqStartHz, freqEndHz] is divided into (steps-1)
// equal intervals. At each point, a full Simulate() call is performed
// (geometry discretization, matrix assembly, and solve). Each frequency step
// is independent — the impedance matrix is rebuilt from scratch because the
// wavenumber k changes.
//
// Frequencies in the output are converted to MHz for display convenience.
func Sweep(input SimulationInput, freqStartHz, freqEndHz float64, steps int) (*SweepResult, error) {
	if steps < 2 {
		return nil, fmt.Errorf("frequency sweep requires at least 2 steps")
	}

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

	stepSize := (freqEndHz - freqStartHz) / float64(steps-1)

	for i := 0; i < steps; i++ {
		freq := freqStartHz + float64(i)*stepSize
		result.Frequencies[i] = freq / 1e6 // store in MHz

		stepInput := input
		stepInput.Frequency = freq

		res, err := Simulate(stepInput)
		if err != nil {
			return nil, fmt.Errorf("sweep failed at %.3f MHz: %w", freq/1e6, err)
		}

		result.SWR[i] = res.SWR
		result.Impedance[i] = res.Impedance
		result.Reflections[i] = res.Reflection
	}

	// Validate the geometry at both ends of the sweep (a sweep typically
	// spans much wider frequency ranges than a single Simulate call, so
	// the start may have segments-too-short warnings while the end has
	// segments-too-long ones).  Dedupe by code so the banner stays tidy.
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
	_ = seen

	// A sweep wider than ~20:1 in frequency cannot satisfy both NEC
	// accuracy bounds with any single segment count: the high-freq end
	// needs N >= 10*L*f_high/c (lambda/10) while the low-freq end needs
	// N <= 200*L*f_low/c (lambda/200).  Both satisfied only if
	// f_high/f_low <= 20.  Without this advisory the user sees the
	// two warnings *serially* as they adjust N and ends up chasing
	// them in circles.  10:1 catches the "tight but possible" zone.
	if freqStartHz > 0 {
		ratio := freqEndHz / freqStartHz
		if ratio > 10 {
			sev := SeverityInfo
			msg := fmt.Sprintf(
				"sweep ratio %.1f:1 (%.3f→%.3f MHz) is wider than any fixed segment count can fully satisfy; expect impedance drift near each band edge.  Either narrow the sweep or split it into bands and pick segments per band",
				ratio, freqStartHz/1e6, freqEndHz/1e6)
			if ratio > 20 {
				sev = SeverityWarn
				msg = fmt.Sprintf(
					"sweep ratio %.1f:1 (%.3f→%.3f MHz) exceeds 20:1; no fixed segment count satisfies both NEC accuracy bounds (λ/200 low, λ/20 high).  Results near each extreme will be approximate; split the sweep into bands for trustworthy numbers",
					ratio, freqStartHz/1e6, freqEndHz/1e6)
			}
			result.Warnings = append(result.Warnings, Warning{
				Code:     "sweep_range_unsatisfiable",
				Severity: sev,
				Message:  msg,
			})
			// Downgrade the per-end seg-length warnings: with the
			// range-unsatisfiable advisory now visible, they are
			// just expected consequences and don't need their own
			// alarm colour.
			downgrade := map[string]bool{
				"segment_too_long":               true,
				"segment_below_lambda_over_20":   true,
				"segment_too_short_for_frequency": true,
				"segment_short_for_frequency":     true,
			}
			for i := range result.Warnings {
				w := &result.Warnings[i]
				if downgrade[w.Code] {
					w.Severity = SeverityInfo
				}
			}
		}
	}

	return result, nil
}

// solveComplexLU solves the complex linear system Z·I = V by converting it to an
// equivalent 2N x 2N real system and using LU decomposition.
//
// The complex equation (A + jB)(x + jy) = (c + jd) is rewritten as:
//
//	| A  -B | | x |   | c |
//	| B   A | | y | = | d |
//
// where A = Re(Z), B = Im(Z), x = Re(I), y = Im(I), c = Re(V), d = Im(V).
// This avoids the need for a complex LU implementation (gonum only provides
// real LU). The 2N system is twice as large but uses only real arithmetic.
func solveComplexLU(Z *mat.CDense, V []complex128, n int) ([]complex128, error) {
	// Build the 2N x 2N real system
	A := mat.NewDense(2*n, 2*n, nil)
	b := mat.NewVecDense(2*n, nil)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			z := Z.At(i, j)
			re := real(z)
			im := imag(z)
			// Top-left block: Re(Z), top-right block: -Im(Z)
			A.Set(i, j, re)
			A.Set(i, n+j, -im)
			// Bottom-left block: Im(Z), bottom-right block: Re(Z)
			A.Set(n+i, j, im)
			A.Set(n+i, n+j, re)
		}
		// RHS: [Re(V); Im(V)]
		b.SetVec(i, real(V[i]))
		b.SetVec(n+i, imag(V[i]))
	}

	// LU factorization and solve
	var lu mat.LU
	lu.Factorize(A)

	x := mat.NewVecDense(2*n, nil)
	if err := lu.SolveVecTo(x, false, b); err != nil {
		return nil, fmt.Errorf("LU solve failed: %w", err)
	}

	// Reconstruct complex solution: I[i] = x[i] + j*x[n+i]
	I := make([]complex128, n)
	for i := 0; i < n; i++ {
		I[i] = complex(x.AtVec(i), x.AtVec(n+i))
	}
	return I, nil
}
