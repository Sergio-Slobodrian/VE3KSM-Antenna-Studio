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

package geometry

import (
	"fmt"
	"math"
)

// Template defines a preset antenna geometry generator. Each template has a
// name, human-readable description, a list of user-configurable parameters
// with defaults, and a Generate function that produces wire geometry, source
// placement, and ground config. The Generate function is tagged json:"-"
// because it is not serializable; the GET /api/templates endpoint returns
// only name, description, and parameters.
type Template struct {
	Name        string                                               `json:"name"`
	Description string                                               `json:"description"`
	Parameters  []TemplateParam                                      `json:"parameters"`
	Generate    func(params map[string]float64) (*TemplateResult, error) `json:"-"`
}

// TemplateParam describes a single configurable parameter for a template.
// Name is the parameter key (e.g. "frequency_mhz"), Type is "float" or "int"
// (used by the frontend to render the appropriate input widget), and Default
// is the value used when the parameter is not provided in the request body.
type TemplateParam struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Default float64 `json:"default"`
}

// TemplateResult holds the complete antenna geometry produced by a template.
// The frontend loads this directly into the 3D editor: wires define the
// structure, source specifies the feed point, and ground sets the environment.
type TemplateResult struct {
	Wires  []WireDTO  `json:"wires"`
	Source SourceDTO  `json:"source"`
	Ground GroundDTO  `json:"ground"`
}

// defaultWireRadius is the conductor radius used by all templates (1 mm).
// This is a reasonable value for HF/VHF wire antennas and satisfies the
// thin-wire approximation for typical segment counts.
const defaultWireRadius = 0.001 // 1 mm

// getParam retrieves a named parameter from the user-supplied map, falling
// back to the given default if the key is absent. This lets templates use
// partial parameter overrides.
func getParam(params map[string]float64, name string, def float64) float64 {
	if v, ok := params[name]; ok {
		return v
	}
	return def
}

// GetTemplates returns the full list of available antenna preset templates.
// The order here determines the display order in the frontend template picker.
func GetTemplates() []Template {
	return []Template{
		halfWaveDipole(),
		quarterWaveVertical(),
		threeElementYagi(),
		invertedVDipole(),
		fullWaveLoop(),
	}
}

// halfWaveDipole creates a vertical half-wave dipole template.
// The wire is oriented along the Z axis, centered at the given height.
// Total length = lambda/2, so each arm extends lambda/4 above and below center.
// lambda = 300 / freq_MHz (speed of light approximation for MHz to meters).
// Segments must be odd so the center segment aligns with the feed point.
func halfWaveDipole() Template {
	return Template{
		Name:        "half_wave_dipole",
		Description: "Half-wave dipole: single wire of length lambda/2, center-fed in free space",
		Parameters: []TemplateParam{
			{Name: "frequency_mhz", Type: "float", Default: 146.0},
			{Name: "height", Type: "float", Default: 10.0},
			{Name: "segments", Type: "int", Default: 21},
		},
		Generate: func(params map[string]float64) (*TemplateResult, error) {
			freqMHz := getParam(params, "frequency_mhz", 146.0)
			height := getParam(params, "height", 10.0)
			segments := int(getParam(params, "segments", 21))

			if freqMHz <= 0 {
				return nil, fmt.Errorf("frequency must be positive")
			}
			if segments < 1 {
				segments = 21
			}
			// Force odd segment count so segments/2 lands exactly at center
			if segments%2 == 0 {
				segments++
			}

			lambda := 300.0 / freqMHz   // wavelength in meters
			halfLen := lambda / 4.0      // each arm is lambda/4

			return &TemplateResult{
				Wires: []WireDTO{
					{
						X1: 0, Y1: 0, Z1: height - halfLen,
						X2: 0, Y2: 0, Z2: height + halfLen,
						Radius:   defaultWireRadius,
						Segments: segments,
					},
				},
				Source: SourceDTO{
					WireIndex:    0,
					SegmentIndex: segments / 2, // center feed
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "free_space"},
			}, nil
		},
	}
}

// quarterWaveVertical creates a vertical quarter-wave monopole template.
// The wire starts at Z=0 (ground level) and extends upward by lambda/4.
// It uses a perfect ground plane, which via image theory makes this
// electrically equivalent to a half-wave dipole but with half the structure.
// The source is at segment 0 (base of the wire, at ground level).
func quarterWaveVertical() Template {
	return Template{
		Name:        "quarter_wave_vertical",
		Description: "Quarter-wave vertical monopole: single wire of length lambda/4, base-fed over perfect ground",
		Parameters: []TemplateParam{
			{Name: "frequency_mhz", Type: "float", Default: 146.0},
			{Name: "segments", Type: "int", Default: 21},
		},
		Generate: func(params map[string]float64) (*TemplateResult, error) {
			freqMHz := getParam(params, "frequency_mhz", 146.0)
			segments := int(getParam(params, "segments", 21))

			if freqMHz <= 0 {
				return nil, fmt.Errorf("frequency must be positive")
			}
			if segments < 1 {
				segments = 21
			}

			lambda := 300.0 / freqMHz
			quarterLen := lambda / 4.0

			return &TemplateResult{
				Wires: []WireDTO{
					{
						X1: 0, Y1: 0, Z1: 0,
						X2: 0, Y2: 0, Z2: quarterLen,
						Radius:   defaultWireRadius,
						Segments: segments,
					},
				},
				Source: SourceDTO{
					WireIndex:    0,
					SegmentIndex: 0, // base-fed at ground level
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "perfect"},
			}, nil
		},
	}
}

// threeElementYagi creates a 3-element Yagi-Uda beam antenna template.
// Elements are oriented along the Y axis, spaced along the X axis:
//   - Reflector at X = -0.2*lambda: length 0.51*lambda (slightly longer than
//     the driven element to create a parasitic impedance that reflects energy forward)
//   - Driven element at X = 0: length 0.48*lambda (slightly shorter than lambda/2
//     to present a reasonable feed impedance, ~20-25 ohms)
//   - Director at X = +0.2*lambda: length 0.44*lambda (shorter still, which
//     capacitively loads it to direct radiation forward)
//
// The 0.2*lambda spacing is a classic Yagi design compromise: closer spacing
// gives more gain but narrower bandwidth; wider spacing is the opposite.
// Segments must be odd for center-feed on the driven element (wire index 1).
func threeElementYagi() Template {
	return Template{
		Name:        "3_element_yagi",
		Description: "3-element Yagi-Uda: reflector, driven element, and director with ~0.2 lambda spacing",
		Parameters: []TemplateParam{
			{Name: "frequency_mhz", Type: "float", Default: 146.0},
			{Name: "height", Type: "float", Default: 10.0},
			{Name: "segments", Type: "int", Default: 21},
		},
		Generate: func(params map[string]float64) (*TemplateResult, error) {
			freqMHz := getParam(params, "frequency_mhz", 146.0)
			height := getParam(params, "height", 10.0)
			segments := int(getParam(params, "segments", 21))

			if freqMHz <= 0 {
				return nil, fmt.Errorf("frequency must be positive")
			}
			if segments < 1 {
				segments = 21
			}
			if segments%2 == 0 {
				segments++
			}

			lambda := 300.0 / freqMHz
			spacing := 0.2 * lambda // inter-element spacing

			// Element half-lengths. Each is a fraction of lambda chosen for
			// the parasitic element's role:
			// - 0.51*lambda reflector: slightly longer than resonant to be inductive
			// - 0.48*lambda driven: near-resonant, center-fed
			// - 0.44*lambda director: shorter than resonant to be capacitive
			reflectorHalf := 0.51 * lambda / 2.0
			drivenHalf := 0.48 * lambda / 2.0
			directorHalf := 0.44 * lambda / 2.0

			return &TemplateResult{
				Wires: []WireDTO{
					// Wire 0: Reflector (behind the driven element)
					{
						X1: -spacing, Y1: -reflectorHalf, Z1: height,
						X2: -spacing, Y2: reflectorHalf, Z2: height,
						Radius: defaultWireRadius, Segments: segments,
					},
					// Wire 1: Driven element (center, fed at midpoint)
					{
						X1: 0, Y1: -drivenHalf, Z1: height,
						X2: 0, Y2: drivenHalf, Z2: height,
						Radius: defaultWireRadius, Segments: segments,
					},
					// Wire 2: Director (in front of the driven element)
					{
						X1: spacing, Y1: -directorHalf, Z1: height,
						X2: spacing, Y2: directorHalf, Z2: height,
						Radius: defaultWireRadius, Segments: segments,
					},
				},
				Source: SourceDTO{
					WireIndex:    1,             // driven element
					SegmentIndex: segments / 2,  // center feed
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "free_space"},
			}, nil
		},
	}
}

// invertedVDipole creates an inverted-V dipole template, commonly used for
// HF bands (default 7.1 MHz / 40m band). The antenna has two arms drooping
// 30 degrees from a central apex, forming a "V" shape when viewed from the side.
// Each arm is lambda/4 long (total wire length = lambda/2).
//
// The 30-degree droop angle is a practical compromise: it lowers the wire ends
// for easier installation while maintaining a radiation pattern close to a
// horizontal dipole. The apex is at the specified height; wire ends are lower.
//
// Uses perfect ground because inverted-V antennas are typically deployed as
// low-height HF antennas where ground effects dominate the radiation pattern.
// The source is at segment 0 of wire 0 (the apex junction).
func invertedVDipole() Template {
	return Template{
		Name:        "inverted_v_dipole",
		Description: "Inverted-V dipole: two wires from an apex angled 30 degrees downward, over perfect ground",
		Parameters: []TemplateParam{
			{Name: "frequency_mhz", Type: "float", Default: 7.1},
			{Name: "apex_height", Type: "float", Default: 12.0},
			{Name: "segments", Type: "int", Default: 21},
		},
		Generate: func(params map[string]float64) (*TemplateResult, error) {
			freqMHz := getParam(params, "frequency_mhz", 7.1)
			apexHeight := getParam(params, "apex_height", 12.0)
			segments := int(getParam(params, "segments", 21))

			if freqMHz <= 0 {
				return nil, fmt.Errorf("frequency must be positive")
			}
			if segments < 1 {
				segments = 21
			}

			lambda := 300.0 / freqMHz
			armLen := lambda / 4.0 // each arm is a quarter wavelength

			// Decompose the arm into horizontal and vertical components
			// using the 30-degree droop angle from horizontal
			angle := 30.0 * math.Pi / 180.0
			horizontalDist := armLen * math.Cos(angle)
			verticalDrop := armLen * math.Sin(angle)
			endHeight := apexHeight - verticalDrop

			return &TemplateResult{
				Wires: []WireDTO{
					// Left arm: apex to lower-left
					{
						X1: 0, Y1: 0, Z1: apexHeight,
						X2: -horizontalDist, Y2: 0, Z2: endHeight,
						Radius: defaultWireRadius, Segments: segments,
					},
					// Right arm: apex to lower-right
					{
						X1: 0, Y1: 0, Z1: apexHeight,
						X2: horizontalDist, Y2: 0, Z2: endHeight,
						Radius: defaultWireRadius, Segments: segments,
					},
				},
				Source: SourceDTO{
					WireIndex:    0,
					SegmentIndex: 0, // fed at the apex junction
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "perfect"},
			}, nil
		},
	}
}

// fullWaveLoop creates a square full-wave loop antenna template.
// The loop perimeter equals one wavelength (4 sides of lambda/4 each),
// making it resonant. Default frequency is 14.2 MHz (20m amateur band).
//
// The loop lies in the XZ plane (Y=0) with the bottom edge at the specified
// height. Four wires trace the square clockwise: bottom, right, top, left.
// The source is center-fed on the bottom wire (wire 0, middle segment).
//
// Segments must be odd per wire so the center feed point aligns exactly.
// A full-wave loop has about 1 dB gain over a dipole and lower radiation
// angle, making it popular for DX (long-distance) HF communication.
func fullWaveLoop() Template {
	return Template{
		Name:        "full_wave_loop",
		Description: "Full-wave loop: 4 wires forming a square with perimeter = lambda, center-fed on bottom wire",
		Parameters: []TemplateParam{
			{Name: "frequency_mhz", Type: "float", Default: 14.2},
			{Name: "height", Type: "float", Default: 10.0},
			{Name: "segments", Type: "int", Default: 11},
		},
		Generate: func(params map[string]float64) (*TemplateResult, error) {
			freqMHz := getParam(params, "frequency_mhz", 14.2)
			height := getParam(params, "height", 10.0)
			segments := int(getParam(params, "segments", 11))

			if freqMHz <= 0 {
				return nil, fmt.Errorf("frequency must be positive")
			}
			if segments < 1 {
				segments = 11
			}
			if segments%2 == 0 {
				segments++
			}

			lambda := 300.0 / freqMHz
			side := lambda / 4.0       // each side = lambda/4, total perimeter = lambda
			halfSide := side / 2.0     // offset from center for the horizontal edges

			bottom := height           // Z coordinate of the bottom edge
			top := height + side       // Z coordinate of the top edge

			return &TemplateResult{
				Wires: []WireDTO{
					// Wire 0: Bottom edge (horizontal, left to right)
					{
						X1: -halfSide, Y1: 0, Z1: bottom,
						X2: halfSide, Y2: 0, Z2: bottom,
						Radius: defaultWireRadius, Segments: segments,
					},
					// Wire 1: Right edge (vertical, bottom to top)
					{
						X1: halfSide, Y1: 0, Z1: bottom,
						X2: halfSide, Y2: 0, Z2: top,
						Radius: defaultWireRadius, Segments: segments,
					},
					// Wire 2: Top edge (horizontal, right to left)
					{
						X1: halfSide, Y1: 0, Z1: top,
						X2: -halfSide, Y2: 0, Z2: top,
						Radius: defaultWireRadius, Segments: segments,
					},
					// Wire 3: Left edge (vertical, top to bottom)
					{
						X1: -halfSide, Y1: 0, Z1: top,
						X2: -halfSide, Y2: 0, Z2: bottom,
						Radius: defaultWireRadius, Segments: segments,
					},
				},
				Source: SourceDTO{
					WireIndex:    0,
					SegmentIndex: segments / 2, // center of bottom wire
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "free_space"},
			}, nil
		},
	}
}
