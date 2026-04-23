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
	"math/cmplx"
	"sort"

	"gonum.org/v1/gonum/mat"
)

// CMAMode holds one characteristic mode from the eigendecomposition.
type CMAMode struct {
	Index              int       `json:"index"`               // 1-based mode number (sorted by significance)
	Eigenvalue         float64   `json:"eigenvalue"`          // λ_n (real; from generalised eigenproblem)
	ModalSignificance  float64   `json:"modal_significance"`  // MS_n = 1/√(1+λ_n²), range [0,1]
	CharacteristicAngle float64  `json:"characteristic_angle"` // α_n = 180° − atan(λ_n), degrees
	// Modal current magnitudes on each segment (for visualisation).
	// Length = number of segments; indexed by global segment index.
	CurrentMagnitudes []float64 `json:"current_magnitudes"`
}

// CMAResult holds the full set of characteristic modes at one frequency.
type CMAResult struct {
	Modes     []CMAMode `json:"modes"`      // sorted by modal significance (highest first)
	NumModes  int       `json:"num_modes"`  // total number of modes (= number of basis functions)
	FreqMHz   float64   `json:"freq_mhz"`   // analysis frequency
}

// ComputeCMA performs Characteristic Mode Analysis on the fully-assembled
// MoM impedance matrix Z.  It solves the generalised eigenproblem
//
//	X · J_n = λ_n · R · J_n
//
// where Z = R + jX (R = Re(Z), X = Im(Z)) are real symmetric matrices for
// a reciprocal antenna structure.
//
// The algorithm:
//  1. Extract R and X as real SymDense matrices.
//  2. Cholesky-factorise R = L · Lᵀ (requires R positive-definite).
//  3. Form M = L⁻¹ · X · L⁻ᵀ  (real, symmetric).
//  4. Symmetric eigendecomposition of M → eigenvalues λ_n, eigenvectors y_n.
//  5. Recover modal currents J_n = L⁻ᵀ · y_n.
//  6. Compute modal significance MS_n and characteristic angle α_n.
//
// Returns modes sorted by decreasing modal significance.
func ComputeCMA(Z *mat.CDense, segments []Segment, bases []TriangleBasis,
	wireSegOffsets, wireSegCounts []int) (*CMAResult, error) {

	n, _ := Z.Dims()
	if n == 0 {
		return nil, fmt.Errorf("empty Z-matrix")
	}

	// Step 1: Extract R = Re(Z) and X = Im(Z) as SymDense.
	// For a reciprocal antenna, Z is symmetric, so R and X are symmetric.
	// We symmetrise explicitly: R_ij = (Re(Z_ij) + Re(Z_ji)) / 2
	R := mat.NewSymDense(n, nil)
	X := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			zij := Z.At(i, j)
			zji := Z.At(j, i)
			rVal := (real(zij) + real(zji)) / 2.0
			xVal := (imag(zij) + imag(zji)) / 2.0
			R.SetSym(i, j, rVal)
			X.SetSym(i, j, xVal)
		}
	}

	// Step 2: Cholesky factorisation of R.
	// R must be positive-definite (it represents radiation + ohmic loss power).
	// If R is only semi-definite (e.g. a structure that barely radiates),
	// add a tiny diagonal regularisation to make it positive-definite.
	var chol mat.Cholesky
	if ok := chol.Factorize(R); !ok {
		// Regularise: add small fraction of the diagonal norm
		diagNorm := 0.0
		for i := 0; i < n; i++ {
			diagNorm += R.At(i, i)
		}
		eps := 1e-10 * diagNorm / float64(n)
		if eps < 1e-20 {
			eps = 1e-20
		}
		for i := 0; i < n; i++ {
			R.SetSym(i, i, R.At(i, i)+eps)
		}
		if ok := chol.Factorize(R); !ok {
			return nil, fmt.Errorf("CMA: R-matrix Cholesky failed even after regularisation; R may not be positive-definite")
		}
	}

	// Extract L from Cholesky: R = L · Lᵀ  where L is lower-triangular.
	var L mat.TriDense
	chol.LTo(&L)

	// Step 3: Form M = L⁻¹ · X · L⁻ᵀ  (symmetric).
	// First compute L⁻¹ by solving L · Y = I.
	Linv := mat.NewDense(n, n, nil)
	eye := mat.NewDense(n, n, nil)
	for i := 0; i < n; i++ {
		eye.Set(i, i, 1.0)
	}
	if err := Linv.Solve(&L, eye); err != nil {
		return nil, fmt.Errorf("CMA: failed to invert Cholesky factor L: %w", err)
	}

	// Xdense for multiplication
	Xdense := mat.NewDense(n, n, nil)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			v := X.At(i, j)
			Xdense.Set(i, j, v)
			Xdense.Set(j, i, v)
		}
	}

	// M = Linv · X · Linvᵀ
	var tmp mat.Dense
	tmp.Mul(Linv, Xdense) // tmp = L⁻¹ · X
	var Mdense mat.Dense
	Mdense.Mul(&tmp, Linv.T()) // M = tmp · L⁻ᵀ

	// Symmetrise M explicitly (numerical noise can break strict symmetry)
	Msym := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			v := (Mdense.At(i, j) + Mdense.At(j, i)) / 2.0
			Msym.SetSym(i, j, v)
		}
	}

	// Step 4: Symmetric eigendecomposition M · y = λ · y.
	var eig mat.EigenSym
	if ok := eig.Factorize(Msym, true); !ok {
		return nil, fmt.Errorf("CMA: symmetric eigendecomposition failed")
	}

	eigenvalues := make([]float64, n)
	eig.Values(eigenvalues)

	var evecs mat.Dense
	eig.VectorsTo(&evecs)

	// Step 5: Recover modal currents J_n = L⁻ᵀ · y_n  and interpolate to segments.
	// L⁻ᵀ = (L⁻¹)ᵀ  which is Linvᵀ.
	var Jbasis mat.Dense
	Jbasis.Mul(Linv.T(), &evecs) // each column is a modal current in basis-function space

	// Build modes
	modes := make([]CMAMode, n)
	for m := 0; m < n; m++ {
		lam := eigenvalues[m]
		ms := 1.0 / math.Sqrt(1.0+lam*lam)
		alpha := 180.0 - math.Atan(lam)*180.0/math.Pi

		// Extract basis-function currents for this mode (column m of Jbasis)
		basisCurrents := make([]complex128, n)
		for i := 0; i < n; i++ {
			basisCurrents[i] = complex(Jbasis.At(i, m), 0) // modal currents are real-valued
		}

		// Interpolate to segment currents
		segCurrents := interpolateSegmentCurrents(basisCurrents, bases, segments,
			wireSegOffsets, wireSegCounts)

		// Store magnitudes for visualisation
		mags := make([]float64, len(segCurrents))
		for i, c := range segCurrents {
			mags[i] = cmplx.Abs(c)
		}

		// Normalise magnitudes so the peak is 1.0
		peak := 0.0
		for _, v := range mags {
			if v > peak {
				peak = v
			}
		}
		if peak > 0 {
			for i := range mags {
				mags[i] /= peak
			}
		}

		modes[m] = CMAMode{
			Index:               m + 1,
			Eigenvalue:          lam,
			ModalSignificance:   ms,
			CharacteristicAngle: alpha,
			CurrentMagnitudes:   mags,
		}
	}

	// Sort by decreasing modal significance (most significant first)
	sort.Slice(modes, func(i, j int) bool {
		return modes[i].ModalSignificance > modes[j].ModalSignificance
	})
	// Re-index after sorting
	for i := range modes {
		modes[i].Index = i + 1
	}

	return &CMAResult{
		Modes:    modes,
		NumModes: n,
	}, nil
}

// SimulateCMA runs the MoM Z-matrix assembly pipeline and then performs
// Characteristic Mode Analysis.  This is the top-level entry point called
// by the API handler.
func SimulateCMA(input SimulationInput) (*CMAResult, error) {
	if len(input.Wires) == 0 {
		return nil, fmt.Errorf("no wires provided")
	}
	if input.Frequency <= 0 {
		return nil, fmt.Errorf("frequency must be positive")
	}

	// --- Replicate Steps 1-5 from Simulate to build the fully-assembled Z ---
	var allSegments []Segment
	wireSegOffsets := make([]int, len(input.Wires))
	wireSegCounts := make([]int, len(input.Wires))
	for wi, w := range input.Wires {
		wireSegOffsets[wi] = len(allSegments)
		numSeg := w.Segments
		if numSeg < 3 {
			numSeg = 3
		}
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
	if len(allSegments) == 0 {
		return nil, fmt.Errorf("no segments generated")
	}

	// Build triangle basis functions
	var bases []TriangleBasis
	wireBasisOffsets := make([]int, len(input.Wires))
	wireStartJunction := make([]bool, len(input.Wires))
	wireEndJunction := make([]bool, len(input.Wires))
	hasGround := input.Ground.Type == "perfect" || input.Ground.Type == "real"

	for wi := range input.Wires {
		wireBasisOffsets[wi] = len(bases)
		off := wireSegOffsets[wi]
		nSeg := wireSegCounts[wi]
		w := input.Wires[wi]

		if hasGround && w.Z1 == 0 {
			wireStartJunction[wi] = true
			segRight := &allSegments[off]
			bases = append(bases, TriangleBasis{
				NodeIndex: len(bases), NodePos: segRight.Start,
				SegLeft: nil, SegRight: segRight,
				ChargeDensLeft: 0, ChargeDensRight: 1.0 / (2.0 * segRight.HalfLength),
			})
		}
		for ni := 1; ni < nSeg; ni++ {
			segLeft := &allSegments[off+ni-1]
			segRight := &allSegments[off+ni]
			bases = append(bases, TriangleBasis{
				NodeIndex: len(bases), NodePos: segLeft.End,
				SegLeft: segLeft, SegRight: segRight,
				ChargeDensLeft: -1.0 / (2.0 * segLeft.HalfLength),
				ChargeDensRight: 1.0 / (2.0 * segRight.HalfLength),
			})
		}
		if hasGround && w.Z2 == 0 {
			wireEndJunction[wi] = true
			segLeft := &allSegments[off+nSeg-1]
			bases = append(bases, TriangleBasis{
				NodeIndex: len(bases), NodePos: segLeft.End,
				SegLeft: segLeft, SegRight: nil,
				ChargeDensLeft: -1.0 / (2.0 * segLeft.HalfLength),
				ChargeDensRight: 0,
			})
		}
	}
	// Add cross-wire junction bases (same as Simulate) so the Z-matrix
	// includes coupling across shared wire endpoints (e.g. inverted-V apex).
	// Without these, R = Re(Z) is not positive-definite and Cholesky fails.
	addCrossWireJunctions(&bases, input.Wires, allSegments,
		wireSegOffsets, wireSegCounts,
		wireEndJunction, wireStartJunction)
	nBasis := len(bases)
	if nBasis == 0 {
		return nil, fmt.Errorf("no basis functions")
	}

	freq := input.Frequency
	omega := 2.0 * math.Pi * freq
	k := omega / C0

	// Assemble Z-matrix (same pipeline as Simulate)
	Z := buildTriangleZMatrix(bases, allSegments, k, omega)

	switch input.Ground.Type {
	case "perfect":
		imageSegs := ApplyPerfectGround(allSegments)
		addGroundTriangleBasis(Z, bases, allSegments, imageSegs, k, omega)
	case "real":
		imageSegs := ApplyPerfectGround(allSegments)
		addComplexImageGroundBasis(Z, bases, allSegments, imageSegs, k, omega,
			input.Ground.Conductivity, input.Ground.Permittivity)
	}

	lossPerBasis := make([]float64, nBasis)
	if len(input.Loads) > 0 {
		if err := applyLoads(Z, input.Loads, omega, input.Wires,
			wireSegOffsets, wireSegCounts, wireBasisOffsets, lossPerBasis); err != nil {
			return nil, fmt.Errorf("applying loads: %w", err)
		}
	}
	if err := applyMaterialLoss(cdenseAdder{Z: Z}, input.Wires, allSegments,
		wireSegOffsets, wireSegCounts, wireBasisOffsets, freq, lossPerBasis); err != nil {
		return nil, fmt.Errorf("applying material loss: %w", err)
	}
	if len(input.TransmissionLines) > 0 {
		if err := applyTransmissionLines(Z, input.TransmissionLines, omega,
			input.Wires, wireSegCounts, wireBasisOffsets, lossPerBasis); err != nil {
			return nil, fmt.Errorf("applying transmission lines: %w", err)
		}
	}

	// Suppress unused variables
	_ = wireStartJunction
	_ = wireEndJunction

	// Perform CMA on the fully-assembled Z
	result, err := ComputeCMA(Z, allSegments, bases, wireSegOffsets, wireSegCounts)
	if err != nil {
		return nil, err
	}
	result.FreqMHz = freq / 1e6

	return result, nil
}
