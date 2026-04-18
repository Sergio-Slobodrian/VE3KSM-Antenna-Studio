// Command ituimport rewrites frontend/src/data/itu_r_p832.json from an
// external GeoJSON source (ITU-R P.832 atlas conversion, Natural Earth
// overlay, per-region dataset, etc.).
//
// For each input feature it:
//   - classifies the feature into an ITU-R P.832 zone (1-6), preferring an
//     explicit properties.zone (int in [1,6]) and falling back to
//     properties.sigma (or conductivity) against the canonical thresholds;
//   - splits MultiPolygons into one feature per outer ring and drops holes;
//   - optionally simplifies each outer ring via Douglas-Peucker at a chosen
//     epsilon (in lon/lat degree space);
//   - writes the classified + simplified features back out in the picker's
//     schema: FeatureCollection with properties {id, name, zone}.
//
// Usage:
//
//	ituimport -src path/to/source.geojson -out frontend/src/data/itu_r_p832.json
//	ituimport -src source.geojson -out dest.json -merge       # append to existing
//	ituimport -src source.geojson -out dest.json -eps 0.1     # coarser simplify
//	ituimport -src source.geojson -out dest.json -eps 0       # no simplification
//
// The tool never downloads anything; it only consumes files the caller points
// it at.  Run it via the Makefile target update-itu-zones.
package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Canonical σ thresholds for classification when a feature lacks an explicit
// zone.  A feature's σ falls into the first row whose Min it meets or exceeds.
// Values are the geometric midpoints between the canonical zone σ values in
// ITU_P832_ZONES so a feature tagged exactly at a canonical σ lands in its
// intended zone.
//
// Zone canonical σ: 5.0 / 0.03 / 0.01 / 0.003 / 0.001 / 0.0001 S/m.
type zoneThreshold struct {
	Min   float64
	Zone  int
	Label string
}

var sigmaThresholds = []zoneThreshold{
	{Min: 1.0, Zone: 1, Label: "Sea water"},
	{Min: 0.02, Zone: 2, Label: "Very good ground"},
	{Min: 0.005, Zone: 3, Label: "Good ground"},
	{Min: 0.002, Zone: 4, Label: "Moderate ground"},
	{Min: 0.0005, Zone: 5, Label: "Poor ground"},
	// Anything below 0.0005 → zone 6.  Provided as a separate constant so the
	// classifier is a pure linear scan.
}

const floorZone = 6
const floorLabel = "Very poor / ice"

// classifyBySigma returns the zone number (1-6) for a conductivity value.
func classifyBySigma(sigma float64) int {
	for _, t := range sigmaThresholds {
		if sigma >= t.Min {
			return t.Zone
		}
	}
	return floorZone
}

// zoneLabel maps a zone number to its human-friendly label.  Kept in sync
// with ITU_P832_ZONES on the frontend side.
func zoneLabel(zone int) string {
	switch zone {
	case 1:
		return "Sea water"
	case 2:
		return "Very good ground"
	case 3:
		return "Good ground"
	case 4:
		return "Moderate ground"
	case 5:
		return "Poor ground"
	case 6:
		return floorLabel
	}
	return "Unknown"
}

// ----------------------------------------------------------------------------
// GeoJSON types (partial — we only use what we need).
// ----------------------------------------------------------------------------

type geoFeatureCollection struct {
	Type     string       `json:"type"`
	Note     string       `json:"_note,omitempty"`
	Features []geoFeature `json:"features"`
}

type geoFeature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   geoGeometry            `json:"geometry"`
}

// geoGeometry is decoded loosely because Polygon and MultiPolygon have
// different shapes for coordinates; we re-parse Coordinates case-by-case.
type geoGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// featureKey extracts classification inputs from a feature's properties.
// Returns zone when present (1-6) and/or sigma when present (>0).  Either or
// both may be missing — the caller decides how to proceed.
func featureKey(p map[string]interface{}) (zone int, sigma float64, hasZone, hasSigma bool) {
	if v, ok := p["zone"]; ok {
		switch t := v.(type) {
		case float64:
			if t >= 1 && t <= 6 {
				zone = int(t)
				hasZone = true
			}
		case int:
			if t >= 1 && t <= 6 {
				zone = t
				hasZone = true
			}
		}
	}
	for _, key := range []string{"sigma", "conductivity"} {
		if v, ok := p[key]; ok {
			if f, ok := v.(float64); ok && f > 0 {
				sigma = f
				hasSigma = true
				break
			}
		}
	}
	return
}

// stringProp pulls a string-valued property or returns "" if missing.
func stringProp(p map[string]interface{}, key string) string {
	if v, ok := p[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ----------------------------------------------------------------------------
// Polygon extraction + Douglas-Peucker simplification.
// ----------------------------------------------------------------------------

// decodePolygonRings returns the outer rings for a Polygon or MultiPolygon
// geometry.  Holes (inner rings) are discarded — the picker's point-in-poly
// lookup doesn't model them and dropping them keeps the bundle small.
func decodePolygonRings(g geoGeometry) ([][][]float64, error) {
	switch g.Type {
	case "Polygon":
		var rings [][][]float64
		if err := json.Unmarshal(g.Coordinates, &rings); err != nil {
			return nil, fmt.Errorf("polygon coords: %w", err)
		}
		if len(rings) == 0 {
			return nil, errors.New("polygon has no rings")
		}
		return [][][]float64{rings[0]}, nil
	case "MultiPolygon":
		var polygons [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &polygons); err != nil {
			return nil, fmt.Errorf("multipolygon coords: %w", err)
		}
		out := make([][][]float64, 0, len(polygons))
		for _, poly := range polygons {
			if len(poly) == 0 {
				continue
			}
			out = append(out, poly[0])
		}
		if len(out) == 0 {
			return nil, errors.New("multipolygon has no outer rings")
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported geometry type %q", g.Type)
	}
}

// perpDist is the perpendicular distance from (px,py) to the line (ax,ay)-(bx,by).
func perpDist(px, py, ax, ay, bx, by float64) float64 {
	dx := bx - ax
	dy := by - ay
	if dx == 0 && dy == 0 {
		// Degenerate segment → point-to-point distance.
		return math.Hypot(px-ax, py-ay)
	}
	num := math.Abs(dy*px - dx*py + bx*ay - by*ax)
	den := math.Hypot(dx, dy)
	return num / den
}

// simplifyDP runs Douglas-Peucker on a ring in lon/lat degree space.  The ring
// must be closed (first == last); the result is also closed.  ε = 0 returns
// the ring unchanged (no allocation, no simplification).
func simplifyDP(ring [][]float64, eps float64) [][]float64 {
	if eps <= 0 || len(ring) < 4 {
		return ring
	}
	// Operate on the open ring (drop trailing duplicate) then re-close.
	open := ring
	if len(open) >= 2 &&
		open[0][0] == open[len(open)-1][0] &&
		open[0][1] == open[len(open)-1][1] {
		open = open[:len(open)-1]
	}
	keep := make([]bool, len(open))
	keep[0] = true
	keep[len(open)-1] = true
	// Recursive stack — avoid deep recursion on huge inputs.
	type span struct{ lo, hi int }
	stack := []span{{0, len(open) - 1}}
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if s.hi-s.lo < 2 {
			continue
		}
		ax, ay := open[s.lo][0], open[s.lo][1]
		bx, by := open[s.hi][0], open[s.hi][1]
		maxD := 0.0
		maxI := -1
		for i := s.lo + 1; i < s.hi; i++ {
			d := perpDist(open[i][0], open[i][1], ax, ay, bx, by)
			if d > maxD {
				maxD = d
				maxI = i
			}
		}
		if maxD > eps && maxI != -1 {
			keep[maxI] = true
			stack = append(stack, span{s.lo, maxI}, span{maxI, s.hi})
		}
	}
	out := make([][]float64, 0, len(open)+1)
	for i, k := range keep {
		if k {
			out = append(out, open[i])
		}
	}
	// Re-close.
	out = append(out, []float64{out[0][0], out[0][1]})
	return out
}

// ----------------------------------------------------------------------------
// Classification + feature conversion.
// ----------------------------------------------------------------------------

// outFeature is the picker's on-disk schema (subset of GeoJSON).
type outFeature struct {
	Type       string         `json:"type"`
	Properties outProperties  `json:"properties"`
	Geometry   outGeometry    `json:"geometry"`
}

type outProperties struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Zone int    `json:"zone"`
}

type outGeometry struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"` // always [[outerRing]]
}

type outFeatureCollection struct {
	Type     string       `json:"type"`
	Note     string       `json:"_note,omitempty"`
	Features []outFeature `json:"features"`
}

// classifyAndConvert turns a single input feature into zero or more output
// features.  Skipped features (no zone + no sigma) contribute nothing and are
// counted at the call site.
func classifyAndConvert(
	f geoFeature, sourceHash string, startIndex int, eps float64,
) ([]outFeature, error) {
	zone, sigma, hasZone, hasSigma := featureKey(f.Properties)
	if !hasZone && !hasSigma {
		return nil, nil // skipped
	}
	if !hasZone {
		zone = classifyBySigma(sigma)
	}
	rings, err := decodePolygonRings(f.Geometry)
	if err != nil {
		return nil, err
	}
	name := stringProp(f.Properties, "name")
	id := stringProp(f.Properties, "id")
	out := make([]outFeature, 0, len(rings))
	for i, ring := range rings {
		simplified := simplifyDP(ring, eps)
		fid := id
		if fid == "" {
			fid = fmt.Sprintf("itu-%s-%d", sourceHash, startIndex+i)
		} else if len(rings) > 1 {
			fid = fmt.Sprintf("%s-%d", id, i)
		}
		fname := name
		if fname == "" {
			fname = fmt.Sprintf("ITU zone %d — %s", zone, zoneLabel(zone))
		}
		out = append(out, outFeature{
			Type: "Feature",
			Properties: outProperties{
				ID:   fid,
				Name: fname,
				Zone: zone,
			},
			Geometry: outGeometry{
				Type:        "Polygon",
				Coordinates: [][][]float64{simplified},
			},
		})
	}
	return out, nil
}

// convertAll processes every feature in a source collection.  Returns the
// classified features plus the count that was skipped for lack of zone/sigma
// keys.
func convertAll(src geoFeatureCollection, sourceHash string, eps float64) ([]outFeature, int, error) {
	out := make([]outFeature, 0, len(src.Features))
	skipped := 0
	for i, f := range src.Features {
		produced, err := classifyAndConvert(f, sourceHash, i, eps)
		if err != nil {
			return nil, 0, fmt.Errorf("feature %d: %w", i, err)
		}
		if len(produced) == 0 {
			skipped++
			continue
		}
		out = append(out, produced...)
	}
	return out, skipped, nil
}

// mergeDedup concatenates existing and incoming feature lists, deduplicating
// by properties.id (incoming wins).  Features with empty ids are kept as-is.
func mergeDedup(existing, incoming []outFeature) []outFeature {
	idx := map[string]int{}
	out := make([]outFeature, 0, len(existing)+len(incoming))
	for _, f := range existing {
		if f.Properties.ID != "" {
			idx[f.Properties.ID] = len(out)
		}
		out = append(out, f)
	}
	for _, f := range incoming {
		if f.Properties.ID == "" {
			out = append(out, f)
			continue
		}
		if pos, ok := idx[f.Properties.ID]; ok {
			out[pos] = f
			continue
		}
		idx[f.Properties.ID] = len(out)
		out = append(out, f)
	}
	return out
}

// ----------------------------------------------------------------------------
// File I/O + main.
// ----------------------------------------------------------------------------

func readSource(path string) (geoFeatureCollection, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return geoFeatureCollection{}, "", fmt.Errorf("read source: %w", err)
	}
	h := sha1.Sum(data)
	sum := hex.EncodeToString(h[:])[:8]
	var fc geoFeatureCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		return geoFeatureCollection{}, "", fmt.Errorf("parse source: %w", err)
	}
	if !strings.EqualFold(fc.Type, "FeatureCollection") {
		return geoFeatureCollection{}, "", fmt.Errorf("source must be a GeoJSON FeatureCollection, got %q", fc.Type)
	}
	return fc, sum, nil
}

func readExistingOut(path string) ([]outFeature, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var fc outFeatureCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse existing out file: %w", err)
	}
	return fc.Features, nil
}

func writeOut(path string, fc outFeatureCollection) error {
	buf, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(buf, '\n'), 0o644)
}

func run(src, out string, merge bool, eps float64) error {
	fc, hash, err := readSource(src)
	if err != nil {
		return err
	}
	incoming, skipped, err := convertAll(fc, hash, eps)
	if err != nil {
		return err
	}

	var features []outFeature
	if merge {
		existing, err := readExistingOut(out)
		if err != nil {
			return fmt.Errorf("merge: %w", err)
		}
		features = mergeDedup(existing, incoming)
	} else {
		features = incoming
	}

	note := fmt.Sprintf("Generated by backend/cmd/ituimport from %s (sha1 %s). eps=%g. %d features.",
		filepath.Base(src), hash, eps, len(features))

	fmt.Fprintf(os.Stderr,
		"ituimport: src=%s → %d features (%d skipped, missing zone/sigma). out=%s merge=%v eps=%g\n",
		src, len(features), skipped, out, merge, eps)

	return writeOut(out, outFeatureCollection{
		Type:     "FeatureCollection",
		Note:     note,
		Features: features,
	})
}

func main() {
	src := flag.String("src", "", "path to source GeoJSON FeatureCollection (required)")
	out := flag.String("out", "", "path to destination JSON (required)")
	merge := flag.Bool("merge", false, "append to existing destination instead of replacing (dedup by properties.id)")
	eps := flag.Float64("eps", 0.05, "Douglas-Peucker epsilon in lon/lat degrees (0 disables simplification)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: ituimport -src <source.geojson> -out <dest.json> [-merge] [-eps N]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Classification thresholds when properties.zone is absent (σ → zone):")
		for _, t := range sigmaThresholds {
			fmt.Fprintf(os.Stderr, "  σ ≥ %-8g S/m  → zone %d (%s)\n", t.Min, t.Zone, t.Label)
		}
		fmt.Fprintf(os.Stderr, "  otherwise          → zone %d (%s)\n", floorZone, floorLabel)
		fmt.Fprintln(os.Stderr, "")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *src == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*src, *out, *merge, *eps); err != nil {
		fmt.Fprintln(os.Stderr, "ituimport:", err)
		os.Exit(1)
	}
}
