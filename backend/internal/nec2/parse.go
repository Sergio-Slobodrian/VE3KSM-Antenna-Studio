package nec2

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Card categorises the parsed result of one NEC line.  The Mnemonic is
// the two-letter card type ("GW", "EX", ...).  Ints/Floats hold the
// numeric fields after the mnemonic; Comment is set for CM/CE.  Line
// is the original 1-based file line for diagnostics.
type Card struct {
	Mnemonic string
	Ints     []int
	Floats   []float64
	Comment  string
	Line     int
}

// File is the parsed contents of a NEC-2 deck.
type File struct {
	Comments []string
	Cards    []Card
}

// Parse reads a NEC-2 deck from r and returns its structured form.
// Parsing tolerates blank lines and lines beginning with '#' as comments.
// Numeric fields are tab- or space-delimited.  Inline trailing comments
// after a single quote (Fortran style) are stripped.
func Parse(r io.Reader) (*File, error) {
	f := &File{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text()
		trim := strings.TrimSpace(raw)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		// Strip inline trailing single-quote comment (Fortran convention).
		if i := strings.Index(trim, "'"); i >= 0 {
			trim = strings.TrimSpace(trim[:i])
			if trim == "" {
				continue
			}
		}
		if len(trim) < 2 {
			return nil, fmt.Errorf("line %d: too short for a NEC card", lineNo)
		}
		mnem := strings.ToUpper(trim[:2])
		rest := ""
		if len(trim) > 2 {
			rest = strings.TrimSpace(trim[2:])
		}
		card := Card{Mnemonic: mnem, Line: lineNo}
		switch mnem {
		case "CM", "CE":
			card.Comment = rest
			f.Comments = append(f.Comments, rest)
		case "EN":
			f.Cards = append(f.Cards, card)
			return f, nil
		default:
			ints, floats, err := splitFields(rest)
			if err != nil {
				return nil, fmt.Errorf("line %d (%s): %w", lineNo, mnem, err)
			}
			card.Ints = ints
			card.Floats = floats
		}
		f.Cards = append(f.Cards, card)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	return f, nil
}

// splitFields parses the comma- or whitespace-separated numeric fields
// after the card mnemonic.  Integer-looking fields are placed in Ints,
// float-looking ones in Floats.  Both arrays grow in parallel: each
// field is added to whichever slice's index matches the field position.
// The card-type-specific consumer (GW, EX, ...) re-slices these by
// position.  Empty / "0"-by-default trailing fields are not synthesised
// here; consumers must check len() before indexing.
func splitFields(s string) ([]int, []float64, error) {
	if s == "" {
		return nil, nil, nil
	}
	// Accept comma OR whitespace.
	norm := strings.NewReplacer(",", " ").Replace(s)
	parts := strings.Fields(norm)
	ints := make([]int, len(parts))
	floats := make([]float64, len(parts))
	for i, p := range parts {
		if iv, err := strconv.Atoi(p); err == nil {
			ints[i] = iv
			floats[i] = float64(iv)
			continue
		}
		fv, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("field %d %q: %w", i+1, p, err)
		}
		floats[i] = fv
		ints[i] = int(math.Trunc(fv))
	}
	return ints, floats, nil
}

// FieldInt returns the i-th (0-based) integer field of the card, or
// def if the field is missing.
func (c Card) FieldInt(i, def int) int {
	if i < len(c.Ints) {
		return c.Ints[i]
	}
	return def
}

// FieldFloat returns the i-th (0-based) float field, or def.
func (c Card) FieldFloat(i int, def float64) float64 {
	if i < len(c.Floats) {
		return c.Floats[i]
	}
	return def
}
