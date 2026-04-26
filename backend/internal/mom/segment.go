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

import "math"

// Segment represents a single discretized wire segment for the Method of Moments.
// Each segment is a short straight piece of wire characterized by its center point,
// endpoints, half-length, orientation (unit direction vector), and wire radius.
// The MoM solver uses these segments as the domain for basis function expansion.
type Segment struct {
	Index      int        // global index across all wires (used for matrix addressing)
	WireIndex  int        // index of the parent wire this segment belongs to
	Center     [3]float64 // midpoint of the segment (m) — used as the collocation point
	Start      [3]float64 // start endpoint of the segment (m)
	End        [3]float64 // end endpoint of the segment (m)
	HalfLength float64    // half the segment length (m), i.e. Δl/2
	Direction  [3]float64 // unit vector along the segment from Start to End
	Radius     float64    // wire cross-section radius (m) — used in the thin-wire kernel
}

// SubdivideWire divides a straight wire into numSegments equal-length segments.
// This is the geometry discretization step of MoM: the continuous wire is replaced
// by a chain of short segments on which basis functions will be defined.
//
// Parameters:
//   - wireIndex: index of this wire in the input wire array
//   - (x1,y1,z1), (x2,y2,z2): wire endpoints in Cartesian coordinates (m)
//   - radiusStart: wire radius at the (x1,y1,z1) endpoint (m)
//   - radiusEnd:   wire radius at the (x2,y2,z2) endpoint (m). For a uniform
//     wire, pass radiusStart == radiusEnd. For a linearly tapered wire each
//     segment's Radius is interpolated at the segment center.
//   - numSegments: number of subdivisions (clamped to minimum 1)
//
// Returns nil if the wire has zero length. The returned segments have local
// indices (0..N-1); the caller is responsible for assigning global indices.
func SubdivideWire(wireIndex int, x1, y1, z1, x2, y2, z2, radiusStart, radiusEnd float64, numSegments int) []Segment {
	if numSegments < 1 {
		numSegments = 1
	}

	// Wire vector from start to end
	dx := x2 - x1
	dy := y2 - y1
	dz := z2 - z1
	wireLength := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if wireLength < 1e-15 {
		return nil // degenerate zero-length wire
	}

	// Each segment has length wireLength/numSegments; store half for quadrature scaling
	halfLen := wireLength / (2.0 * float64(numSegments))

	// Unit direction vector along the wire (shared by all segments of this wire)
	dir := [3]float64{dx / wireLength, dy / wireLength, dz / wireLength}

	segments := make([]Segment, numSegments)
	for i := 0; i < numSegments; i++ {
		// Parametric positions along the wire: t in [0, 1]
		tCenter := (float64(i) + 0.5) / float64(numSegments)
		tStart := float64(i) / float64(numSegments)
		tEnd := float64(i+1) / float64(numSegments)

		// Linear radius interpolation at the segment center. When start==end
		// this reduces to a bit-identical constant radius (same arithmetic as
		// the pre-taper code path).
		segRadius := radiusStart + tCenter*(radiusEnd-radiusStart)

		segments[i] = Segment{
			Index:     i,
			WireIndex: wireIndex,
			Center: [3]float64{
				x1 + tCenter*dx,
				y1 + tCenter*dy,
				z1 + tCenter*dz,
			},
			Start: [3]float64{
				x1 + tStart*dx,
				y1 + tStart*dy,
				z1 + tStart*dz,
			},
			End: [3]float64{
				x1 + tEnd*dx,
				y1 + tEnd*dy,
				z1 + tEnd*dz,
			},
			HalfLength: halfLen,
			Direction:  dir,
			Radius:     segRadius,
		}
	}
	return segments
}
