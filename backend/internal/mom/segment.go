package mom

import "math"

// Segment represents a discretized wire segment for the Method of Moments.
type Segment struct {
	Index      int
	WireIndex  int
	Center     [3]float64
	Start      [3]float64
	End        [3]float64
	HalfLength float64
	Direction  [3]float64
	Radius     float64
}

// SubdivideWire divides a straight wire into numSegments equal-length segments.
// wireIndex identifies which wire this belongs to.
// (x1,y1,z1) and (x2,y2,z2) are the wire endpoints.
// radius is the wire radius. numSegments is the number of subdivisions.
func SubdivideWire(wireIndex int, x1, y1, z1, x2, y2, z2 float64, radius float64, numSegments int) []Segment {
	if numSegments < 1 {
		numSegments = 1
	}

	dx := x2 - x1
	dy := y2 - y1
	dz := z2 - z1
	wireLength := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if wireLength < 1e-15 {
		return nil
	}

	halfLen := wireLength / (2.0 * float64(numSegments))

	// Unit direction vector along the wire
	dir := [3]float64{dx / wireLength, dy / wireLength, dz / wireLength}

	segments := make([]Segment, numSegments)
	for i := 0; i < numSegments; i++ {
		tCenter := (float64(i) + 0.5) / float64(numSegments)
		tStart := float64(i) / float64(numSegments)
		tEnd := float64(i+1) / float64(numSegments)

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
			Radius:     radius,
		}
	}
	return segments
}
