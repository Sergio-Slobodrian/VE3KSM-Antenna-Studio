package nec2

import (
	"fmt"
	"io"
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
}

type WireRow struct {
	X1, Y1, Z1 float64
	X2, Y2, Z2 float64
	Radius     float64
	Segments   int
	Sigma      float64 // 0 = perfect / unspecified
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

// Write serialises a GeometryWriteInput to a NEC-2 deck.  The output
// is free-format with one card per line.
func Write(w io.Writer, input GeometryWriteInput, opts WriteOptions) error {
	var sb strings.Builder

	comments := opts.Comments
	if len(comments) == 0 {
		comments = []string{"Antenna Studio export"}
	}
	for _, c := range comments {
		fmt.Fprintf(&sb, "CM %s\n", c)
	}
	fmt.Fprint(&sb, "CE\n")

	for i, wire := range input.Wires {
		tag := i + 1
		fmt.Fprintf(&sb, "GW %d %d %g %g %g %g %g %g %g\n",
			tag, wire.Segments,
			wire.X1, wire.Y1, wire.Z1,
			wire.X2, wire.Y2, wire.Z2,
			wire.Radius)
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
	return err
}
