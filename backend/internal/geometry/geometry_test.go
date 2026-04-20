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
	"math"
	"testing"
)

// ---------- wire.go ----------

func TestWireLength(t *testing.T) {
	tests := []struct {
		name                       string
		x1, y1, z1, x2, y2, z2    float64
		want                       float64
	}{
		{"3-4-5 triangle", 0, 0, 0, 3, 4, 0, 5.0},
		{"unit diagonal", 0, 0, 0, 1, 1, 1, math.Sqrt(3)},
		{"same point", 0, 0, 0, 0, 0, 0, 0.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WireLength(tc.x1, tc.y1, tc.z1, tc.x2, tc.y2, tc.z2)
			if math.Abs(got-tc.want) > 1e-12 {
				t.Errorf("WireLength = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateWire(t *testing.T) {
	t.Run("valid wire", func(t *testing.T) {
		w := WireDTO{X1: 0, Y1: 0, Z1: 0, X2: 0, Y2: 0, Z2: 1, Radius: 0.001, Segments: 11}
		if err := ValidateWire(w); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("zero length", func(t *testing.T) {
		w := WireDTO{X1: 1, Y1: 2, Z1: 3, X2: 1, Y2: 2, Z2: 3, Radius: 0.001, Segments: 11}
		if err := ValidateWire(w); err == nil {
			t.Error("expected error for zero-length wire")
		}
	})

	t.Run("negative radius", func(t *testing.T) {
		w := WireDTO{X1: 0, Y1: 0, Z1: 0, X2: 0, Y2: 0, Z2: 1, Radius: -0.001, Segments: 11}
		if err := ValidateWire(w); err == nil {
			t.Error("expected error for negative radius")
		}
	})

	t.Run("zero segments", func(t *testing.T) {
		w := WireDTO{X1: 0, Y1: 0, Z1: 0, X2: 0, Y2: 0, Z2: 1, Radius: 0.001, Segments: 0}
		if err := ValidateWire(w); err == nil {
			t.Error("expected error for zero segments")
		}
	})

	t.Run("thin wire ratio violation", func(t *testing.T) {
		// 1 segment of length 1, radius 0.6 => ratio = 0.6 > 0.5
		w := WireDTO{X1: 0, Y1: 0, Z1: 0, X2: 0, Y2: 0, Z2: 1, Radius: 0.6, Segments: 1}
		if err := ValidateWire(w); err == nil {
			t.Error("expected error for thin-wire ratio violation")
		}
	})
}

// ---------- ground.go ----------

func TestValidateGround(t *testing.T) {
	validCases := []struct {
		name   string
		ground GroundDTO
	}{
		{"empty type defaults to free_space", GroundDTO{Type: ""}},
		{"free_space", GroundDTO{Type: "free_space"}},
		{"perfect", GroundDTO{Type: "perfect"}},
		{"real with valid params", GroundDTO{Type: "real", Conductivity: 0.005, Permittivity: 13.0}},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateGround(tc.ground); err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}

	invalidCases := []struct {
		name   string
		ground GroundDTO
	}{
		{"invalid type", GroundDTO{Type: "wet_sand"}},
		{"real zero conductivity", GroundDTO{Type: "real", Conductivity: 0, Permittivity: 13.0}},
		{"real negative conductivity", GroundDTO{Type: "real", Conductivity: -1, Permittivity: 13.0}},
		{"real zero permittivity", GroundDTO{Type: "real", Conductivity: 0.005, Permittivity: 0}},
		{"real negative permittivity", GroundDTO{Type: "real", Conductivity: 0.005, Permittivity: -5}},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateGround(tc.ground); err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
}

// ---------- templates.go ----------

func TestGetTemplatesCount(t *testing.T) {
	templates := GetTemplates()
	if len(templates) != 5 {
		t.Fatalf("expected 5 templates, got %d", len(templates))
	}
}

// withinPct returns true if got is within pct% of want.
func withinPct(got, want, pct float64) bool {
	if want == 0 {
		return got == 0
	}
	return math.Abs(got-want)/math.Abs(want) <= pct/100.0
}

func findTemplate(name string) *Template {
	for _, tmpl := range GetTemplates() {
		if tmpl.Name == name {
			t := tmpl
			return &t
		}
	}
	return nil
}

func wireLen(w WireDTO) float64 {
	return WireLength(w.X1, w.Y1, w.Z1, w.X2, w.Y2, w.Z2)
}

func TestHalfWaveDipole(t *testing.T) {
	tmpl := findTemplate("half_wave_dipole")
	if tmpl == nil {
		t.Fatal("template half_wave_dipole not found")
	}

	res, err := tmpl.Generate(nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// 1 wire
	if len(res.Wires) != 1 {
		t.Fatalf("expected 1 wire, got %d", len(res.Wires))
	}

	// free space ground
	if res.Ground.Type != "free_space" {
		t.Errorf("expected free_space ground, got %q", res.Ground.Type)
	}

	// center-fed: segment index = segments/2
	if res.Source.SegmentIndex != res.Wires[0].Segments/2 {
		t.Errorf("expected center feed at segment %d, got %d", res.Wires[0].Segments/2, res.Source.SegmentIndex)
	}

	// wire length ~ lambda/2
	lambda := 300.0 / 146.0
	expectedLen := lambda / 2.0
	gotLen := wireLen(res.Wires[0])
	if !withinPct(gotLen, expectedLen, 1.0) {
		t.Errorf("wire length = %f, want ~%f (lambda/2)", gotLen, expectedLen)
	}
}

func TestQuarterWaveVertical(t *testing.T) {
	tmpl := findTemplate("quarter_wave_vertical")
	if tmpl == nil {
		t.Fatal("template quarter_wave_vertical not found")
	}

	res, err := tmpl.Generate(nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// 1 wire
	if len(res.Wires) != 1 {
		t.Fatalf("expected 1 wire, got %d", len(res.Wires))
	}

	// perfect ground
	if res.Ground.Type != "perfect" {
		t.Errorf("expected perfect ground, got %q", res.Ground.Type)
	}

	// base-fed: segment 0
	if res.Source.SegmentIndex != 0 {
		t.Errorf("expected base feed at segment 0, got %d", res.Source.SegmentIndex)
	}

	// wire length ~ lambda/4
	lambda := 300.0 / 146.0
	expectedLen := lambda / 4.0
	gotLen := wireLen(res.Wires[0])
	if !withinPct(gotLen, expectedLen, 1.0) {
		t.Errorf("wire length = %f, want ~%f (lambda/4)", gotLen, expectedLen)
	}
}

func TestThreeElementYagi(t *testing.T) {
	tmpl := findTemplate("3_element_yagi")
	if tmpl == nil {
		t.Fatal("template 3_element_yagi not found")
	}

	res, err := tmpl.Generate(nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// 3 wires
	if len(res.Wires) != 3 {
		t.Fatalf("expected 3 wires, got %d", len(res.Wires))
	}

	// free space ground
	if res.Ground.Type != "free_space" {
		t.Errorf("expected free_space ground, got %q", res.Ground.Type)
	}

	// driven element is wire 1, center-fed
	if res.Source.WireIndex != 1 {
		t.Errorf("expected source on wire 1, got %d", res.Source.WireIndex)
	}
	if res.Source.SegmentIndex != res.Wires[1].Segments/2 {
		t.Errorf("expected center feed at segment %d, got %d", res.Wires[1].Segments/2, res.Source.SegmentIndex)
	}
}

func TestInvertedVDipole(t *testing.T) {
	tmpl := findTemplate("inverted_v_dipole")
	if tmpl == nil {
		t.Fatal("template inverted_v_dipole not found")
	}

	res, err := tmpl.Generate(nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// 2 wires
	if len(res.Wires) != 2 {
		t.Fatalf("expected 2 wires, got %d", len(res.Wires))
	}

	// perfect ground
	if res.Ground.Type != "perfect" {
		t.Errorf("expected perfect ground, got %q", res.Ground.Type)
	}
}

func TestFullWaveLoop(t *testing.T) {
	tmpl := findTemplate("full_wave_loop")
	if tmpl == nil {
		t.Fatal("template full_wave_loop not found")
	}

	res, err := tmpl.Generate(nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// 4 wires
	if len(res.Wires) != 4 {
		t.Fatalf("expected 4 wires, got %d", len(res.Wires))
	}

	// free space ground
	if res.Ground.Type != "free_space" {
		t.Errorf("expected free_space ground, got %q", res.Ground.Type)
	}

	// perimeter ~ lambda
	lambda := 300.0 / 14.2
	perimeter := 0.0
	for _, w := range res.Wires {
		perimeter += wireLen(w)
	}
	if !withinPct(perimeter, lambda, 1.0) {
		t.Errorf("perimeter = %f, want ~%f (lambda)", perimeter, lambda)
	}
}

func TestTemplateCustomFrequency(t *testing.T) {
	tmpl := findTemplate("half_wave_dipole")
	if tmpl == nil {
		t.Fatal("template half_wave_dipole not found")
	}

	// Default frequency 146 MHz
	resDefault, err := tmpl.Generate(nil)
	if err != nil {
		t.Fatalf("Generate with defaults returned error: %v", err)
	}

	// Custom frequency 300 MHz (lambda = 1 m, half-wave = 0.5 m)
	resCustom, err := tmpl.Generate(map[string]float64{"frequency_mhz": 300.0})
	if err != nil {
		t.Fatalf("Generate with custom freq returned error: %v", err)
	}

	defaultLen := wireLen(resDefault.Wires[0])
	customLen := wireLen(resCustom.Wires[0])

	// Wire lengths should differ
	if math.Abs(defaultLen-customLen) < 1e-6 {
		t.Errorf("expected different wire lengths for different frequencies, both got %f", defaultLen)
	}

	// Custom length should be lambda/2 = 0.5 m
	if !withinPct(customLen, 0.5, 1.0) {
		t.Errorf("custom wire length = %f, want ~0.5", customLen)
	}
}

func TestTemplateNegativeFrequency(t *testing.T) {
	for _, name := range []string{"half_wave_dipole", "quarter_wave_vertical", "3_element_yagi", "inverted_v_dipole", "full_wave_loop"} {
		t.Run(name, func(t *testing.T) {
			tmpl := findTemplate(name)
			if tmpl == nil {
				t.Fatalf("template %s not found", name)
			}
			_, err := tmpl.Generate(map[string]float64{"frequency_mhz": -10.0})
			if err == nil {
				t.Errorf("expected error for negative frequency in template %s", name)
			}
		})
	}
}
