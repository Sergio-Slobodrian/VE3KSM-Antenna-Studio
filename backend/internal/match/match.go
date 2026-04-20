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

// Package match implements closed-form impedance-matching network
// designers for the most common HF/VHF topologies:
//
//   L-network        — 2 reactive elements, 4 configurations selected
//                      automatically based on R_L vs Z0 and X_L sign.
//   pi-network       — 3 elements (shunt-series-shunt) with chosen Q.
//   T-network        — 3 elements (series-shunt-series) with chosen Q.
//   gamma match      — shunt arm + series capacitor for Yagi driven
//                      elements (computes the gamma rod length and
//                      series cap).
//   beta (hairpin)   — shunt inductor + series cap, common on folded
//                      dipoles and short Yagi feedpoints.
//
// All designers operate at a single frequency.  Inductances are in
// henries, capacitances in farads, lengths in metres, reactances and
// resistances in ohms.
package match

import (
	"fmt"
	"math"
)

// Request describes a single matching problem.
type Request struct {
	LoadR    float64 // Re(Z_load), ohms
	LoadX    float64 // Im(Z_load), ohms
	SourceZ0 float64 // characteristic impedance of the source / line, ohms
	FreqHz   float64
	// QFactor for narrow-band designs (pi, T).  Typical 5..15.  When
	// zero or negative, defaults to 10.
	QFactor float64
}

// Component is one passive element in a matching network.
type Component struct {
	Kind      string  `json:"kind"`      // "C", "L", "R", "shorted_stub", "open_stub"
	Position  string  `json:"position"`  // "series", "shunt"
	Value     float64 `json:"value"`     // farads / henries / ohms
	Reactance float64 `json:"reactance"` // at FreqHz, +ve for L, -ve for C
	Label     string  `json:"label"`     // e.g. "L1 (series)"
}

// Solution is one matching-network candidate.
type Solution struct {
	Topology   string       `json:"topology"`
	Components []Component  `json:"components"`
	Notes      string       `json:"notes,omitempty"`
	Cores      []CoreOption `json:"cores,omitempty"`
}

// CoreOption is a single toroid candidate for a transformer design.
// Populated only by the toroid topology.
type CoreOption struct {
	Name             string  `json:"name"`              // e.g. "T-50-2", "FT-114-43"
	Material         string  `json:"material"`          // e.g. "Iron powder #2"
	FreqRange        string  `json:"freq_range"`        // human-friendly band, e.g. "1-30 MHz"
	ALnHperT2        float64 `json:"al_nh_per_t2"`      // inductance index, nH per turn^2
	PrimaryTurns     int     `json:"primary_turns"`
	SecondaryTurns   int     `json:"secondary_turns"`
	PrimaryInductanceUH float64 `json:"primary_inductance_uh"`
}

// Result is the full set of designs returned to the caller.  Each
// topology may produce zero or one solution; an empty Components slice
// means that topology cannot match the requested load (e.g. L-net needs
// a specific R relationship).
type Result struct {
	LoadR    float64    `json:"load_r"`
	LoadX    float64    `json:"load_x"`
	SourceZ0 float64    `json:"source_z0"`
	FreqHz   float64    `json:"freq_hz"`
	Solutions []Solution `json:"solutions"`
}

// All returns matching-network designs for every supported topology.
func All(req Request) (Result, error) {
	if err := validate(&req); err != nil {
		return Result{}, err
	}
	out := Result{
		LoadR: req.LoadR, LoadX: req.LoadX,
		SourceZ0: req.SourceZ0, FreqHz: req.FreqHz,
	}
	for _, fn := range []func(Request) (Solution, error){
		designLNetwork,
		designPiNetwork,
		designTNetwork,
		designGammaMatch,
		designBetaMatch,
		designToroidalTransformer,
	} {
		s, err := fn(req)
		if err != nil {
			// record an empty solution with notes so the UI can show why
			out.Solutions = append(out.Solutions, Solution{
				Topology: s.Topology,
				Notes:    "skipped: " + err.Error(),
			})
			continue
		}
		out.Solutions = append(out.Solutions, s)
	}
	return out, nil
}

func validate(r *Request) error {
	if r.SourceZ0 <= 0 {
		r.SourceZ0 = 50
	}
	if r.FreqHz <= 0 {
		return fmt.Errorf("freq_hz must be positive, got %g", r.FreqHz)
	}
	if r.LoadR < 0 {
		return fmt.Errorf("load_r must be >= 0, got %g", r.LoadR)
	}
	if r.QFactor <= 0 {
		r.QFactor = 10
	}
	return nil
}

// componentFromX builds a series or shunt L or C from a reactance value.
// Positive X is interpreted as inductive (an L), negative as capacitive (a C).
func componentFromX(X, freqHz float64, position, label string) Component {
	omega := 2 * math.Pi * freqHz
	if X >= 0 {
		L := X / omega
		return Component{
			Kind: "L", Position: position, Value: L, Reactance: X, Label: label,
		}
	}
	C := -1 / (omega * X) // X<0 here
	return Component{
		Kind: "C", Position: position, Value: C, Reactance: X, Label: label,
	}
}
