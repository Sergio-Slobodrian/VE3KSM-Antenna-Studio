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
	"math"
	"math/cmplx"
	"testing"
)

// TestDiag_FreeSpaceDipole checks that a half-wave dipole in free space
// gives the expected Z ≈ 73 + j42 Ω.  This validates the base MPIE solver
// independent of any ground code.
func TestDiag_FreeSpaceDipole(t *testing.T) {
	freq := 14e6
	lambda := C0 / freq
	halfL := lambda / 4.0

	input := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: -halfL,
			X2: 0, Y2: 0, Z2: halfL,
			Radius: 1e-3, Segments: 21,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}

	res, err := Simulate(input)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}

	t.Logf("Free-space dipole: Z = %.2f %+.2fj Ω, SWR = %.2f",
		res.Impedance.R, res.Impedance.X, res.SWR)

	if res.Impedance.R < 30 || res.Impedance.R > 150 {
		t.Errorf("R = %.2f Ω, expected ~73 Ω (range [30,150])", res.Impedance.R)
	}
	if math.Abs(res.Impedance.X) > 200 {
		t.Errorf("|X| = %.2f Ω, expected ~42 (range [-200,200])", math.Abs(res.Impedance.X))
	}
}

// TestDiag_FreeSpaceQuarterWave checks what a lone λ/4 wire gives
// in free space (NOT a monopole — no ground).  This should be a high-
// impedance, capacitive stub, NOT a resonant monopole.
func TestDiag_FreeSpaceQuarterWave(t *testing.T) {
	freq := 14e6
	lambda := C0 / freq

	input := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: 0.05,
			X2: 0, Y2: 0, Z2: lambda / 4,
			Radius: 1e-3, Segments: 21,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 0, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}

	res, err := Simulate(input)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}

	t.Logf("Free-space λ/4 wire (bottom-fed): Z = %.2f %+.2fj Ω",
		res.Impedance.R, res.Impedance.X)
}

// TestDiag_ExplicitDipoleVsMonopole creates an explicit dipole by specifying
// two wires (one above z=0, one below) vs the ground-image monopole.
// If they differ, the ground image assembly is at fault.
func TestDiag_ExplicitDipoleVsMonopole(t *testing.T) {
	freq := 14e6
	lambda := C0 / freq
	halfL := lambda / 4.0

	// --- Case 1: Monopole over perfect ground (base at z=0) ---
	monopoleInput := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: 0,
			X2: 0, Y2: 0, Z2: halfL,
			Radius: 1e-3, Segments: 21,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 0, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "perfect"},
	}

	monoRes, err := Simulate(monopoleInput)
	if err != nil {
		t.Fatalf("Monopole Simulate: %v", err)
	}

	// --- Case 2: Full dipole in free space (center-fed) ---
	dipoleInput := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: -halfL,
			X2: 0, Y2: 0, Z2: halfL,
			Radius: 1e-3, Segments: 41, // double segments for similar per-arm resolution
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 20, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}

	dipRes, err := Simulate(dipoleInput)
	if err != nil {
		t.Fatalf("Dipole Simulate: %v", err)
	}

	t.Logf("Monopole (perfect ground): Z = %.2f %+.2fj Ω", monoRes.Impedance.R, monoRes.Impedance.X)
	t.Logf("Dipole (free space):       Z = %.2f %+.2fj Ω", dipRes.Impedance.R, dipRes.Impedance.X)
	t.Logf("Dipole/2 (expected mono):  Z = %.2f %+.2fj Ω", dipRes.Impedance.R/2, dipRes.Impedance.X/2)
}

// TestDiag_ZMatrixScalarMagnitude prints the magnitude of the vector and
// scalar potential contributions to the Z-matrix diagonal for the feed basis,
// with and without ground.  This shows whether the scalar potential is
// disproportionately large.
func TestDiag_ZMatrixScalarMagnitude(t *testing.T) {
	freq := 14e6
	omega := 2 * math.Pi * freq
	k := omega / C0
	lambda := C0 / freq
	halfL := lambda / 4.0

	// Build a quarter-wave vertical.
	wire := Wire{
		X1: 0, Y1: 0, Z1: 0.05,
		X2: 0, Y2: 0, Z2: halfL,
		Radius: 1e-3, Segments: 21,
	}
	numSeg := 21

	segs := SubdivideWire(0, wire.X1, wire.Y1, wire.Z1, wire.X2, wire.Y2, wire.Z2, wire.Radius, wire.Radius, numSeg)
	for j := range segs {
		segs[j].Index = j
	}

	// Build triangle bases (same as solver.go).
	var bases []TriangleBasis
	for ni := 1; ni < numSeg; ni++ {
		segLeft := &segs[ni-1]
		segRight := &segs[ni]
		bases = append(bases, TriangleBasis{
			NodeIndex:       len(bases),
			NodePos:         segRight.Start,
			SegLeft:         segLeft,
			SegRight:        segRight,
			ChargeDensLeft:  -1.0 / (2.0 * segLeft.HalfLength),
			ChargeDensRight: 1.0 / (2.0 * segRight.HalfLength),
		})
	}

	feedIdx := 0 // first interior node = feed

	// MPIE prefactors.
	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	// Compute feed-point self-coupling (diagonal element Z[feed][feed]).
	vecTerm, scaTerm := TriangleKernel(bases[feedIdx], bases[feedIdx], k, omega, segs)
	zVec := vecPrefactor * vecTerm
	zSca := scaPrefactor * scaTerm
	zDirect := zVec + zSca

	t.Logf("Feed self-term (direct):")
	t.Logf("  vecPrefactor = %v", vecPrefactor)
	t.Logf("  scaPrefactor = %v", scaPrefactor)
	t.Logf("  vecTerm = %v  (|vec| = %.4f)", vecTerm, cmplx.Abs(vecTerm))
	t.Logf("  scaTerm = %v  (|sca| = %.4f)", scaTerm, cmplx.Abs(scaTerm))
	t.Logf("  Z_vec = %v  (Im = %.2f)", zVec, imag(zVec))
	t.Logf("  Z_sca = %v  (Im = %.2f)", zSca, imag(zSca))
	t.Logf("  Z_total = %v  (Im = %.2f)", zDirect, imag(zDirect))

	// Now add image contribution for perfect ground.
	imageSegs := ApplyPerfectGround(segs)
	var imgLeft, imgRight *Segment
	if bases[feedIdx].SegLeft != nil {
		s := imageSegs[bases[feedIdx].SegLeft.Index]
		imgLeft = &s
	}
	if bases[feedIdx].SegRight != nil {
		s := imageSegs[bases[feedIdx].SegRight.Index]
		imgRight = &s
	}
	imgBasis := TriangleBasis{
		NodeIndex:       feedIdx,
		NodePos:         [3]float64{bases[feedIdx].NodePos[0], bases[feedIdx].NodePos[1], -bases[feedIdx].NodePos[2]},
		SegLeft:         imgLeft,
		SegRight:        imgRight,
		ChargeDensLeft:  -bases[feedIdx].ChargeDensLeft,  // negated per current code
		ChargeDensRight: -bases[feedIdx].ChargeDensRight,
	}

	vecTermImg, scaTermImg := TriangleKernel(bases[feedIdx], imgBasis, k, omega, nil)
	zVecImg := vecPrefactor * vecTermImg
	zScaImg := scaPrefactor * scaTermImg

	t.Logf("Feed self-term (image):")
	t.Logf("  vecTerm_img = %v  (|vec| = %.4f)", vecTermImg, cmplx.Abs(vecTermImg))
	t.Logf("  scaTerm_img = %v  (|sca| = %.4f)", scaTermImg, cmplx.Abs(scaTermImg))
	t.Logf("  Z_vec_img = %v  (Im = %.2f)", zVecImg, imag(zVecImg))
	t.Logf("  Z_sca_img = %v  (Im = %.2f)", zScaImg, imag(zScaImg))

	// Also try with SAME charge sign (un-negated).
	imgBasisSameCharge := imgBasis
	imgBasisSameCharge.ChargeDensLeft = bases[feedIdx].ChargeDensLeft
	imgBasisSameCharge.ChargeDensRight = bases[feedIdx].ChargeDensRight

	_, scaTermImgSame := TriangleKernel(bases[feedIdx], imgBasisSameCharge, k, omega, nil)
	zScaImgSame := scaPrefactor * scaTermImgSame
	t.Logf("  Z_sca_img (same charge) = %v  (Im = %.2f)", zScaImgSame, imag(zScaImgSame))

	t.Logf("Total Z[feed][feed] with ground = %v", zDirect+zVecImg+zScaImg)

	// Sum entire feed row (all off-diagonal + diagonal) for direct and image.
	n := len(bases)
	var sumVecDirect, sumScaDirect complex128
	var sumVecImage, sumScaImage complex128
	for j := 0; j < n; j++ {
		vd, sd := TriangleKernel(bases[feedIdx], bases[j], k, omega, segs)
		sumVecDirect += vecPrefactor * vd
		sumScaDirect += scaPrefactor * sd

		var iL, iR *Segment
		if bases[j].SegLeft != nil {
			s := imageSegs[bases[j].SegLeft.Index]
			iL = &s
		}
		if bases[j].SegRight != nil {
			s := imageSegs[bases[j].SegRight.Index]
			iR = &s
		}
		ib := TriangleBasis{
			NodeIndex:       j,
			NodePos:         [3]float64{bases[j].NodePos[0], bases[j].NodePos[1], -bases[j].NodePos[2]},
			SegLeft:         iL,
			SegRight:        iR,
			ChargeDensLeft:  -bases[j].ChargeDensLeft,
			ChargeDensRight: -bases[j].ChargeDensRight,
		}
		vi, si := TriangleKernel(bases[feedIdx], ib, k, omega, nil)
		sumVecImage += vecPrefactor * vi
		sumScaImage += scaPrefactor * si
	}
	t.Logf("Full feed row sum (direct):  vec = Im %.2f,  sca = Im %.2f,  total = Im %.2f",
		imag(sumVecDirect), imag(sumScaDirect), imag(sumVecDirect+sumScaDirect))
	t.Logf("Full feed row sum (image):   vec = Im %.2f,  sca = Im %.2f,  total = Im %.2f",
		imag(sumVecImage), imag(sumScaImage), imag(sumVecImage+sumScaImage))
	t.Logf("Grand total feed row:        Im(vec) = %.2f, Im(sca) = %.2f, Im(total) = %.2f",
		imag(sumVecDirect+sumVecImage), imag(sumScaDirect+sumScaImage),
		imag(sumVecDirect+sumScaDirect+sumVecImage+sumScaImage))
}

// TestDiag_FullZDiagonal prints the Z-matrix diagonal for the quarter-wave
// monopole (free space and perfect ground) so we can compare magnitudes.
func TestDiag_FullZDiagonal(t *testing.T) {
	freq := 14e6
	omega := 2 * math.Pi * freq
	k := omega / C0
	lambda := C0 / freq
	halfL := lambda / 4.0

	wire := Wire{
		X1: 0, Y1: 0, Z1: 0.05,
		X2: 0, Y2: 0, Z2: halfL,
		Radius: 1e-3, Segments: 21,
	}
	numSeg := 21

	segs := SubdivideWire(0, wire.X1, wire.Y1, wire.Z1, wire.X2, wire.Y2, wire.Z2, wire.Radius, wire.Radius, numSeg)
	for j := range segs {
		segs[j].Index = j
	}

	var bases []TriangleBasis
	for ni := 1; ni < numSeg; ni++ {
		segLeft := &segs[ni-1]
		segRight := &segs[ni]
		bases = append(bases, TriangleBasis{
			NodeIndex:       len(bases),
			NodePos:         segRight.Start,
			SegLeft:         segLeft,
			SegRight:        segRight,
			ChargeDensLeft:  -1.0 / (2.0 * segLeft.HalfLength),
			ChargeDensRight: 1.0 / (2.0 * segRight.HalfLength),
		})
	}

	Z := buildTriangleZMatrix(bases, segs, k, omega)

	t.Logf("Free-space Z diagonal (first 5 and last 2):")
	n := len(bases)
	for _, i := range []int{0, 1, 2, 3, 4, n - 2, n - 1} {
		z := Z.At(i, i)
		t.Logf("  Z[%2d][%2d] = %10.2f %+10.2fj", i, i, real(z), imag(z))
	}

	// Add perfect ground and print diagonal again.
	imageSegs := ApplyPerfectGround(segs)
	addGroundTriangleBasis(Z, bases, segs, imageSegs, k, omega)

	t.Logf("After perfect ground Z diagonal (first 5 and last 2):")
	for _, i := range []int{0, 1, 2, 3, 4, n - 2, n - 1} {
		z := Z.At(i, i)
		t.Logf("  Z[%2d][%2d] = %10.2f %+10.2fj", i, i, real(z), imag(z))
	}

	// Solve with delta-gap at feed (basis 0).
	V := make([]complex128, n)
	V[0] = 1 + 0i

	I, err := solveComplexLU(Z, V, n)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}

	Zin := V[0] / I[0]
	t.Logf("Z_in = %.2f %+.2fj Ω", real(Zin), imag(Zin))

	// Also solve free-space only for comparison.
	Z2 := buildTriangleZMatrix(bases, segs, k, omega)
	I2, err := solveComplexLU(Z2, V, n)
	if err != nil {
		t.Fatalf("solve free-space: %v", err)
	}
	Zin2 := V[0] / I2[0]
	t.Logf("Z_in (free space only) = %.2f %+.2fj Ω", real(Zin2), imag(Zin2))
}

// TestDiag_ImageKernelConsistency checks that the TriangleKernel for
// real-to-image coupling gives results consistent with real-to-real coupling
// when the "image" is placed at the geometric image position.
func TestDiag_ImageKernelConsistency(t *testing.T) {
	freq := 14e6
	omega := 2 * math.Pi * freq
	k := omega / C0

	// Create a simple 3-segment wire from z=1 to z=2.
	segs := SubdivideWire(0, 0, 0, 1, 0, 0, 2, 0.001, 0.001, 3)
	for j := range segs {
		segs[j].Index = j
	}

	// Build basis at node 1 (between segments 0 and 1).
	b := TriangleBasis{
		NodeIndex:       0,
		NodePos:         segs[1].Start,
		SegLeft:         &segs[0],
		SegRight:        &segs[1],
		ChargeDensLeft:  -1.0 / (2.0 * segs[0].HalfLength),
		ChargeDensRight: 1.0 / (2.0 * segs[1].HalfLength),
	}

	// Image segments (mirror across z=0).
	imageSegs := ApplyPerfectGround(segs)

	imgBasis := TriangleBasis{
		NodeIndex: 0,
		NodePos:   [3]float64{b.NodePos[0], b.NodePos[1], -b.NodePos[2]},
		SegLeft:   &imageSegs[0],
		SegRight:  &imageSegs[1],
		// Test BOTH charge sign conventions.
		ChargeDensLeft:  b.ChargeDensLeft,
		ChargeDensRight: b.ChargeDensRight,
	}

	vecSame, scaSame := TriangleKernel(b, imgBasis, k, omega, nil)

	imgBasisNeg := imgBasis
	imgBasisNeg.ChargeDensLeft = -b.ChargeDensLeft
	imgBasisNeg.ChargeDensRight = -b.ChargeDensRight

	vecNeg, scaNeg := TriangleKernel(b, imgBasisNeg, k, omega, nil)

	// Self-coupling for reference.
	vecSelf, scaSelf := TriangleKernel(b, b, k, omega, segs)

	prefV := complex(0, omega*Mu0/(4.0*math.Pi))
	prefS := -complex(0, omega*Mu0/(4.0*math.Pi*k*k))

	t.Logf("Self:          Z_vec = Im %.4f,  Z_sca = Im %.4f,  Z_total = Im %.4f",
		imag(prefV*vecSelf), imag(prefS*scaSelf), imag(prefV*vecSelf+prefS*scaSelf))
	t.Logf("Image (same):  Z_vec = Im %.4f,  Z_sca = Im %.4f,  Z_total = Im %.4f",
		imag(prefV*vecSame), imag(prefS*scaSame), imag(prefV*vecSame+prefS*scaSame))
	t.Logf("Image (neg):   Z_vec = Im %.4f,  Z_sca = Im %.4f,  Z_total = Im %.4f",
		imag(prefV*vecNeg), imag(prefS*scaNeg), imag(prefV*vecNeg+prefS*scaNeg))
	t.Logf("vec terms: self=%.6f, image=%.6f (should match for vertical wire)",
		cmplx.Abs(vecSelf), cmplx.Abs(vecSame))
	t.Logf("sca terms: self=%.6f, same=%.6f, neg=%.6f",
		cmplx.Abs(scaSelf), cmplx.Abs(scaSame), cmplx.Abs(scaNeg))

	// For a vertical wire, the image vec should equal the self vec (to first order
	// when image distance ≈ self distance).  For a wire at z=1..2, the image is
	// at z=-2..-1, so the distance real→image is about 2m which is larger than the
	// self-term (wire radius ~0.001m).  The image coupling should be much weaker.
	t.Logf("Image coupling (same charge) / self coupling: vec=%.4f, sca=%.4f",
		cmplx.Abs(vecSame)/cmplx.Abs(vecSelf), cmplx.Abs(scaSame)/cmplx.Abs(scaSelf))
}

// TestDiag_CheckScaPrefactor verifies the scalar potential prefactor sign and
// magnitude against the analytic expression.
func TestDiag_CheckScaPrefactor(t *testing.T) {
	freq := 14e6
	omega := 2 * math.Pi * freq
	k := omega / C0

	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	// Expected: scaPrefactor = -j / (4π ε₀ ω), where ε₀ = 1/(μ₀c²)
	epsilon0 := 1.0 / (Mu0 * C0 * C0)
	expectedSca := complex(0, -1.0/(4.0*math.Pi*epsilon0*omega))

	t.Logf("vecPrefactor = %v", vecPrefactor)
	t.Logf("scaPrefactor = %v", scaPrefactor)
	t.Logf("expected sca = %v", expectedSca)
	t.Logf("ratio sca/vec = %v", scaPrefactor/vecPrefactor)

	if cmplx.Abs(scaPrefactor-expectedSca) > 1e-10*cmplx.Abs(expectedSca) {
		t.Errorf("scaPrefactor mismatch: got %v, expected %v", scaPrefactor, expectedSca)
	}
}

