package nec2

import (
	"fmt"
	"io"
	"math"
	"strings"
)

// WriteOptions configures Write.
type WriteOptions struct {
	// Comments inserted as CM cards at the top of the file.  An empty
	// slice produces a single fallback comment.
	Comments []string

	// FreqStartMHz / FreqStepMHz / FreqSteps drive the FR card.  When
	// FreqSteps <= 1 a single-frequency FR is emitted.  When
	// FreqStartMHz is zero no FR card is written.
	FreqStartMHz float64
	FreqStepMHz  float64
	FreqSteps    int
}

// GeometryWriteInput is a JSON-friendly view of a SimulationInput
// suitable for Write.  Keeping it independent of the mom package lets
// callers export from places that don't have the solver loaded.
type GeometryWriteInput struct {
	Wires             []WireRow
	Loads             []LoadRow
	TransmissionLines []TLRow
	Source            SourceRow
	GroundType        string
	Conductivity      float64
	Permittivity      float64
	MoisturePreset    string
	Weather           WeatherRow
}

type WireRow struct {
	X1, Y1, Z1 float64
	X2, Y2, Z2 float64
	Radius     float64
	Segments   int
	Sigma      float64 // 0 = perfect / unspecified

	// Dielectric coating fields from the MoM IS-card model. NEC-2 has no
	// native equivalent, so Write collapses these into an effective radius
	// (Tsai/Richmond lossless approximation) and preserves the originals as
	// CM comment cards.  Zero thickness ⇒ bare wire and these fields are
	// ignored.
	CoatingThickness float64
	CoatingEpsR      float64
	CoatingLossTan   float64
}

// WeatherRow mirrors mom.WeatherConfig for the NEC-2 writer.  When
// Thickness > 0 and EpsR ≥ 1 (after preset fallback) the writer adds the
// weather film as an outer layer on every wire's effective-radius stack.
type WeatherRow struct {
	Preset    string
	Thickness float64
	EpsR      float64
	LossTan   float64
}

type LoadRow struct {
	WireIndex        int
	SegmentIndex     int
	ParallelTopology bool
	R, L, C          float64
}

type TLRow struct {
	AWireIndex, ASegmentIndex int
	BWireIndex, BSegmentIndex int // -1 = short, -2 = open
	Z0, Length                float64
}

type SourceRow struct {
	WireIndex    int
	SegmentIndex int
	Voltage      complex128
}

// weatherDefaults mirrors mom.weatherLayer for the writer so we don't
// have to depend on the mom package.  Explicit εr/tanδ on the WeatherRow
// override the preset values; a dry preset (or no preset) is inert.
func weatherDefaults(preset string) (epsR, lossTan float64) {
	switch preset {
	case "rain":
		return 80.0, 0.05
	case "ice":
		return 3.17, 0.001
	case "wet_snow":
		return 1.6, 0.005
	}
	return 0, 0
}

// effectiveRadius collapses a multi-layer dielectric stack into the
// equivalent bare-wire radius using the lossless Tsai/Richmond formula:
//
//	ln(a_eff) = ln(a) + Σ_i (1 − 1/εr_i) · ln(b_i / b_{i−1})
//
// Layers are given inner-to-outer as (εr, outer radius) pairs.  The
// approximation matches the real part of the IS-card per-unit-length
// impedance in the quasi-TEM limit and reproduces the resonance shift /
// velocity-factor change; it does not capture resistive loading from
// lossy coatings (tanδ > 0), which must be reported as a warning.
func effectiveRadius(a float64, layers [][2]float64) float64 {
	if a <= 0 || len(layers) == 0 {
		return a
	}
	lnAEff := math.Log(a)
	prevR := a
	for _, l := range layers {
		eps, outer := l[0], l[1]
		if eps < 1 || outer <= prevR {
			continue
		}
		lnAEff += (1 - 1/eps) * math.Log(outer/prevR)
		prevR = outer
	}
	return math.Exp(lnAEff)
}

// Write serialises a GeometryWriteInput to a NEC-2 deck.  The output
// is free-format with one card per line.  The returned []string holds
// non-fatal warnings (e.g. lossy coatings that NEC-2 cannot represent
// exactly); the file itself is still valid and usable.
func Write(w io.Writer, input GeometryWriteInput, opts WriteOptions) ([]string, error) {
	var sb strings.Builder
	var warnings []string

	// Resolve the weather film once.  An explicit εr ≥ 1 on the
	// WeatherRow overrides the preset default (matches the solver).
	weatherEpsR, weatherLossTan := weatherDefaults(input.Weather.Preset)
	if input.Weather.EpsR >= 1 {
		weatherEpsR = input.Weather.EpsR
		weatherLossTan = input.Weather.LossTan
	}
	hasWeather := input.Weather.Thickness > 0 && weatherEpsR >= 1

	// Figure out whether any wire will be approximated so we can emit a
	// single explanatory header instead of one per wire.
	anyCoating := hasWeather
	anyLossy := hasWeather && weatherLossTan > 0
	for _, wire := range input.Wires {
		if wire.CoatingThickness > 0 && wire.CoatingEpsR > 1 {
			anyCoating = true
			if wire.CoatingLossTan > 0 {
				anyLossy = true
			}
		}
	}

	comments := opts.Comments
	if len(comments) == 0 {
		comments = []string{"VE3KSM Antenna Studio export"}
	}
	for _, c := range comments {
		fmt.Fprintf(&sb, "CM %s\n", c)
	}
	if anyCoating {
		fmt.Fprint(&sb, "CM Dielectric coatings approximated by effective radius\n")
		fmt.Fprint(&sb, "CM   (Tsai/Richmond: ln(a_eff) = ln(a) + Σ (1 - 1/εr_i) ln(b_i/b_{i-1}))\n")
		if anyLossy {
			fmt.Fprint(&sb, "CM Warning: lossy coatings (tanδ > 0) cannot be represented in NEC-2;\n")
			fmt.Fprint(&sb, "CM   resistive loading from tanδ is DROPPED in this export.\n")
			warnings = append(warnings,
				"Lossy dielectric coatings (tanδ > 0) cannot be represented in NEC-2; resistive loading has been dropped in the exported deck.")
		}
		if hasWeather {
			fmt.Fprintf(&sb, "CM Weather: preset=%q thickness=%g m εr=%g tanδ=%g\n",
				input.Weather.Preset, input.Weather.Thickness, weatherEpsR, weatherLossTan)
		}
		warnings = append(warnings,
			"Dielectric coatings were approximated by an effective wire radius. Use NEC-4 or the in-app solver for full fidelity.")
	}
	fmt.Fprint(&sb, "CE\n")

	for i, wire := range input.Wires {
		tag := i + 1
		radius := wire.Radius

		// Build the concentric layer stack (inner → outer): wire coating
		// first, weather film on top.  Empty ⇒ bare wire and we use the
		// conductor radius unchanged.
		var layers [][2]float64
		curR := wire.Radius
		if wire.CoatingThickness > 0 && wire.CoatingEpsR > 1 {
			curR += wire.CoatingThickness
			layers = append(layers, [2]float64{wire.CoatingEpsR, curR})
		}
		if hasWeather {
			layers = append(layers, [2]float64{weatherEpsR, curR + input.Weather.Thickness})
		}

		if len(layers) > 0 {
			radius = effectiveRadius(wire.Radius, layers)
			// One CM card per coated wire so the original physical
			// parameters survive the round-trip as documentation.
			if wire.CoatingThickness > 0 && wire.CoatingEpsR > 1 {
				fmt.Fprintf(&sb,
					"CM wire %d coating: thickness=%g m εr=%g tanδ=%g; a=%g m → a_eff=%g m\n",
					tag, wire.CoatingThickness, wire.CoatingEpsR, wire.CoatingLossTan,
					wire.Radius, radius)
			} else if hasWeather {
				fmt.Fprintf(&sb, "CM wire %d weather film: a=%g m → a_eff=%g m\n",
					tag, wire.Radius, radius)
			}
		}

		fmt.Fprintf(&sb, "GW %d %d %g %g %g %g %g %g %g\n",
			tag, wire.Segments,
			wire.X1, wire.Y1, wire.Z1,
			wire.X2, wire.Y2, wire.Z2,
			radius)
	}

	geFlag := 0
	if input.GroundType == "perfect" || input.GroundType == "real" {
		geFlag = 1
	}
	fmt.Fprintf(&sb, "GE %d\n", geFlag)

	for i, wire := range input.Wires {
		if wire.Sigma <= 0 {
			continue
		}
		tag := i + 1
		fmt.Fprintf(&sb, "LD 5 %d 0 0 %g\n", tag, wire.Sigma)
	}

	for _, ld := range input.Loads {
		tag := ld.WireIndex + 1
		typ := 0
		if ld.ParallelTopology {
			typ = 1
		}
		fmt.Fprintf(&sb, "LD %d %d %d %d %g %g %g\n",
			typ, tag, ld.SegmentIndex+1, ld.SegmentIndex+1, ld.R, ld.L, ld.C)
	}

	for _, tl := range input.TransmissionLines {
		tag1 := tl.AWireIndex + 1
		seg1 := tl.ASegmentIndex + 1
		tag2 := tl.BWireIndex + 1
		seg2 := tl.BSegmentIndex + 1
		z0 := tl.Z0
		switch tl.BWireIndex {
		case -1:
			tag2 = -1
			seg2 = 0
		case -2:
			tag2 = 0
			seg2 = 0
			z0 = -z0
		}
		fmt.Fprintf(&sb, "TL %d %d %d %d %g %g 0 0 0 0\n",
			tag1, seg1, tag2, seg2, z0, tl.Length)
	}

	if input.Source.Voltage != 0 {
		tag := input.Source.WireIndex + 1
		re, im := real(input.Source.Voltage), imag(input.Source.Voltage)
		fmt.Fprintf(&sb, "EX 0 %d %d 0 %g %g\n", tag, input.Source.SegmentIndex+1, re, im)
	}

	if input.GroundType == "real" {
		if mp := input.MoisturePreset; mp != "" && mp != "custom" {
			fmt.Fprintf(&sb, "CM Ground moisture preset: %s\n", mp)
		}
		fmt.Fprintf(&sb, "GN 2 0 0 0 %g %g\n", input.Permittivity, input.Conductivity)
	}

	if opts.FreqStartMHz > 0 {
		n := opts.FreqSteps
		if n < 1 {
			n = 1
		}
		step := opts.FreqStepMHz
		if n == 1 {
			step = 0
		}
		fmt.Fprintf(&sb, "FR 0 %d 0 0 %g %g\n", n, opts.FreqStartMHz, step)
	}

	fmt.Fprint(&sb, "EN\n")
	_, err := io.WriteString(w, sb.String())
	return warnings, err
}
