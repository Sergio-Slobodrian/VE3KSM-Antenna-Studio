package mom

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

// ApplyPerfectGround returns image segments mirrored across the z=0 ground plane
// for a perfect electric conductor (PEC) ground.
//
// Image theory for PEC ground:
//   - A vertical (z-directed) current at (x,y,z) has an image at (x,y,-z) with the
//     SAME current direction (z-component preserved).
//   - A horizontal current at (x,y,z) has an image at (x,y,-z) with OPPOSITE
//     horizontal direction (x,y components negated).
//
// Therefore the image segment direction has: dir_x -> -dir_x, dir_y -> -dir_y, dir_z -> +dir_z
func ApplyPerfectGround(segments []Segment) []Segment {
	images := make([]Segment, len(segments))
	for i, seg := range segments {
		images[i] = Segment{
			Index:     seg.Index,
			WireIndex: seg.WireIndex,
			Center: [3]float64{
				seg.Center[0],
				seg.Center[1],
				-seg.Center[2],
			},
			Start: [3]float64{
				seg.Start[0],
				seg.Start[1],
				-seg.Start[2],
			},
			End: [3]float64{
				seg.End[0],
				seg.End[1],
				-seg.End[2],
			},
			HalfLength: seg.HalfLength,
			Direction: [3]float64{
				-seg.Direction[0],
				-seg.Direction[1],
				seg.Direction[2],
			},
			Radius: seg.Radius,
		}
	}
	return images
}

// AddGroundContributions adds the mutual impedance contributions from image segments
// to the Z-matrix of the real segments. This implements PEC image theory:
// each Z[i][j] gains an additional term from the coupling between real segment i
// and the image of segment j.
//
// The Z-matrix remains N x N (for N real segments).
func AddGroundContributions(Z *mat.CDense, realSegs, imageSegs []Segment, k, omega float64) {
	n := len(realSegs)
	if len(imageSegs) != n {
		return
	}

	prefactor := complex(0, omega*Mu0/(4.0*math.Pi))

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			// Mutual impedance between real segment i and image segment j.
			// Image segments are never coincident with real segments (different z),
			// so reduced kernel is not needed.
			kernel := PocklingtonKernel(k, realSegs[i], imageSegs[j], false)
			contribution := prefactor * kernel

			existing := Z.At(i, j)
			Z.Set(i, j, existing+contribution)
		}
	}
}
