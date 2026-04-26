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

package nec2

import "antenna-studio/backend/internal/mom"

// FromInput projects a mom.SimulationInput into the JSON-friendly form
// the writer accepts.  Material names are translated to their bulk
// conductivity values via the MoM material library.
func FromInput(in mom.SimulationInput) GeometryWriteInput {
	out := GeometryWriteInput{
		GroundType:     in.Ground.Type,
		Conductivity:   in.Ground.Conductivity,
		Permittivity:   in.Ground.Permittivity,
		MoisturePreset: in.Ground.MoisturePreset,
		RegionPreset:   in.Ground.RegionPreset,
		Source: SourceRow{
			WireIndex:    in.Source.WireIndex,
			SegmentIndex: in.Source.SegmentIndex,
			Voltage:      in.Source.Voltage,
		},
		Weather: WeatherRow{
			Preset:    in.Weather.Preset,
			Thickness: in.Weather.Thickness,
			EpsR:      in.Weather.EpsR,
			LossTan:   in.Weather.LossTan,
		},
	}

	for _, w := range in.Wires {
		row := WireRow{
			X1: w.X1, Y1: w.Y1, Z1: w.Z1,
			X2: w.X2, Y2: w.Y2, Z2: w.Z2,
			Radius: w.Radius, Segments: w.Segments,
			RadiusStart:      w.RadiusStart,
			RadiusEnd:        w.RadiusEnd,
			CoatingThickness: w.CoatingThickness,
			CoatingEpsR:      w.CoatingEpsR,
			CoatingLossTan:   w.CoatingLossTan,
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
