package mom

import (
	"fmt"
	"math"
)

// WarnSeverity grades a validation warning.  All warnings are non-
// blocking; the simulation proceeds even if any are emitted.
type WarnSeverity string

const (
	SeverityInfo  WarnSeverity = "info"  // worth knowing, not actionable
	SeverityWarn  WarnSeverity = "warn"  // likely accuracy degradation
	SeverityError WarnSeverity = "error" // results probably unreliable
)

// Warning is a single soft-validation finding.  WireIndex / SegmentIndex
// point at the offending feature (-1 means "applies to the model as a
// whole").  Code is a short machine-readable tag the frontend can use
// to render help links.  Message is a human-readable explanation.
type Warning struct {
	Code         string       `json:"code"`
	Severity     WarnSeverity `json:"severity"`
	Message      string       `json:"message"`
	WireIndex    int          `json:"wire_index,omitempty"`
	SegmentIndex int          `json:"segment_index,omitempty"`
}

// ValidateGeometry runs a battery of "thin-wire MoM accuracy heuristics"
// against the model at the requested operating frequency and returns
// any warnings it finds.  None of the checks block simulation; they
// surface as a list the frontend can show to the user.
//
// The rules implemented:
//
//  1. Segment length λ/N rule.
//     λ/20 is the NEC-2 "comfortable" target.
//     λ/10 is the practical lower bound; coarser segmentation produces
//     visibly wrong impedance and gain.
//  2. Thin-wire kernel validity.
//     The kernel assumes radius << segment length; we warn at
//     segment_length / radius < 8 and error at < 2.
//  3. Adjacent segment-length ratio.
//     The triangle basis assumes neighbouring segments are similar in
//     length; ratios > 2× warn (kernel mismatch), > 5× error.
//  4. Wire endpoints sharing radius at junctions.
//     When two wires meet at a common point but have different radii,
//     the implicit step-radius makes the thin-wire kernel jump.
//  5. Source segment count (already enforced by Validate() but worth
//     surfacing as a friendly warning).
//
// freqHz must be the operating frequency the user is about to simulate
// at.  For sweeps, callers should re-run validation at both the
// minimum and maximum frequency since wavelength changes.
func ValidateGeometry(wires []Wire, freqHz float64) []Warning {
	var ws []Warning
	if freqHz <= 0 || len(wires) == 0 {
		return ws
	}
	wavelength := C0 / freqHz

	for wi, w := range wires {
		dx := w.X2 - w.X1
		dy := w.Y2 - w.Y1
		dz := w.Z2 - w.Z1
		length := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if length <= 0 || w.Segments < 1 {
			continue
		}
		segLen := length / float64(w.Segments)

		// 1a. Segment too short relative to wavelength — Z-matrix
		// becomes near-singular when seg ≪ λ/200 (NEC-2 recommendation).
		if wavelength/segLen > 1000 {
			ws = append(ws, Warning{
				Code:      "segment_too_short_for_frequency",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("segment length %.4f m is < λ/1000 (=%.4f m) at %.3f MHz; Z-matrix is severely ill-conditioned and the solver may return non-physical impedance.  Reduce segments to ≤ %d for this frequency.  In a sweep, this is the low-frequency end constraint", segLen, wavelength/1000, freqHz/1e6, int(math.Floor(length*1000/wavelength))),
				WireIndex: wi,
			})
		} else if wavelength/segLen > 200 {
			ws = append(ws, Warning{
				Code:      "segment_short_for_frequency",
				Severity:  SeverityWarn,
				Message:   fmt.Sprintf("segment length %.4f m is < λ/200 (=%.4f m) at %.3f MHz; matrix conditioning degrades.  NEC-2 recommends ≥ λ/200 for accurate impedance.  In a sweep, this is the low-frequency end constraint", segLen, wavelength/200, freqHz/1e6),
				WireIndex: wi,
			})
		}

		// 1. λ/N rule
		if segLen > wavelength/10.0 {
			ws = append(ws, Warning{
				Code:      "segment_too_long",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("segment length %.3f m exceeds λ/10 = %.3f m at %.3f MHz; use ≥ %d segments here.  In a sweep, this rule applies at the highest frequency in the band", segLen, wavelength/10, freqHz/1e6, int(math.Ceil(20*length/wavelength))),
				WireIndex: wi,
			})
		} else if segLen > wavelength/20.0 {
			ws = append(ws, Warning{
				Code:      "segment_below_lambda_over_20",
				Severity:  SeverityWarn,
				Message:   fmt.Sprintf("segment length %.3f m exceeds λ/20 = %.3f m at %.3f MHz; impedance accuracy degraded.  Increase segments to ≥ %d for the recommended λ/20 target.  In a sweep, this is the high-frequency end constraint", segLen, wavelength/20, freqHz/1e6, int(math.Ceil(20*length/wavelength))),
				WireIndex: wi,
			})
		}

		// 2. Thin-wire kernel: segment length / radius
		if w.Radius > 0 {
			ratio := segLen / w.Radius
			switch {
			case ratio < 2:
				ws = append(ws, Warning{
					Code:      "kernel_invalid_radius",
					Severity:  SeverityError,
					Message:   fmt.Sprintf("segment_length / radius = %.2f < 2; thin-wire kernel is mathematically invalid (the wire is fatter than it is long per segment)", ratio),
					WireIndex: wi,
				})
			case ratio < 8:
				ws = append(ws, Warning{
					Code:      "kernel_marginal_radius",
					Severity:  SeverityWarn,
					Message:   fmt.Sprintf("segment_length / radius = %.2f < 8; thin-wire kernel becomes approximate.  Consider thinner wire or fewer segments", ratio),
					WireIndex: wi,
				})
			}
		}

		// 5. Source-segment requirement (need ≥ 2 segments to host a basis)
		if w.Segments < 2 {
			ws = append(ws, Warning{
				Code:      "wire_segments_too_few",
				Severity:  SeverityWarn,
				Message:   fmt.Sprintf("wire %d has %d segment(s); a load or source on this wire requires ≥ 2", wi, w.Segments),
				WireIndex: wi,
			})
		}
	}

	// 3. Adjacent segment length ratio (within a single wire all
	//    segments have equal length, so this only fires across wires
	//    that share an endpoint and have very different segment lengths).
	for i := range wires {
		for j := i + 1; j < len(wires); j++ {
			if !wiresShareEndpoint(wires[i], wires[j]) {
				continue
			}
			si := segmentLengthOf(wires[i])
			sj := segmentLengthOf(wires[j])
			if si == 0 || sj == 0 {
				continue
			}
			ratio := math.Max(si, sj) / math.Min(si, sj)
			switch {
			case ratio > 5:
				ws = append(ws, Warning{
					Code:     "adjacent_segment_ratio_severe",
					Severity: SeverityError,
					Message:  fmt.Sprintf("wires %d and %d share an endpoint but their segment lengths differ by %.1f×; expect kernel discontinuities at the junction", i, j, ratio),
				})
			case ratio > 2:
				ws = append(ws, Warning{
					Code:     "adjacent_segment_ratio_warn",
					Severity: SeverityWarn,
					Message:  fmt.Sprintf("wires %d and %d share an endpoint but their segment lengths differ by %.1f×; aim for similar segmentation across junctions", i, j, ratio),
				})
			}
		}
	}

	// 4. Wires sharing an endpoint with mismatched radii.
	for i := range wires {
		for j := i + 1; j < len(wires); j++ {
			if !wiresShareEndpoint(wires[i], wires[j]) {
				continue
			}
			ri := wires[i].Radius
			rj := wires[j].Radius
			if ri == 0 || rj == 0 {
				continue
			}
			ratio := math.Max(ri, rj) / math.Min(ri, rj)
			if ratio > 2 {
				ws = append(ws, Warning{
					Code:     "junction_radius_mismatch",
					Severity: SeverityWarn,
					Message:  fmt.Sprintf("wires %d and %d join at a common endpoint with radii differing by %.1f× (%.4f m vs %.4f m); the implicit step-radius creates a kernel discontinuity", i, j, ratio, ri, rj),
				})
			}
		}
	}

	return ws
}

// segmentLengthOf returns the per-segment length of a wire, or 0 if
// the wire has no length or zero segments.
func segmentLengthOf(w Wire) float64 {
	dx := w.X2 - w.X1
	dy := w.Y2 - w.Y1
	dz := w.Z2 - w.Z1
	length := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if w.Segments <= 0 || length <= 0 {
		return 0
	}
	return length / float64(w.Segments)
}

// wiresShareEndpoint returns true when the two wires share at least
// one endpoint within a tolerance proportional to the smaller wire's
// length (default 0.1 % of the shorter wire, or 1 µm absolute floor).
func wiresShareEndpoint(a, b Wire) bool {
	tol := 1e-6
	la := segmentLengthOf(a) * float64(maxInt(a.Segments, 1))
	lb := segmentLengthOf(b) * float64(maxInt(b.Segments, 1))
	if l := math.Min(la, lb); l > 0 {
		tol = math.Max(tol, l*1e-3)
	}
	pts := [][2][3]float64{
		{{a.X1, a.Y1, a.Z1}, {b.X1, b.Y1, b.Z1}},
		{{a.X1, a.Y1, a.Z1}, {b.X2, b.Y2, b.Z2}},
		{{a.X2, a.Y2, a.Z2}, {b.X1, b.Y1, b.Z1}},
		{{a.X2, a.Y2, a.Z2}, {b.X2, b.Y2, b.Z2}},
	}
	for _, pp := range pts {
		dx := pp[0][0] - pp[1][0]
		dy := pp[0][1] - pp[1][1]
		dz := pp[0][2] - pp[1][2]
		if math.Sqrt(dx*dx+dy*dy+dz*dz) <= tol {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
