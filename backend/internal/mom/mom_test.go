package mom

import (
	"math"
	"math/cmplx"
	"testing"
)

// ---------------------------------------------------------------------------
// green.go tests
// ---------------------------------------------------------------------------

func TestFreeSpaceGreens(t *testing.T) {
	// G(R) = exp(-jkR) / (4*pi*R)
	// For k=1, R=1: exp(-j) / (4*pi)
	k := 1.0
	R := 1.0
	got := FreeSpaceGreens(k, R)

	expMinusJ := cmplx.Exp(complex(0, -1)) // exp(-j)
	want := expMinusJ / complex(4*math.Pi, 0)

	if cmplx.Abs(got-want) > 1e-10 {
		t.Errorf("FreeSpaceGreens(1,1) = %v, want %v", got, want)
	}

	// Verify the approximate numerical values: exp(-j)/(4*pi)
	// cos(1)/(4*pi) ≈ 0.04300, -sin(1)/(4*pi) ≈ -0.06696
	wantReal := math.Cos(1) / (4 * math.Pi)
	wantImag := -math.Sin(1) / (4 * math.Pi)
	if math.Abs(real(got)-wantReal) > 1e-10 || math.Abs(imag(got)-wantImag) > 1e-10 {
		t.Errorf("FreeSpaceGreens(1,1) ≈ (%.10f, %.10f), want ≈ (%.10f, %.10f)",
			real(got), imag(got), wantReal, wantImag)
	}
}

func TestFreeSpaceGreensRClamping(t *testing.T) {
	// R=0 should be clamped, not produce NaN or Inf
	got := FreeSpaceGreens(1.0, 0.0)
	if cmplx.IsNaN(got) || cmplx.IsInf(got) {
		t.Errorf("FreeSpaceGreens(1,0) = %v, expected finite clamped value", got)
	}
}

func TestFreeSpaceGreensLargeR(t *testing.T) {
	// At large R the magnitude should decay as 1/(4*pi*R)
	k := 2 * math.Pi // k = 2*pi
	R := 100.0
	got := FreeSpaceGreens(k, R)
	gotAbs := cmplx.Abs(got)
	wantAbs := 1.0 / (4 * math.Pi * R)
	if math.Abs(gotAbs-wantAbs)/wantAbs > 1e-10 {
		t.Errorf("|FreeSpaceGreens| = %e, want %e", gotAbs, wantAbs)
	}
}

func TestPsi(t *testing.T) {
	// psi(k, R) = exp(-jkR) / R
	k := 2.0
	R := 3.0
	got := psi(k, R)
	want := cmplx.Exp(complex(0, -k*R)) / complex(R, 0)
	if cmplx.Abs(got-want) > 1e-12 {
		t.Errorf("psi(%v,%v) = %v, want %v", k, R, got, want)
	}
}

func TestPsiRClamping(t *testing.T) {
	got := psi(1.0, 0.0)
	if cmplx.IsNaN(got) || cmplx.IsInf(got) {
		t.Errorf("psi(1,0) = %v, expected finite clamped value", got)
	}
}

func TestPsiRelationToGreens(t *testing.T) {
	// FreeSpaceGreens(k,R) = psi(k,R) / (4*pi)
	k := 5.0
	R := 0.7
	g := FreeSpaceGreens(k, R)
	p := psi(k, R)
	want := p / complex(4*math.Pi, 0)
	if cmplx.Abs(g-want) > 1e-12 {
		t.Errorf("FreeSpaceGreens != psi/(4pi): G=%v, psi/(4pi)=%v", g, want)
	}
}

func TestDist(t *testing.T) {
	a := [3]float64{1, 2, 3}
	b := [3]float64{4, 6, 3}
	// Distance = sqrt(9+16+0) = 5
	got := dist(a, b, false, 0)
	if math.Abs(got-5.0) > 1e-10 {
		t.Errorf("dist without reduced kernel = %v, want 5", got)
	}
}

func TestDistReducedKernel(t *testing.T) {
	a := [3]float64{0, 0, 0}
	b := [3]float64{3, 4, 0}
	radius := 2.0
	// R = sqrt(9+16+0 + 4) = sqrt(29)
	got := dist(a, b, true, radius)
	want := math.Sqrt(29)
	if math.Abs(got-want) > 1e-10 {
		t.Errorf("dist with reduced kernel = %v, want %v", got, want)
	}
}

func TestDistSamePoint(t *testing.T) {
	a := [3]float64{1, 2, 3}
	// Without reduced kernel, same point should get clamped (not zero)
	got := dist(a, a, false, 0)
	if got <= 0 || math.IsNaN(got) || math.IsInf(got, 0) {
		t.Errorf("dist(a,a) = %v, expected small positive clamped value", got)
	}
}

func TestDistSamePointReduced(t *testing.T) {
	a := [3]float64{0, 0, 0}
	radius := 0.005
	got := dist(a, a, true, radius)
	if math.Abs(got-radius) > 1e-10 {
		t.Errorf("dist(a,a, reduced, r=%v) = %v, want %v", radius, got, radius)
	}
}

func TestTriangleKernelSelfTerm(t *testing.T) {
	// Create two adjacent z-directed segments and a basis function spanning them.
	seg0 := Segment{
		Index: 0, WireIndex: 0,
		Center:     [3]float64{0, 0, -0.01},
		Start:      [3]float64{0, 0, -0.02},
		End:        [3]float64{0, 0, 0},
		HalfLength: 0.01,
		Direction:  [3]float64{0, 0, 1},
		Radius:     0.001,
	}
	seg1 := Segment{
		Index: 1, WireIndex: 0,
		Center:     [3]float64{0, 0, 0.01},
		Start:      [3]float64{0, 0, 0},
		End:        [3]float64{0, 0, 0.02},
		HalfLength: 0.01,
		Direction:  [3]float64{0, 0, 1},
		Radius:     0.001,
	}

	dl := 2 * seg0.HalfLength
	basis := TriangleBasis{
		NodeIndex:       0,
		NodePos:         [3]float64{0, 0, 0},
		SegLeft:         &seg0,
		SegRight:        &seg1,
		ChargeDensLeft:  -1.0 / dl,
		ChargeDensRight: 1.0 / dl,
	}

	k := 2 * math.Pi / 1.0 // k for 1 m wavelength
	omega := k * C0
	segs := []Segment{seg0, seg1}

	vecTerm, scaTerm := TriangleKernel(basis, basis, k, omega, segs)

	// Both terms should be non-zero and finite
	if cmplx.IsNaN(vecTerm) || cmplx.IsInf(vecTerm) {
		t.Errorf("TriangleKernel self-term vectorTerm is NaN/Inf: %v", vecTerm)
	}
	if cmplx.IsNaN(scaTerm) || cmplx.IsInf(scaTerm) {
		t.Errorf("TriangleKernel self-term scalarTerm is NaN/Inf: %v", scaTerm)
	}
	if cmplx.Abs(vecTerm) < 1e-30 {
		t.Errorf("TriangleKernel self-term vectorTerm is effectively zero: %v", vecTerm)
	}
	if cmplx.Abs(scaTerm) < 1e-30 {
		t.Errorf("TriangleKernel self-term scalarTerm is effectively zero: %v", scaTerm)
	}
}

func TestTriangleKernelMutualTerm(t *testing.T) {
	// Two adjacent segments along z
	seg0 := Segment{
		Index: 0, WireIndex: 0,
		Center: [3]float64{0, 0, -0.01}, Start: [3]float64{0, 0, -0.02}, End: [3]float64{0, 0, 0},
		HalfLength: 0.01, Direction: [3]float64{0, 0, 1}, Radius: 0.001,
	}
	seg1 := Segment{
		Index: 1, WireIndex: 0,
		Center: [3]float64{0, 0, 0.01}, Start: [3]float64{0, 0, 0}, End: [3]float64{0, 0, 0.02},
		HalfLength: 0.01, Direction: [3]float64{0, 0, 1}, Radius: 0.001,
	}

	dl := 2 * seg0.HalfLength
	basisM := TriangleBasis{
		NodeIndex: 0, NodePos: [3]float64{0, 0, 0},
		SegLeft: &seg0, SegRight: &seg1,
		ChargeDensLeft: -1.0 / dl, ChargeDensRight: 1.0 / dl,
	}
	basisN := TriangleBasis{
		NodeIndex: 0, NodePos: [3]float64{0, 0, 0},
		SegLeft: &seg0, SegRight: &seg1,
		ChargeDensLeft: -1.0 / dl, ChargeDensRight: 1.0 / dl,
	}

	k := 2 * math.Pi / 1.0
	omega := k * C0
	segs := []Segment{seg0, seg1}

	vecTerm, scaTerm := TriangleKernel(basisM, basisN, k, omega, segs)

	if cmplx.IsNaN(vecTerm) || cmplx.IsInf(vecTerm) {
		t.Errorf("Mutual vectorTerm NaN/Inf: %v", vecTerm)
	}
	if cmplx.IsNaN(scaTerm) || cmplx.IsInf(scaTerm) {
		t.Errorf("Mutual scalarTerm NaN/Inf: %v", scaTerm)
	}
}

// ---------------------------------------------------------------------------
// farfield.go tests
// ---------------------------------------------------------------------------

func TestComputeFarFieldShortDipole(t *testing.T) {
	// A single z-directed segment with uniform current = short dipole.
	// Pattern should be proportional to sin^2(theta).
	seg := Segment{
		Index: 0, WireIndex: 0,
		Center:     [3]float64{0, 0, 0},
		Start:      [3]float64{0, 0, -0.005},
		End:        [3]float64{0, 0, 0.005},
		HalfLength: 0.005,
		Direction:  [3]float64{0, 0, 1},
		Radius:     0.001,
	}
	segs := []Segment{seg}
	currents := []complex128{complex(1, 0)}
	k := 2 * math.Pi / 1.0

	pattern, gainDBi := ComputeFarField(segs, currents, k)

	// Check that pattern is not empty
	if len(pattern) == 0 {
		t.Fatal("ComputeFarField returned empty pattern")
	}

	// Find gain at theta=0 (should be null, -100 dB)
	// Find gain at theta=90 (should be maximum)
	var gainAtPole, gainAt90 float64
	foundPole, found90 := false, false
	for _, p := range pattern {
		if p.ThetaDeg == 0 && p.PhiDeg == 0 {
			gainAtPole = p.GainDB
			foundPole = true
		}
		if p.ThetaDeg == 90 && p.PhiDeg == 0 {
			gainAt90 = p.GainDB
			found90 = true
		}
	}

	if !foundPole || !found90 {
		t.Fatal("Could not find pattern points at theta=0 and theta=90")
	}

	// At theta=0, gain should be very low (null)
	if gainAtPole > -30 {
		t.Errorf("Gain at theta=0 = %.2f dB, expected < -30 dB (null on axis)", gainAtPole)
	}

	// At theta=90, gain should be maximum and close to the peak
	// For a short dipole, peak directivity = 1.5 = 1.76 dBi
	if math.Abs(gainAt90-gainDBi) > 0.5 {
		t.Errorf("Gain at theta=90 = %.2f dB, peak = %.2f dB; expected them to be close", gainAt90, gainDBi)
	}

	// Short dipole gain should be approximately 1.76 dBi (allow 10% = ~0.2 dB margin,
	// but numerical integration with 2-degree steps can have more error, so allow wider)
	if math.Abs(gainDBi-1.76) > 0.5 {
		t.Errorf("Peak gain = %.2f dBi, expected ~1.76 dBi for short dipole", gainDBi)
	}

	// At theta=180 (other pole), should also be null
	for _, p := range pattern {
		if p.ThetaDeg == 180 && p.PhiDeg == 0 {
			if p.GainDB > -30 {
				t.Errorf("Gain at theta=180 = %.2f dB, expected < -30 dB", p.GainDB)
			}
			break
		}
	}

	// Check sin^2 pattern shape: gain at theta=60 should be ~sin^2(60) = 0.75 of peak
	// In dB: 10*log10(0.75) = -1.25 dB relative to peak
	for _, p := range pattern {
		if p.ThetaDeg == 60 && p.PhiDeg == 0 {
			relativeDB := p.GainDB - gainDBi
			expectedRelDB := 10 * math.Log10(math.Pow(math.Sin(60*math.Pi/180), 2))
			if math.Abs(relativeDB-expectedRelDB) > 0.5 {
				t.Errorf("Relative gain at theta=60 = %.2f dB, expected ~%.2f dB",
					relativeDB, expectedRelDB)
			}
			break
		}
	}
}

func TestComputeFarFieldWithGroundBelowHorizon(t *testing.T) {
	// Pattern below ground (theta > 90) should be -100 dB.
	seg := Segment{
		Index: 0, WireIndex: 0,
		Center:     [3]float64{0, 0, 0.25},
		Start:      [3]float64{0, 0, 0.24},
		End:        [3]float64{0, 0, 0.26},
		HalfLength: 0.01,
		Direction:  [3]float64{0, 0, 1},
		Radius:     0.001,
	}
	realSegs := []Segment{seg}
	imageSegs := ApplyPerfectGround(realSegs)
	currents := []complex128{complex(1, 0)}
	k := 2 * math.Pi / 1.0

	pattern, _ := ComputeFarFieldWithGround(realSegs, imageSegs, currents, k)

	for _, p := range pattern {
		if p.ThetaDeg > 90 {
			if p.GainDB != -100.0 {
				t.Errorf("Below-ground gain at theta=%.0f should be -100 dB, got %.2f dB",
					p.ThetaDeg, p.GainDB)
				break
			}
		}
	}
}

func TestComputeFarFieldWithGroundHigherGain(t *testing.T) {
	// A vertical monopole over perfect ground should have higher gain than the
	// same wire in free space, due to ground-plane doubling effect.
	seg := Segment{
		Index: 0, WireIndex: 0,
		Center:     [3]float64{0, 0, 0.125},
		Start:      [3]float64{0, 0, 0.0},
		End:        [3]float64{0, 0, 0.25},
		HalfLength: 0.125,
		Direction:  [3]float64{0, 0, 1},
		Radius:     0.001,
	}
	realSegs := []Segment{seg}
	imageSegs := ApplyPerfectGround(realSegs)
	currents := []complex128{complex(1, 0)}
	k := 2 * math.Pi / 1.0

	_, gainFreeSpace := ComputeFarField(realSegs, currents, k)
	_, gainWithGround := ComputeFarFieldWithGround(realSegs, imageSegs, currents, k)

	if gainWithGround <= gainFreeSpace {
		t.Errorf("Ground-plane gain %.2f dBi should exceed free-space gain %.2f dBi",
			gainWithGround, gainFreeSpace)
	}
}

// ---------------------------------------------------------------------------
// ground_image.go tests
// ---------------------------------------------------------------------------

func TestApplyPerfectGroundVertical(t *testing.T) {
	// A vertical segment at (0, 0, z=0.5) pointing in +z direction
	seg := Segment{
		Index: 0, WireIndex: 0,
		Center:     [3]float64{0, 0, 0.5},
		Start:      [3]float64{0, 0, 0.4},
		End:        [3]float64{0, 0, 0.6},
		HalfLength: 0.1,
		Direction:  [3]float64{0, 0, 1},
		Radius:     0.001,
	}

	images := ApplyPerfectGround([]Segment{seg})
	if len(images) != 1 {
		t.Fatalf("Expected 1 image segment, got %d", len(images))
	}
	img := images[0]

	// z-coordinates should be negated
	if math.Abs(img.Center[2]-(-0.5)) > 1e-10 {
		t.Errorf("Image center z = %v, want -0.5", img.Center[2])
	}
	if math.Abs(img.Start[2]-(-0.4)) > 1e-10 {
		t.Errorf("Image start z = %v, want -0.4", img.Start[2])
	}
	if math.Abs(img.End[2]-(-0.6)) > 1e-10 {
		t.Errorf("Image end z = %v, want -0.6", img.End[2])
	}

	// x, y coordinates preserved
	if img.Center[0] != 0 || img.Center[1] != 0 {
		t.Errorf("Image center x,y should be 0,0, got %v,%v", img.Center[0], img.Center[1])
	}

	// Direction: for vertical wire, dir=(0,0,1) -> image dir=(0,0,1) (z preserved)
	if math.Abs(img.Direction[2]-1.0) > 1e-10 {
		t.Errorf("Image direction z = %v, want 1.0 (preserved)", img.Direction[2])
	}
	if math.Abs(img.Direction[0]) > 1e-10 || math.Abs(img.Direction[1]) > 1e-10 {
		t.Errorf("Image direction x,y should be 0, got %v,%v", img.Direction[0], img.Direction[1])
	}

	// HalfLength and radius preserved
	if math.Abs(img.HalfLength-seg.HalfLength) > 1e-10 {
		t.Errorf("Image HalfLength = %v, want %v", img.HalfLength, seg.HalfLength)
	}
	if math.Abs(img.Radius-seg.Radius) > 1e-10 {
		t.Errorf("Image Radius = %v, want %v", img.Radius, seg.Radius)
	}
}

func TestApplyPerfectGroundHorizontal(t *testing.T) {
	// A horizontal segment at z=1, directed in x
	seg := Segment{
		Index: 0, WireIndex: 0,
		Center:     [3]float64{2, 3, 1},
		Start:      [3]float64{1.5, 3, 1},
		End:        [3]float64{2.5, 3, 1},
		HalfLength: 0.5,
		Direction:  [3]float64{1, 0, 0},
		Radius:     0.002,
	}

	images := ApplyPerfectGround([]Segment{seg})
	img := images[0]

	// z negated
	if math.Abs(img.Center[2]-(-1)) > 1e-10 {
		t.Errorf("Image center z = %v, want -1", img.Center[2])
	}

	// x, y preserved
	if math.Abs(img.Center[0]-2) > 1e-10 || math.Abs(img.Center[1]-3) > 1e-10 {
		t.Errorf("Image center x,y = %v,%v, want 2,3", img.Center[0], img.Center[1])
	}

	// Direction: horizontal -> x negated, y negated, z preserved
	// dir=(1,0,0) -> (-1,0,0)
	if math.Abs(img.Direction[0]-(-1)) > 1e-10 {
		t.Errorf("Image direction x = %v, want -1", img.Direction[0])
	}
	if math.Abs(img.Direction[1]) > 1e-10 {
		t.Errorf("Image direction y = %v, want 0", img.Direction[1])
	}
	if math.Abs(img.Direction[2]) > 1e-10 {
		t.Errorf("Image direction z = %v, want 0", img.Direction[2])
	}
}

func TestApplyPerfectGroundDiagonal(t *testing.T) {
	// A 45-degree segment with direction (0, 1/sqrt2, 1/sqrt2)
	s := 1.0 / math.Sqrt(2)
	seg := Segment{
		Index: 0, WireIndex: 0,
		Center:     [3]float64{0, 0, 1},
		Start:      [3]float64{0, -0.1, 0.9},
		End:        [3]float64{0, 0.1, 1.1},
		HalfLength: 0.1 * math.Sqrt(2),
		Direction:  [3]float64{0, s, s},
		Radius:     0.001,
	}

	images := ApplyPerfectGround([]Segment{seg})
	img := images[0]

	// Direction: y negated, z preserved
	if math.Abs(img.Direction[1]-(-s)) > 1e-10 {
		t.Errorf("Image direction y = %v, want %v", img.Direction[1], -s)
	}
	if math.Abs(img.Direction[2]-s) > 1e-10 {
		t.Errorf("Image direction z = %v, want %v", img.Direction[2], s)
	}
}

// ---------------------------------------------------------------------------
// segment.go additional tests
// ---------------------------------------------------------------------------

func TestSubdivideWireOneSegment(t *testing.T) {
	segs := SubdivideWire(0, 0, 0, 0, 0, 0, 1, 0.001, 1)
	if len(segs) != 1 {
		t.Fatalf("Expected 1 segment, got %d", len(segs))
	}
	if math.Abs(segs[0].HalfLength-0.5) > 1e-10 {
		t.Errorf("HalfLength = %v, want 0.5", segs[0].HalfLength)
	}
	if math.Abs(segs[0].Center[2]-0.5) > 1e-10 {
		t.Errorf("Center z = %v, want 0.5", segs[0].Center[2])
	}
}

func TestSubdivideWireZeroLength(t *testing.T) {
	segs := SubdivideWire(0, 5, 5, 5, 5, 5, 5, 0.001, 10)
	if segs != nil {
		t.Errorf("Zero-length wire should return nil, got %d segments", len(segs))
	}
}

func TestSubdivideWireDirectionsAreUnitVectors(t *testing.T) {
	// Diagonal wire
	segs := SubdivideWire(0, 1, 2, 3, 4, 6, 3, 0.001, 7)
	if segs == nil {
		t.Fatal("SubdivideWire returned nil for non-zero-length wire")
	}
	for i, seg := range segs {
		mag := math.Sqrt(seg.Direction[0]*seg.Direction[0] +
			seg.Direction[1]*seg.Direction[1] +
			seg.Direction[2]*seg.Direction[2])
		if math.Abs(mag-1.0) > 1e-10 {
			t.Errorf("Segment %d direction magnitude = %v, want 1.0", i, mag)
		}
	}
}

func TestSubdivideWireLengthsSum(t *testing.T) {
	x1, y1, z1 := 1.0, 2.0, 3.0
	x2, y2, z2 := 4.0, 6.0, 8.0
	n := 13
	segs := SubdivideWire(0, x1, y1, z1, x2, y2, z2, 0.001, n)
	if len(segs) != n {
		t.Fatalf("Expected %d segments, got %d", n, len(segs))
	}

	totalLength := 0.0
	for _, seg := range segs {
		totalLength += 2 * seg.HalfLength
	}

	dx := x2 - x1
	dy := y2 - y1
	dz := z2 - z1
	wireLength := math.Sqrt(dx*dx + dy*dy + dz*dz)

	if math.Abs(totalLength-wireLength) > 1e-10 {
		t.Errorf("Total segment length = %v, wire length = %v", totalLength, wireLength)
	}
}

func TestSubdivideWireSegmentsContinuous(t *testing.T) {
	// Verify that segment i's End matches segment i+1's Start
	segs := SubdivideWire(0, 0, 0, 0, 1, 1, 1, 0.001, 5)
	for i := 0; i < len(segs)-1; i++ {
		for d := 0; d < 3; d++ {
			if math.Abs(segs[i].End[d]-segs[i+1].Start[d]) > 1e-10 {
				t.Errorf("Segment %d end[%d]=%v != segment %d start[%d]=%v",
					i, d, segs[i].End[d], i+1, d, segs[i+1].Start[d])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// quadrature.go additional tests
// ---------------------------------------------------------------------------

func TestGaussLegendreX4(t *testing.T) {
	// Integral of x^4 over [-1,1] = 2/5 = 0.4
	// A 3-point rule integrates up to degree 2*3-1=5 exactly, so x^4 should be exact.
	for _, n := range []int{3, 5, 8} {
		nodes, weights := GaussLegendre(n)
		sum := 0.0
		for i, x := range nodes {
			sum += weights[i] * x * x * x * x
		}
		expected := 2.0 / 5.0
		if math.Abs(sum-expected) > 1e-12 {
			t.Errorf("GaussLegendre(%d): integral of x^4 = %.15f, expected %.15f", n, sum, expected)
		}
	}
}

func TestGaussLegendreSymmetry(t *testing.T) {
	n := 7
	nodes, weights := GaussLegendre(n)

	// Nodes should be symmetric: node[i] = -node[n-1-i]
	for i := 0; i < n/2; i++ {
		if math.Abs(nodes[i]+nodes[n-1-i]) > 1e-14 {
			t.Errorf("Nodes not symmetric: nodes[%d]=%v, nodes[%d]=%v",
				i, nodes[i], n-1-i, nodes[n-1-i])
		}
	}

	// Weights should be symmetric: weights[i] = weights[n-1-i]
	for i := 0; i < n/2; i++ {
		if math.Abs(weights[i]-weights[n-1-i]) > 1e-14 {
			t.Errorf("Weights not symmetric: weights[%d]=%v, weights[%d]=%v",
				i, weights[i], n-1-i, weights[n-1-i])
		}
	}
}

func TestGaussLegendreCaching(t *testing.T) {
	// Calling GaussLegendre twice with the same n should return the same slices
	n := 17
	nodes1, weights1 := GaussLegendre(n)
	nodes2, weights2 := GaussLegendre(n)

	// Check pointer equality (same backing array)
	if &nodes1[0] != &nodes2[0] {
		t.Error("GaussLegendre nodes not cached: different slice backing arrays")
	}
	if &weights1[0] != &weights2[0] {
		t.Error("GaussLegendre weights not cached: different slice backing arrays")
	}
}

func TestGaussLegendreOddPolynomial(t *testing.T) {
	// Integral of x^3 over [-1,1] = 0 (odd function)
	nodes, weights := GaussLegendre(4)
	sum := 0.0
	for i, x := range nodes {
		sum += weights[i] * x * x * x
	}
	if math.Abs(sum) > 1e-14 {
		t.Errorf("Integral of x^3 = %v, expected 0", sum)
	}
}
