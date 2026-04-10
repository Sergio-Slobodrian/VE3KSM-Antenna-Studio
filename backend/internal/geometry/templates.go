package geometry

import (
	"fmt"
	"math"
)

// Template defines a preset antenna geometry generator.
type Template struct {
	Name        string                                               `json:"name"`
	Description string                                               `json:"description"`
	Parameters  []TemplateParam                                      `json:"parameters"`
	Generate    func(params map[string]float64) (*TemplateResult, error) `json:"-"`
}

// TemplateParam describes a configurable parameter for a template.
type TemplateParam struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Default float64 `json:"default"`
}

// TemplateResult holds the generated antenna geometry.
type TemplateResult struct {
	Wires  []WireDTO  `json:"wires"`
	Source SourceDTO  `json:"source"`
	Ground GroundDTO  `json:"ground"`
}

const defaultWireRadius = 0.001 // 1 mm

// getParam retrieves a parameter value or returns its default.
func getParam(params map[string]float64, name string, def float64) float64 {
	if v, ok := params[name]; ok {
		return v
	}
	return def
}

// GetTemplates returns all available antenna templates.
func GetTemplates() []Template {
	return []Template{
		halfWaveDipole(),
		quarterWaveVertical(),
		threeElementYagi(),
		invertedVDipole(),
		fullWaveLoop(),
	}
}

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
			if segments%2 == 0 {
				segments++
			}

			lambda := 300.0 / freqMHz
			halfLen := lambda / 4.0

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
					SegmentIndex: segments / 2,
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "free_space"},
			}, nil
		},
	}
}

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
					SegmentIndex: 0,
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "perfect"},
			}, nil
		},
	}
}

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
			spacing := 0.2 * lambda

			reflectorHalf := 0.51 * lambda / 2.0
			drivenHalf := 0.48 * lambda / 2.0
			directorHalf := 0.44 * lambda / 2.0

			return &TemplateResult{
				Wires: []WireDTO{
					{
						X1: -spacing, Y1: -reflectorHalf, Z1: height,
						X2: -spacing, Y2: reflectorHalf, Z2: height,
						Radius: defaultWireRadius, Segments: segments,
					},
					{
						X1: 0, Y1: -drivenHalf, Z1: height,
						X2: 0, Y2: drivenHalf, Z2: height,
						Radius: defaultWireRadius, Segments: segments,
					},
					{
						X1: spacing, Y1: -directorHalf, Z1: height,
						X2: spacing, Y2: directorHalf, Z2: height,
						Radius: defaultWireRadius, Segments: segments,
					},
				},
				Source: SourceDTO{
					WireIndex:    1,
					SegmentIndex: segments / 2,
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "free_space"},
			}, nil
		},
	}
}

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
			armLen := lambda / 4.0

			angle := 30.0 * math.Pi / 180.0
			horizontalDist := armLen * math.Cos(angle)
			verticalDrop := armLen * math.Sin(angle)
			endHeight := apexHeight - verticalDrop

			return &TemplateResult{
				Wires: []WireDTO{
					{
						X1: 0, Y1: 0, Z1: apexHeight,
						X2: -horizontalDist, Y2: 0, Z2: endHeight,
						Radius: defaultWireRadius, Segments: segments,
					},
					{
						X1: 0, Y1: 0, Z1: apexHeight,
						X2: horizontalDist, Y2: 0, Z2: endHeight,
						Radius: defaultWireRadius, Segments: segments,
					},
				},
				Source: SourceDTO{
					WireIndex:    0,
					SegmentIndex: 0,
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "perfect"},
			}, nil
		},
	}
}

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
			side := lambda / 4.0
			halfSide := side / 2.0

			bottom := height
			top := height + side

			return &TemplateResult{
				Wires: []WireDTO{
					{
						X1: -halfSide, Y1: 0, Z1: bottom,
						X2: halfSide, Y2: 0, Z2: bottom,
						Radius: defaultWireRadius, Segments: segments,
					},
					{
						X1: halfSide, Y1: 0, Z1: bottom,
						X2: halfSide, Y2: 0, Z2: top,
						Radius: defaultWireRadius, Segments: segments,
					},
					{
						X1: halfSide, Y1: 0, Z1: top,
						X2: -halfSide, Y2: 0, Z2: top,
						Radius: defaultWireRadius, Segments: segments,
					},
					{
						X1: -halfSide, Y1: 0, Z1: top,
						X2: -halfSide, Y2: 0, Z2: bottom,
						Radius: defaultWireRadius, Segments: segments,
					},
				},
				Source: SourceDTO{
					WireIndex:    0,
					SegmentIndex: segments / 2,
					Voltage:      1.0,
				},
				Ground: GroundDTO{Type: "free_space"},
			}, nil
		},
	}
}
