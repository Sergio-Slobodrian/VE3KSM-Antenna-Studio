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

package match

import (
	"fmt"
	"math"
)

// toroidCore is the static catalog of common HF/VHF toroidal cores
// (matches the previous frontend table, expanded with a few more sizes).
type toroidCore struct {
	name      string
	material  string
	freqRange string
	al        float64 // nH per turn^2
}

var toroidCatalog = []toroidCore{
	{"T-37-2", "Iron powder #2", "1-30 MHz", 4.0},
	{"T-37-6", "Iron powder #6", "10-50 MHz", 3.0},
	{"T-50-2", "Iron powder #2", "1-30 MHz", 4.9},
	{"T-50-6", "Iron powder #6", "10-50 MHz", 4.0},
	{"T-68-2", "Iron powder #2", "1-30 MHz", 5.7},
	{"T-68-6", "Iron powder #6", "10-50 MHz", 4.7},
	{"T-80-2", "Iron powder #2", "1-30 MHz", 5.5},
	{"T-106-2", "Iron powder #2", "1-30 MHz", 13.5},
	{"T-130-2", "Iron powder #2", "1-30 MHz", 11.0},
	{"T-200-2", "Iron powder #2", "1-30 MHz", 12.0},
	{"FT-37-43", "Ferrite #43", "0.01-1 MHz", 420},
	{"FT-50-43", "Ferrite #43", "0.01-1 MHz", 523},
	{"FT-82-43", "Ferrite #43", "0.01-1 MHz", 557},
	{"FT-114-43", "Ferrite #43", "0.01-30 MHz", 1185},
	{"FT-140-43", "Ferrite #43", "0.01-30 MHz", 1075},
}

var commonImpedanceRatios = []struct {
	name  string
	ratio float64
	turns float64
}{
	{"1:1", 1.0, 1.0},
	{"1:4", 4.0, 0.5},
	{"4:1", 0.25, 2.0},
	{"1:9", 9.0, 1.0 / 3},
	{"9:1", 1.0 / 9, 3.0},
	{"1:16", 16.0, 0.25},
	{"16:1", 1.0 / 16, 4.0},
}

// designToroidalTransformer picks the closest standard impedance ratio
// and then enumerates every core in the catalog that yields a practical
// primary winding (>= 2, <= 80 turns) with primary inductive reactance
// >= 4 * Z_source at the design frequency.  The transformer Component
// summarises the chosen ratio; the Cores slice carries the per-core
// turns recommendations as a table.
func designToroidalTransformer(req Request) (Solution, error) {
	sol := Solution{Topology: "toroid"}
	if req.LoadR <= 0 {
		return sol, fmt.Errorf("toroidal transformer needs LoadR > 0")
	}
	z0 := req.SourceZ0
	rl := req.LoadR
	wantRatio := rl / z0

	bestIdx := 0
	bestErr := math.Inf(1)
	for i, e := range commonImpedanceRatios {
		err := math.Abs(math.Log(e.ratio / wantRatio))
		if err < bestErr {
			bestErr = err
			bestIdx = i
		}
	}
	pick := commonImpedanceRatios[bestIdx]
	mismatch := wantRatio / pick.ratio
	if mismatch < 1 {
		mismatch = 1 / mismatch
	}

	omega := 2 * math.Pi * req.FreqHz
	Lneed := 4 * z0 / omega
	turnsRatio := math.Sqrt(pick.ratio)

	// Default core for the headline transformer Component (FT-114-43:
	// useful HF span and decent power handling).
	const defaultAL = 1185e-9
	NpDefault := math.Ceil(math.Sqrt(Lneed / defaultAL))
	if NpDefault < 4 {
		NpDefault = 4
	}
	NsDefault := math.Round(NpDefault * turnsRatio)
	if NsDefault < 1 {
		NsDefault = 1
	}

	transformer := Component{
		Kind:      "transformer",
		Position:  "series",
		Value:     pick.ratio,
		Reactance: 0,
		Label: fmt.Sprintf("Transformer %s, primary=%d turns, secondary=%d turns",
			pick.name, int(NpDefault), int(NsDefault)),
	}
	sol.Components = []Component{transformer}

	// Build the per-core table.  Skip any core whose winding lands
	// outside the practical 2..80 turn range on either side.
	for _, c := range toroidCatalog {
		alH := c.al * 1e-9
		Np := math.Ceil(math.Sqrt(Lneed / alH))
		if Np < 2 {
			Np = 2
		}
		Ns := math.Round(Np * turnsRatio)
		if Ns < 1 {
			Ns = 1
		}
		if Np > 80 || Ns > 80 {
			continue
		}
		Lprim := alH * Np * Np
		sol.Cores = append(sol.Cores, CoreOption{
			Name: c.name, Material: c.material, FreqRange: c.freqRange,
			ALnHperT2:           c.al,
			PrimaryTurns:        int(Np),
			SecondaryTurns:      int(Ns),
			PrimaryInductanceUH: Lprim * 1e6,
		})
	}

	sol.Notes = fmt.Sprintf(
		"Closest standard ratio %s for load %.0f vs source %.0f Ω (mismatch %.2fx). "+
			"Primary inductive reactance target: 4 * Z0 = %.1f Ω at %.3f MHz.",
		pick.name, rl, z0, mismatch, 4*z0, req.FreqHz/1e6)
	return sol, nil
}
