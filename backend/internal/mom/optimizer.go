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

package mom

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
)

// ──────────────────────────────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────────────────────────────

// OptimVariable defines one tuneable dimension of the search space.
// The optimizer varies this parameter within [Min, Max] across iterations.
// WireIndex + Field identify which wire endpoint coordinate or property
// to modify.  Field is one of: "x1","y1","z1","x2","y2","z2","radius".
type OptimVariable struct {
	Name      string  `json:"name"`
	WireIndex int     `json:"wire_index"`
	Field     string  `json:"field"`     // coordinate name on Wire
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
}

// OptimGoal describes one term of the composite objective function.
// Metric is the name of a scalar quantity extracted from the solver result:
//   - "swr"           – standing wave ratio (unitless)
//   - "gain"          – peak gain (dBi)
//   - "front_to_back" – front-to-back ratio (dB)
//   - "impedance_r"   – feed-point resistance (Ω)
//   - "impedance_x"   – feed-point reactance (Ω)
//   - "efficiency"    – radiation efficiency (0–1)
//
// Target is the desired value; Weight scales its contribution.
// The objective evaluator minimises  Σ weight_i · |metric_i − target_i|
// for each goal.  For SWR, a target of 1.0 and weight 10 is typical.
type OptimGoal struct {
	Metric string  `json:"metric"`
	Target float64 `json:"target"`
	Weight float64 `json:"weight"`
}

// OptimRequest is the full optimisation specification received from
// the API handler.  It bundles the antenna geometry, tuning variables,
// objectives, optional sweep band, and PSO hyper-parameters.
type OptimRequest struct {
	// Base antenna geometry (wires, source, ground, loads, TLs).
	Input SimulationInput

	// Variables to optimise.
	Variables []OptimVariable `json:"variables"`

	// Objective goals.
	Goals []OptimGoal `json:"goals"`

	// Optional: evaluate across a frequency band instead of a single point.
	// When FreqStartHz > 0 && FreqEndHz > FreqStartHz, the objective
	// evaluates at FreqSteps points across the band and uses the worst-case.
	FreqStartHz float64 `json:"freq_start_hz"`
	FreqEndHz   float64 `json:"freq_end_hz"`
	FreqSteps   int     `json:"freq_steps"`

	// PSO hyper-parameters (with sensible defaults if zero).
	Particles  int `json:"particles"`   // swarm size (default 20)
	Iterations int `json:"iterations"`  // max generations (default 40)

	// Seed for reproducibility; 0 = random.
	Seed int64 `json:"seed"`
}

// OptimResult is the full result of an optimisation run.
type OptimResult struct {
	// BestParams maps variable names to their optimal values.
	BestParams map[string]float64 `json:"best_params"`

	// BestCost is the scalar objective value at the optimum.
	BestCost float64 `json:"best_cost"`

	// BestMetrics holds the metric values at the optimal design point.
	BestMetrics map[string]float64 `json:"best_metrics"`

	// Convergence[i] is the best cost after generation i (for plotting).
	Convergence []float64 `json:"convergence"`

	// Iterations actually completed.
	Iterations int `json:"iterations"`

	// The optimised wires for direct loading into the editor.
	OptimizedWires []Wire `json:"optimized_wires"`
}

// ──────────────────────────────────────────────────────────────────────
// PSO engine
// ──────────────────────────────────────────────────────────────────────

// particle is a single member of the PSO swarm.
type particle struct {
	pos      []float64 // current position in variable space
	vel      []float64 // current velocity
	bestPos  []float64 // personal best position
	bestCost float64   // personal best objective value
}

// RunOptimizer executes a Particle Swarm Optimization (PSO) loop.
// PSO was chosen because it handles noisy, multimodal antenna objective
// landscapes well, requires no gradient, and is trivially parallelisable.
//
// Algorithm:
//   1. Initialise particles uniformly in [min, max] for each variable.
//   2. For each generation:
//      a. Evaluate objective for every particle (concurrently).
//      b. Update personal bests and global best.
//      c. Update velocities using cognitive + social terms.
//      d. Clamp positions to [min, max].
//   3. Return global best parameters, cost, and convergence history.
func RunOptimizer(req OptimRequest) (*OptimResult, error) {
	nVar := len(req.Variables)
	if nVar == 0 {
		return nil, fmt.Errorf("no variables defined")
	}
	if len(req.Goals) == 0 {
		return nil, fmt.Errorf("no goals defined")
	}

	// Defaults
	nPart := req.Particles
	if nPart <= 0 {
		nPart = 20
	}
	nIter := req.Iterations
	if nIter <= 0 {
		nIter = 40
	}

	// RNG
	var rng *rand.Rand
	if req.Seed != 0 {
		rng = rand.New(rand.NewSource(req.Seed))
	} else {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}

	// PSO hyper-parameters (Clerc constriction coefficients).
	w := 0.729  // inertia weight
	c1 := 1.494 // cognitive acceleration
	c2 := 1.494 // social acceleration

	// Initialise swarm
	swarm := make([]particle, nPart)
	for i := range swarm {
		swarm[i] = particle{
			pos:      make([]float64, nVar),
			vel:      make([]float64, nVar),
			bestPos:  make([]float64, nVar),
			bestCost: math.Inf(1),
		}
		for d := 0; d < nVar; d++ {
			lo := req.Variables[d].Min
			hi := req.Variables[d].Max
			swarm[i].pos[d] = lo + rng.Float64()*(hi-lo)
			swarm[i].vel[d] = (rng.Float64() - 0.5) * (hi - lo) * 0.1
		}
		copy(swarm[i].bestPos, swarm[i].pos)
	}

	globalBest := make([]float64, nVar)
	globalCost := math.Inf(1)
	convergence := make([]float64, 0, nIter)

	// Iteration loop
	for gen := 0; gen < nIter; gen++ {
		// Evaluate all particles concurrently
		costs := make([]float64, nPart)
		metrics := make([]map[string]float64, nPart)
		var wg sync.WaitGroup
		// Limit concurrency to avoid huge memory spikes from parallel Z-matrices.
		sem := make(chan struct{}, 4)

		for i := 0; i < nPart; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				cost, mets := evaluateObjective(req, swarm[idx].pos)
				costs[idx] = cost
				metrics[idx] = mets
			}(i)
		}
		wg.Wait()

		// Update bests
		for i := 0; i < nPart; i++ {
			if costs[i] < swarm[i].bestCost {
				swarm[i].bestCost = costs[i]
				copy(swarm[i].bestPos, swarm[i].pos)
			}
			if costs[i] < globalCost {
				globalCost = costs[i]
				copy(globalBest, swarm[i].pos)
			}
		}
		convergence = append(convergence, globalCost)

		// Update velocities and positions
		for i := 0; i < nPart; i++ {
			for d := 0; d < nVar; d++ {
				r1 := rng.Float64()
				r2 := rng.Float64()
				swarm[i].vel[d] = w*swarm[i].vel[d] +
					c1*r1*(swarm[i].bestPos[d]-swarm[i].pos[d]) +
					c2*r2*(globalBest[d]-swarm[i].pos[d])
				swarm[i].pos[d] += swarm[i].vel[d]
				// Clamp to bounds
				lo := req.Variables[d].Min
				hi := req.Variables[d].Max
				if swarm[i].pos[d] < lo {
					swarm[i].pos[d] = lo
					swarm[i].vel[d] = 0
				}
				if swarm[i].pos[d] > hi {
					swarm[i].pos[d] = hi
					swarm[i].vel[d] = 0
				}
			}
		}
	}

	// Final evaluation at the global best to get metrics
	_, bestMetrics := evaluateObjective(req, globalBest)

	// Build optimised wire set
	optimWires := applyParams(req.Input.Wires, req.Variables, globalBest)

	bestParams := make(map[string]float64, nVar)
	for d, v := range req.Variables {
		bestParams[v.Name] = globalBest[d]
	}

	return &OptimResult{
		BestParams:     bestParams,
		BestCost:       globalCost,
		BestMetrics:    bestMetrics,
		Convergence:    convergence,
		Iterations:     nIter,
		OptimizedWires: optimWires,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────
// Objective evaluation
// ──────────────────────────────────────────────────────────────────────

// evaluateObjective builds an antenna with the given parameter vector,
// runs the solver (single-point or worst-case over a band), and returns
// the scalar cost and per-metric values.
func evaluateObjective(req OptimRequest, params []float64) (float64, map[string]float64) {
	input := req.Input
	input.Wires = applyParams(input.Wires, req.Variables, params)

	// Decide single-freq or band evaluation
	if req.FreqStartHz > 0 && req.FreqEndHz > req.FreqStartHz && req.FreqSteps >= 2 {
		return evaluateBand(input, req.Goals, req.FreqStartHz, req.FreqEndHz, req.FreqSteps)
	}
	return evaluateSingle(input, req.Goals)
}

// evaluateSingle runs a single Simulate() and scores the goals.
func evaluateSingle(input SimulationInput, goals []OptimGoal) (float64, map[string]float64) {
	result, err := Simulate(input)
	if err != nil {
		// Penalise infeasible designs heavily
		return 1e12, nil
	}
	return scoreResult(result, goals)
}

// evaluateBand runs Simulate() at several frequencies across the band
// and returns the worst-case (maximum) cost across the band, so the
// optimiser aims for broadband performance.
func evaluateBand(input SimulationInput, goals []OptimGoal, startHz, endHz float64, steps int) (float64, map[string]float64) {
	if steps > 20 {
		steps = 20 // cap to keep evaluations tractable
	}
	worstCost := 0.0
	var worstMetrics map[string]float64

	for i := 0; i < steps; i++ {
		var f float64
		if steps == 1 {
			f = (startHz + endHz) / 2.0
		} else {
			f = startHz + float64(i)*(endHz-startHz)/float64(steps-1)
		}
		inp := input
		inp.Frequency = f
		result, err := Simulate(inp)
		if err != nil {
			return 1e12, nil
		}
		cost, mets := scoreResult(result, goals)
		if cost > worstCost {
			worstCost = cost
			worstMetrics = mets
		}
	}
	return worstCost, worstMetrics
}

// scoreResult extracts requested metrics from a SolverResult and computes
// the weighted sum of absolute deviations from the targets.
func scoreResult(r *SolverResult, goals []OptimGoal) (float64, map[string]float64) {
	mets := extractMetrics(r)
	cost := 0.0
	for _, g := range goals {
		val, ok := mets[g.Metric]
		if !ok {
			cost += g.Weight * 100 // unknown metric penalty
			continue
		}
		cost += g.Weight * math.Abs(val-g.Target)
	}
	return cost, mets
}

// extractMetrics builds a string→float64 map of all available metrics.
func extractMetrics(r *SolverResult) map[string]float64 {
	return map[string]float64{
		"swr":           r.SWR,
		"gain":          r.GainDBi,
		"front_to_back": r.Metrics.FrontToBackDB,
		"impedance_r":   r.Impedance.R,
		"impedance_x":   r.Impedance.X,
		"efficiency":    r.Metrics.RadiationEfficiency,
		"beamwidth_az":  r.Metrics.BeamwidthAzDeg,
		"beamwidth_el":  r.Metrics.BeamwidthElDeg,
	}
}

// ──────────────────────────────────────────────────────────────────────
// Parameter application
// ──────────────────────────────────────────────────────────────────────

// applyParams creates a modified copy of the wire list with the optimiser's
// parameter vector applied.  Each variable indexes a specific wire and
// field (coordinate or radius).
func applyParams(wires []Wire, vars []OptimVariable, params []float64) []Wire {
	// Deep copy
	out := make([]Wire, len(wires))
	copy(out, wires)

	for i, v := range vars {
		if v.WireIndex < 0 || v.WireIndex >= len(out) {
			continue
		}
		w := &out[v.WireIndex]
		val := params[i]
		switch v.Field {
		case "x1":
			w.X1 = val
		case "y1":
			w.Y1 = val
		case "z1":
			w.Z1 = val
		case "x2":
			w.X2 = val
		case "y2":
			w.Y2 = val
		case "z2":
			w.Z2 = val
		case "radius":
			w.Radius = val
		}
	}
	return out
}

// ──────────────────────────────────────────────────────────────────────
// Convenience: preset Yagi optimisation
// ──────────────────────────────────────────────────────────────────────

// YagiOptimVariables generates a standard set of optimisation variables
// for a 3-element Yagi at the given centre frequency.  It exposes
// the element half-lengths (y2) and spacings (x1/x2) of the reflector,
// driven element, and director as separate tuneable variables.
// The ranges are ±25% around the initial values.
func YagiOptimVariables(wires []Wire) []OptimVariable {
	if len(wires) < 3 {
		return nil
	}

	vars := make([]OptimVariable, 0, 6)
	names := []string{"reflector", "driven", "director"}
	for i := 0; i < 3 && i < len(wires); i++ {
		w := wires[i]
		// Half-length (Y2 for horizontal Yagi)
		halfLen := math.Abs(w.Y2)
		if halfLen > 0 {
			vars = append(vars, OptimVariable{
				Name:      names[i] + "_half_length",
				WireIndex: i,
				Field:     "y2",
				Min:       halfLen * 0.75,
				Max:       halfLen * 1.25,
			})
		}
		// Spacing (X position for horizontal Yagi, skip driven at X=0)
		if i != 1 && w.X1 != 0 {
			vars = append(vars, OptimVariable{
				Name:      names[i] + "_spacing",
				WireIndex: i,
				Field:     "x1",
				Min:       w.X1 * 0.5,
				Max:       w.X1 * 1.5,
			})
		}
	}

	return vars
}

// SortOptimVariablesByImportance orders variables by how much they affect
// the objective.  This is done by running a quick 1-D sensitivity scan
// for each variable.  Returns the sorted variables with additional info.
type VariableSensitivity struct {
	OptimVariable
	Sensitivity float64 `json:"sensitivity"` // Δcost per unit change
}

func AnalyseVariableSensitivity(req OptimRequest) []VariableSensitivity {
	baseParams := make([]float64, len(req.Variables))
	for i, v := range req.Variables {
		baseParams[i] = (v.Min + v.Max) / 2.0 // midpoint
	}
	baseCost, _ := evaluateObjective(req, baseParams)

	results := make([]VariableSensitivity, len(req.Variables))
	for i, v := range req.Variables {
		perturbed := make([]float64, len(baseParams))
		copy(perturbed, baseParams)
		delta := (v.Max - v.Min) * 0.1
		perturbed[i] = baseParams[i] + delta
		pertCost, _ := evaluateObjective(req, perturbed)
		sens := math.Abs(pertCost-baseCost) / delta
		results[i] = VariableSensitivity{
			OptimVariable: v,
			Sensitivity:   sens,
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Sensitivity > results[j].Sensitivity
	})
	return results
}
