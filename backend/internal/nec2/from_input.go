package nec2

import "antenna-studio/backend/internal/mom"

// FromInput projects a mom.SimulationInput into the JSON-friendly form
// the writer accepts.  Material names are translated to their bulk
// conductivity values via the MoM material library.
func FromInput(in mom.SimulationInput) GeometryWriteInput {
	out := GeometryWriteInput{
		GroundType:   in.Ground.Type,
		Conductivity: in.Ground.Conductivity,
		Permittivity: in.Ground.Permittivity,
		Source: SourceRow{
			WireIndex:    in.Source.WireIndex,
			SegmentIndex: in.Source.SegmentIndex,
			Voltage:      in.Source.Voltage,
		},
	}

	for _, w := range in.Wires {
		row := WireRow{
			X1: w.X1, Y1: w.Y1, Z1: w.Z1,
			X2: w.X2, Y2: w.Y2, Z2: w.Z2,
			Radius: w.Radius, Segments: w.Segments,
		}
		if w.Material != "" && w.Material != mom.MaterialPEC {
			if m, ok := mom.LookupMaterial(w.Material); ok {
				row.Sigma = m.Sigma
			}
		}
		out.Wires = append(out.Wires, row)
	}

	for _, ld := range in.Loads {
		out.Loads = append(out.Loads, LoadRow{
			WireIndex:        ld.WireIndex,
			SegmentIndex:     ld.SegmentIndex,
			ParallelTopology: ld.Topology == mom.LoadParallelRLC,
			R:                ld.R, L: ld.L, C: ld.C,
		})
	}

	for _, tl := range in.TransmissionLines {
		out.TransmissionLines = append(out.TransmissionLines, TLRow{
			AWireIndex: tl.A.WireIndex, ASegmentIndex: tl.A.SegmentIndex,
			BWireIndex: tl.B.WireIndex, BSegmentIndex: tl.B.SegmentIndex,
			Z0: tl.Z0, Length: tl.Length,
		})
	}

	return out
}
