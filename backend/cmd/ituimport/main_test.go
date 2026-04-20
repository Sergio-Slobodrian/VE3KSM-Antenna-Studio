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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeSource helps tests drop a source GeoJSON onto disk and return the path.
func writeSource(t *testing.T, fc geoFeatureCollection) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "src.geojson")
	data, err := json.Marshal(fc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func readOut(t *testing.T, path string) outFeatureCollection {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	var fc outFeatureCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		t.Fatalf("unmarshal out: %v", err)
	}
	return fc
}

// square returns a closed 4-vertex ring centred at (cx, cy) with side half.
func square(cx, cy, half float64) [][]float64 {
	return [][]float64{
		{cx - half, cy - half},
		{cx + half, cy - half},
		{cx + half, cy + half},
		{cx - half, cy + half},
		{cx - half, cy - half},
	}
}

func polygonFeature(props map[string]interface{}, ring [][]float64) geoFeature {
	// The raw JSON form of Polygon coordinates is [[[lon,lat],...]].
	coords, _ := json.Marshal([][][]float64{ring})
	return geoFeature{
		Type:       "Feature",
		Properties: props,
		Geometry:   geoGeometry{Type: "Polygon", Coordinates: coords},
	}
}

func multiPolygonFeature(props map[string]interface{}, polys [][][]float64) geoFeature {
	outer := make([][][][]float64, 0, len(polys))
	for _, p := range polys {
		outer = append(outer, [][][]float64{p})
	}
	coords, _ := json.Marshal(outer)
	return geoFeature{
		Type:       "Feature",
		Properties: props,
		Geometry:   geoGeometry{Type: "MultiPolygon", Coordinates: coords},
	}
}

// ---------------------------------------------------------------------------
// Classification
// ---------------------------------------------------------------------------

// Explicit zone wins over sigma — and wins even when sigma would classify
// differently.
func TestClassify_ByExplicitZone(t *testing.T) {
	src := geoFeatureCollection{
		Type: "FeatureCollection",
		Features: []geoFeature{
			polygonFeature(
				map[string]interface{}{"zone": 3.0, "sigma": 5.0},
				square(0, 0, 1),
			),
		},
	}
	outPath := filepath.Join(t.TempDir(), "out.json")
	if err := run(writeSource(t, src), outPath, false, 0); err != nil {
		t.Fatalf("run: %v", err)
	}
	fc := readOut(t, outPath)
	if len(fc.Features) != 1 {
		t.Fatalf("features: want 1, got %d", len(fc.Features))
	}
	if fc.Features[0].Properties.Zone != 3 {
		t.Errorf("zone: want 3, got %d", fc.Features[0].Properties.Zone)
	}
}

// Each canonical σ value must land in its canonical zone.
func TestClassify_BySigma_Boundaries(t *testing.T) {
	cases := []struct {
		sigma    float64
		wantZone int
	}{
		{5.0, 1},      // sea water
		{0.03, 2},     // very good
		{0.01, 3},     // good
		{0.003, 4},    // moderate
		{0.001, 5},    // poor
		{0.0001, 6},   // very poor / ice
	}
	for _, c := range cases {
		c := c
		t.Run("", func(t *testing.T) {
			if got := classifyBySigma(c.sigma); got != c.wantZone {
				t.Errorf("σ=%v: got zone %d, want %d", c.sigma, got, c.wantZone)
			}
		})
	}
}

// Features with neither zone nor sigma are skipped; the rest still land.
func TestClassify_SkipWhenNothingGiven(t *testing.T) {
	src := geoFeatureCollection{
		Type: "FeatureCollection",
		Features: []geoFeature{
			polygonFeature(map[string]interface{}{"name": "no-info"}, square(0, 0, 1)),
			polygonFeature(map[string]interface{}{"sigma": 0.01}, square(10, 0, 1)),
		},
	}
	outPath := filepath.Join(t.TempDir(), "out.json")
	if err := run(writeSource(t, src), outPath, false, 0); err != nil {
		t.Fatalf("run: %v", err)
	}
	fc := readOut(t, outPath)
	if len(fc.Features) != 1 {
		t.Fatalf("features: want 1 (the sigma=0.01 one), got %d", len(fc.Features))
	}
	if fc.Features[0].Properties.Zone != 3 {
		t.Errorf("zone: want 3, got %d", fc.Features[0].Properties.Zone)
	}
}

// ---------------------------------------------------------------------------
// Geometry handling
// ---------------------------------------------------------------------------

// MultiPolygon splits into one output feature per outer ring, sharing the
// classification.
func TestMultiPolygon_Splits(t *testing.T) {
	src := geoFeatureCollection{
		Type: "FeatureCollection",
		Features: []geoFeature{
			multiPolygonFeature(
				map[string]interface{}{"zone": 2.0, "name": "split"},
				[][][]float64{
					square(-10, 0, 1),
					square(0, 0, 1),
					square(10, 0, 1),
				},
			),
		},
	}
	outPath := filepath.Join(t.TempDir(), "out.json")
	if err := run(writeSource(t, src), outPath, false, 0); err != nil {
		t.Fatalf("run: %v", err)
	}
	fc := readOut(t, outPath)
	if len(fc.Features) != 3 {
		t.Fatalf("features: want 3, got %d", len(fc.Features))
	}
	for _, f := range fc.Features {
		if f.Properties.Zone != 2 {
			t.Errorf("zone: want 2, got %d", f.Properties.Zone)
		}
	}
}

// Douglas-Peucker at non-zero epsilon removes colinear intermediate vertices
// and keeps the ring closed.
func TestDouglasPeucker_Reduces(t *testing.T) {
	// A 200-vertex ring sampled along a square perimeter — all points except
	// the 4 corners are colinear, so DP at any ε > 0 drops them all.
	ring := make([][]float64, 0, 201)
	edges := []struct{ x0, y0, x1, y1 float64 }{
		{0, 0, 10, 0},
		{10, 0, 10, 10},
		{10, 10, 0, 10},
		{0, 10, 0, 0},
	}
	for _, e := range edges {
		for i := 0; i < 50; i++ {
			t := float64(i) / 50.0
			ring = append(ring, []float64{
				e.x0 + (e.x1-e.x0)*t,
				e.y0 + (e.y1-e.y0)*t,
			})
		}
	}
	ring = append(ring, []float64{0, 0})

	simplified := simplifyDP(ring, 0.01)
	if len(simplified) > 6 {
		t.Errorf("simplified ring should collapse to ~5 vertices (4 corners + closing), got %d", len(simplified))
	}
	if len(simplified) < 4 {
		t.Fatalf("simplified ring too small: %v", simplified)
	}
	// Still closed?
	first, last := simplified[0], simplified[len(simplified)-1]
	if first[0] != last[0] || first[1] != last[1] {
		t.Errorf("simplified ring not closed: first=%v last=%v", first, last)
	}
}

// ε = 0 is a pass-through — no vertex reduction.
func TestDouglasPeucker_ZeroEpsKeepsAll(t *testing.T) {
	ring := [][]float64{
		{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}, {0, 2}, {0, 0},
	}
	got := simplifyDP(ring, 0)
	if len(got) != len(ring) {
		t.Errorf("ε=0 should pass through: got %d vertices, want %d", len(got), len(ring))
	}
}

// ---------------------------------------------------------------------------
// Merge behaviour
// ---------------------------------------------------------------------------

// Pre-existing feature with id X + new feature with id X → only one output,
// incoming wins.
func TestMerge_Dedup(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")

	// Seed the destination with an existing feature whose id will clash.
	seed := outFeatureCollection{
		Type: "FeatureCollection",
		Features: []outFeature{
			{
				Type:       "Feature",
				Properties: outProperties{ID: "itu-existing", Name: "old", Zone: 5},
				Geometry:   outGeometry{Type: "Polygon", Coordinates: [][][]float64{square(-50, -50, 1)}},
			},
			{
				Type:       "Feature",
				Properties: outProperties{ID: "itu-keep", Name: "keep", Zone: 4},
				Geometry:   outGeometry{Type: "Polygon", Coordinates: [][][]float64{square(50, 50, 1)}},
			},
		},
	}
	if err := writeOut(outPath, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	src := geoFeatureCollection{
		Type: "FeatureCollection",
		Features: []geoFeature{
			polygonFeature(
				map[string]interface{}{"id": "itu-existing", "zone": 2.0, "name": "replaced"},
				square(0, 0, 1),
			),
			polygonFeature(
				map[string]interface{}{"id": "itu-brand-new", "zone": 3.0, "name": "new"},
				square(5, 5, 1),
			),
		},
	}

	if err := run(writeSource(t, src), outPath, true, 0); err != nil {
		t.Fatalf("run: %v", err)
	}
	fc := readOut(t, outPath)
	// Expect exactly 3 features: the kept original + the replaced-existing + the brand new.
	if len(fc.Features) != 3 {
		t.Fatalf("features: want 3, got %d (%v)", len(fc.Features), fc.Features)
	}
	// itu-existing must now carry the new zone and name.
	var found bool
	for _, f := range fc.Features {
		if f.Properties.ID == "itu-existing" {
			found = true
			if f.Properties.Zone != 2 {
				t.Errorf("replaced zone: want 2, got %d", f.Properties.Zone)
			}
			if f.Properties.Name != "replaced" {
				t.Errorf("replaced name: want 'replaced', got %q", f.Properties.Name)
			}
		}
	}
	if !found {
		t.Fatal("replaced feature not found by id")
	}
}

// Default (non-merge) mode overwrites whatever was at the destination.
func TestRun_ReplacesByDefault(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")

	seed := outFeatureCollection{
		Type: "FeatureCollection",
		Features: []outFeature{{
			Type:       "Feature",
			Properties: outProperties{ID: "itu-old", Name: "old", Zone: 5},
			Geometry:   outGeometry{Type: "Polygon", Coordinates: [][][]float64{square(-50, -50, 1)}},
		}},
	}
	if err := writeOut(outPath, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	src := geoFeatureCollection{
		Type: "FeatureCollection",
		Features: []geoFeature{
			polygonFeature(map[string]interface{}{"zone": 1.0}, square(0, 0, 1)),
		},
	}
	if err := run(writeSource(t, src), outPath, false, 0); err != nil {
		t.Fatalf("run: %v", err)
	}
	fc := readOut(t, outPath)
	if len(fc.Features) != 1 {
		t.Fatalf("features: want 1 (replaced), got %d", len(fc.Features))
	}
	if fc.Features[0].Properties.Zone != 1 {
		t.Errorf("zone: want 1, got %d", fc.Features[0].Properties.Zone)
	}
}
