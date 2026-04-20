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

import (
	"fmt"

	"antenna-studio/backend/internal/mom"
)

// Geometry is what Parse + ToGeometry produce: the antenna model plus
// the sweep parameters (when an FR card is present, FreqStartHz /
// FreqEndHz / FreqSteps are populated; otherwise they are zero).
type Geometry struct {
	Input        mom.SimulationInput
	FreqStartHz  float64
	FreqEndHz    float64
	FreqSteps    int
	Comments     []string
	IgnoredCards []string
}

// ToGeometry walks the parsed cards and produces a SimulationInput.
// NEC tags wires; we use 0-based indices, so this maintains a tag->index map.
func ToGeometry(f *File) (Geometry, error) {
	g := Geometry{Comments: append([]string{}, f.Comments...)}

	tagToIdx := map[int]int{}
	scale := 1.0
	groundType := "free_space"
	var groundEpsR, groundSigma float64

	for _, c := range f.Cards {
		switch c.Mnemonic {
		case "CM", "CE":
			// already collected
		case "EN":
			g.Input.Ground = mom.GroundConfig{Type: groundType, Conductivity: groundSigma, Permittivity: groundEpsR}
			return g, nil
		case "GS":
			if s := c.FieldFloat(3, 0); s > 0 {
				scale = s
			} else if s := c.FieldFloat(0, 0); s > 0 {
				scale = s
			}
		case "GW":
			if len(c.Floats) < 9 {
				return g, fmt.Errorf("line %d: GW needs 9 fields, got %d", c.Line, len(c.Floats))
			}
			tag := c.FieldInt(0, 0)
			seg := c.FieldInt(1, 0)
			if seg < 1 {
				return g, fmt.Errorf("line %d: GW segments must be >= 1", c.Line)
			}
			w := mom.Wire{
				X1: c.Floats[2] * scale, Y1: c.Floats[3] * scale, Z1: c.Floats[4] * scale,
				X2: c.Floats[5] * scale, Y2: c.Floats[6] * scale, Z2: c.Floats[7] * scale,
				Radius: c.Floats[8] * scale, Segments: seg,
			}
			g.Input.Wires = append(g.Input.Wires, w)
			tagToIdx[tag] = len(g.Input.Wires) - 1
		case "GE":
			if c.FieldInt(0, 0) == 1 && groundType == "free_space" {
				groundType = "perfect"
			}
		case "GN":
			switch c.FieldInt(0, -1) {
			case -1:
				groundType = "free_space"
			case 0, 1:
				groundType = "perfect"
			case 2:
				groundType = "real"
				groundEpsR = c.FieldFloat(4, 13)
				groundSigma = c.FieldFloat(5, 0.005)
			}
		case "EX":
			if g.Input.Source.Voltage != 0 {
				continue
			}
			t := c.FieldInt(0, 0)
			if t != 0 {
				g.IgnoredCards = append(g.IgnoredCards, fmt.Sprintf("EX type %d (line %d)", t, c.Line))
				continue
			}
			tag := c.FieldInt(1, 0)
			seg := c.FieldInt(2, 1) - 1
			re := c.FieldFloat(4, 1)
			im := c.FieldFloat(5, 0)
			wi, ok := tagToIdx[tag]
			if !ok {
				return g, fmt.Errorf("line %d: EX references unknown tag %d", c.Line, tag)
			}
			g.Input.Source = mom.Source{WireIndex: wi, SegmentIndex: seg, Voltage: complex(re, im)}
		case "LD":
			t := c.FieldInt(0, 0)
			tag := c.FieldInt(1, 0)
			m := c.FieldInt(2, 0)
			if tag == 0 || m == 0 {
				g.IgnoredCards = append(g.IgnoredCards, fmt.Sprintf("LD blanket type %d (line %d)", t, c.Line))
				continue
			}
			wi, ok := tagToIdx[tag]
			if !ok {
				return g, fmt.Errorf("line %d: LD references unknown tag %d", c.Line, tag)
			}
			switch t {
			case 0, 1:
				topo := mom.LoadSeriesRLC
				if t == 1 {
					topo = mom.LoadParallelRLC
				}
				g.Input.Loads = append(g.Input.Loads, mom.Load{
					WireIndex: wi, SegmentIndex: m - 1, Topology: topo,
					R: c.FieldFloat(4, 0), L: c.FieldFloat(5, 0), C: c.FieldFloat(6, 0),
				})
			case 4:
				R := c.FieldFloat(4, 0); X := c.FieldFloat(5, 0)
				if X == 0 {
					g.Input.Loads = append(g.Input.Loads, mom.Load{
						WireIndex: wi, SegmentIndex: m - 1, Topology: mom.LoadSeriesRLC, R: R,
					})
				} else {
					g.IgnoredCards = append(g.IgnoredCards, fmt.Sprintf("LD type 4 with X=%g not supported (line %d)", X, c.Line))
				}
			case 5:
				g.Input.Wires[wi].Material = guessMaterialBySigma(c.FieldFloat(4, 0))
			default:
				g.IgnoredCards = append(g.IgnoredCards, fmt.Sprintf("LD type %d not supported (line %d)", t, c.Line))
			}
		case "TL":
			if len(c.Floats) < 6 {
				return g, fmt.Errorf("line %d: TL needs 6 fields", c.Line)
			}
			tag1 := c.FieldInt(0, 0); seg1 := c.FieldInt(1, 1) - 1
			tag2 := c.FieldInt(2, 0); seg2 := c.FieldInt(3, 1) - 1
			z0 := c.FieldFloat(4, 50); length := c.FieldFloat(5, 0)
			wi1, ok := tagToIdx[tag1]
			if !ok {
				return g, fmt.Errorf("line %d: TL references unknown tag1 %d", c.Line, tag1)
			}
			tl := mom.TransmissionLine{
				A: mom.TLEnd{WireIndex: wi1, SegmentIndex: seg1},
				Z0: z0, Length: length, VelocityFactor: 1.0,
			}
			switch {
			case tag2 == -1:
				tl.B = mom.TLEnd{WireIndex: mom.TLEndShorted}
			case z0 < 0:
				tl.B = mom.TLEnd{WireIndex: mom.TLEndOpen}
				tl.Z0 = -z0
			default:
				wi2, ok := tagToIdx[tag2]
				if !ok {
					return g, fmt.Errorf("line %d: TL references unknown tag2 %d", c.Line, tag2)
				}
				tl.B = mom.TLEnd{WireIndex: wi2, SegmentIndex: seg2}
			}
			g.Input.TransmissionLines = append(g.Input.TransmissionLines, tl)
		case "FR":
			n := c.FieldInt(1, 1)
			fStart := c.FieldFloat(4, 0); fStep := c.FieldFloat(5, 0)
			if n <= 1 {
				g.Input.Frequency = fStart * 1e6
				continue
			}
			g.FreqStartHz = fStart * 1e6
			g.FreqEndHz = (fStart + float64(n-1)*fStep) * 1e6
			g.FreqSteps = n
			g.Input.Frequency = g.FreqStartHz
		default:
			g.IgnoredCards = append(g.IgnoredCards, fmt.Sprintf("%s (line %d)", c.Mnemonic, c.Line))
		}
	}

	g.Input.Ground = mom.GroundConfig{Type: groundType, Conductivity: groundSigma, Permittivity: groundEpsR}
	return g, nil
}

// guessMaterialBySigma maps a conductivity to the closest library entry.
func guessMaterialBySigma(sigma float64) mom.MaterialName {
	if sigma <= 0 {
		return mom.MaterialPEC
	}
	candidates := []mom.MaterialName{
		mom.MaterialCopper, mom.MaterialAluminum, mom.MaterialBrass,
		mom.MaterialSteel, mom.MaterialStainless, mom.MaterialSilver,
		mom.MaterialGold,
	}
	best := mom.MaterialPEC
	bestRel := 1e9
	for _, name := range candidates {
		m, _ := mom.LookupMaterial(name)
		rel := absDiff(m.Sigma, sigma) / sigma
		if rel < bestRel {
			bestRel = rel
			best = name
		}
	}
	if bestRel > 0.5 {
		return mom.MaterialPEC
	}
	return best
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
