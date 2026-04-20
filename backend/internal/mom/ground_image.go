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

	"gonum.org/v1/gonum/mat"
)

// ApplyPerfectGround constructs image segments for a perfect electric conductor
// (PEC) ground plane located at z=0, using electromagnetic image theory.
//
// Image theory replaces the ground plane boundary condition with equivalent
// image sources below ground, allowing the problem to be solved as if in
// free space (with twice as many sources). For a PEC ground plane:
//
//   - Vertical currents (z-directed) at position (x,y,z) produce an image at
//     (x,y,-z) with the SAME current direction. This is because the tangential
//     E-field must vanish at the PEC surface, and a same-direction vertical
//     image current cancels the tangential E-field at z=0.
//
//   - Horizontal currents (x or y directed) at (x,y,z) produce an image at
//     (x,y,-z) with OPPOSITE current direction. The reversed horizontal image
//     current cancels the tangential E-field at the ground surface.
//
// The net effect on the direction vector is: (dx, dy, dz) -> (-dx, -dy, +dz).
// The returned image segments preserve the original Index and WireIndex so that
// they can be associated with the same basis functions and currents as the
// real segments.
func ApplyPerfectGround(segments []Segment) []Segment {
	images := make([]Segment, len(segments))
	for i, seg := range segments {
		images[i] = Segment{
			Index:     seg.Index,     // same index for basis function mapping
			WireIndex: seg.WireIndex, // same wire association
			Center: [3]float64{       // mirror z coordinate
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
			HalfLength: seg.HalfLength, // length is preserved by reflection
			Direction: [3]float64{       // PEC image rule: negate horizontal, keep vertical
				-seg.Direction[0],
				-seg.Direction[1],
				seg.Direction[2],
			},
			Radius: seg.Radius,
		}
	}
	return images
}

// AddGroundContributions adds the mutual impedance contributions from image
// segments to the impedance matrix Z, implementing the legacy pulse-basis
// PEC ground plane model.
//
// For each matrix entry Z[i][j], the coupling between real segment i and the
// image of segment j is computed and added to the existing value. This
// effectively accounts for the ground-reflected field without increasing the
// matrix dimensions. The image segments are never coincident with real segments
// (they are at -z vs +z), so the reduced kernel (self-term regularization) is
// not needed.
//
// NOTE: This function uses the legacy PocklingtonKernel (now a stub returning
// zero) and is retained for interface compatibility. The active ground plane
// implementation for triangle basis functions is addGroundTriangleBasis in
// solver.go.
func AddGroundContributions(Z *mat.CDense, realSegs, imageSegs []Segment, k, omega float64) {
	n := len(realSegs)
	if len(imageSegs) != n {
		return
	}

	// EFIE prefactor: jωμ₀/(4π)
	prefactor := complex(0, omega*Mu0/(4.0*math.Pi))

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			// Compute mutual coupling between real segment i and image of segment j.
			// reduced=false because image segments are always spatially separated
			// from the real segments (different z coordinates).
			kernel := PocklingtonKernel(k, realSegs[i], imageSegs[j], false)
			contribution := prefactor * kernel

			existing := Z.At(i, j)
			Z.Set(i, j, existing+contribution)
		}
	}
}
