package mom

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
)

// ──────────────────────────────────────────────────────────────────────
// NSGA-II Pareto multi-objective optimizer
// ──────────────────────────────────────────────────────────────────────
//
// Unlike the single-objective PSO in optimizer.go, this returns a
// *set* of non-dominated trade-off solutions (the Pareto front).
// The user picks the design that best balances competing objectives.
//
// Algorithm (Deb et al., IEEE TEC 2002):
//   1. Initialise random population of size N.
//   2. Evaluate all objectives for each individual.
//   3. For each generation:
//      a. Create offspring via tournament selection, SBX crossover,
//         polynomial mutation.
//      b. Merge parent + offspring (size 2N).
//      c. Non-dominated sort into fronts F0, F1, …
//      d. Fill next generation (size N) front-by-front; when a front
//         would overflow, use crowding-distance to pick the most
//         spread-out individuals.
//   4. Return the final non-dominated front (Pareto front).

// ──────────────────────────────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────────────────────────────

// ParetoObjective defines one objective to minimize or maximize.
// Direction is "minimize" or "maximize"; the NSGA-II engine internally
// converts everything to minimisation.
type ParetoObjective struct {
	Metric    string `json:"metric"`    // same metric names as OptimGoal
	Direction string `json:"direction"` // "minimize" or "maximize"
}

// ParetoRequest is the full Pareto optimization specification.
type ParetoRequest struct {
	Input SimulationInput

	Variables  []OptimVariable   `json:"variables"`
	Objectives []ParetoObjective `json:"objectives"`

	// Optional band evaluation (same semantics as OptimRequest).
	FreqStartHz float64 `json:"freq_start_hz"`
	FreqEndHz   float64 `json:"freq_end_hz"`
	FreqSteps   int     `json:"freq_steps"`

	// NSGA-II hyper-parameters.
	PopSize    int `json:"pop_size"`    // population size (default 40)
	Generations int `json:"generations"` // number of generations (default 30)

	// SBX / mutation parameters.
	CrossoverEta float64 `json:"crossover_eta"` // SBX distribution index (default 20)
	MutationEta  float64 `json:"mutation_eta"`  // polynomial mutation index (default 20)

	Seed int64 `json:"seed"`
}

// ParetoSolution is one design on the Pareto front.
type ParetoSolution struct {
	Params  map[string]float64 `json:"params"`
	Metrics map[string]float64 `json:"metrics"`
	Rank    int                `json:"rank"`
	Wires   []Wire             `json:"wires"`
}

// ParetoResult is the complete output of a Pareto optimization run.
type ParetoResult struct {
	Front       []ParetoSolution `json:"front"`
	AllFronts   []ParetoSolution `json:"all_fronts"`   // all ranked individuals for visualization
	Generations int              `json:"generations"`
	Objectives  []string         `json:"objectives"`    // objective names in order
}

// ──────────────────────────────────────────────────────────────────────
// Internal types
// ──────────────────────────────────────────────────────────────────────

type individual struct {
	params   []float64
	objVals  []float64            // objective values (all minimised internally)
	metrics  map[string]float64   // raw metric values
	rank     int                  // non-domination rank (0 = Pareto front)
	crowding float64              // crowding distance
}

// ──────────────────────────────────────────────────────────────────────
// Engine
// ──────────────────────────────────────────────────────────────────────

// RunParetoOptimizer executes the NSGA-II loop and returns the Pareto front.
func RunParetoOptimizer(req ParetoRequest) (*ParetoResult, error) {
	nVar := len(req.Variables)
	nObj := len(req.Objectives)
	if nVar == 0 {
		return nil, fmt.Errorf("no variables defined")
	}
	if nObj < 2 {
		return nil, fmt.Errorf("Pareto optimization requires at least 2 objectives")
	}

	// Defaults
	popSize := req.PopSize
	if popSize <= 0 {
		popSize = 40
	}
	// Ensure even population for crossover pairing
	if popSize%2 != 0 {
		popSize++
	}
	nGen := req.Generations
	if nGen <= 0 {
		nGen = 30
	}
	etaC := req.CrossoverEta
	if etaC <= 0 {
		etaC = 20.0
	}
	etaM := req.MutationEta
	if etaM <= 0 {
		etaM = 20.0
	}

	// Determine which objectives to maximize (we negate them internally).
	maximize := make([]bool, nObj)
	for i, obj := range req.Objectives {
		maximize[i] = (obj.Direction == "maximize")
	}

	// RNG
	var rng *rand.Rand
	if req.Seed != 0 {
		rng = rand.New(rand.NewSource(req.Seed))
	} else {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}

	// ── 1. Initialise population ──
	pop := make([]individual, popSize)
	for i := range pop {
		pop[i].params = make([]float64, nVar)
		for d := 0; d < nVar; d++ {
			lo := req.Variables[d].Min
			hi := req.Variables[d].Max
			pop[i].params[d] = lo + rng.Float64()*(hi-lo)
		}
	}

	// ── 2. Evaluate initial population ──
	evaluatePopulation(pop, req, maximize)

	// ── 3. NSGA-II generational loop ──
	for gen := 0; gen < nGen; gen++ {
		// a. Non-dominated sort + crowding on current pop
		nonDominatedSort(pop)
		assignCrowding(pop, nObj)

		// b. Create offspring via tournament + SBX + mutation
		offspring := make([]individual, popSize)
		for i := 0; i < popSize; i += 2 {
			p1 := tournamentSelect(pop, rng)
			p2 := tournamentSelect(pop, rng)
			c1, c2 := sbxCrossover(p1.params, p2.params, req.Variables, etaC, rng)
			polyMutate(c1, req.Variables, etaM, rng)
			polyMutate(c2, req.Variables, etaM, rng)
			offspring[i] = individual{params: c1}
			if i+1 < popSize {
				offspring[i+1] = individual{params: c2}
			}
		}

		// c. Evaluate offspring
		evaluatePopulation(offspring, req, maximize)

		// d. Merge parent + offspring (size 2N)
		merged := make([]individual, 0, 2*popSize)
		merged = append(merged, pop...)
		merged = append(merged, offspring...)

		// e. Non-dominated sort on merged
		nonDominatedSort(merged)
		assignCrowding(merged, nObj)

		// f. Select next generation (size N) front-by-front
		pop = selectNextGeneration(merged, popSize)
	}

	// ── 4. Final sort and extract Pareto front ──
	nonDominatedSort(pop)
	assignCrowding(pop, nObj)

	// Build result
	objNames := make([]string, nObj)
	for i, obj := range req.Objectives {
		objNames[i] = obj.Metric
	}

	var front []ParetoSolution
	var allFronts []ParetoSolution
	for _, ind := range pop {
		sol := individualToSolution(ind, req, maximize)
		allFronts = append(allFronts, sol)
		if ind.rank == 0 {
			front = append(front, sol)
		}
	}

	// Sort front by first objective for clean visualization
	sort.Slice(front, func(i, j int) bool {
		return front[i].Metrics[objNames[0]] < front[j].Metrics[objNames[0]]
	})

	return &ParetoResult{
		Front:       front,
		AllFronts:   allFronts,
		Generations: nGen,
		Objectives:  objNames,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────
// Evaluation
// ──────────────────────────────────────────────────────────────────────

func evaluatePopulation(pop []individual, req ParetoRequest, maximize []bool) {
	nObj := len(req.Objectives)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // limit concurrency

	for i := range pop {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			metrics := evaluatePareto(req, pop[idx].params)
			pop[idx].metrics = metrics
			pop[idx].objVals = make([]float64, nObj)
			for j, obj := range req.Objectives {
				val := metrics[obj.Metric]
				if maximize[j] {
					val = -val // negate for internal minimisation
				}
				pop[idx].objVals[j] = val
			}
		}(i)
	}
	wg.Wait()
}

func evaluatePareto(req ParetoRequest, params []float64) map[string]float64 {
	input := req.Input
	input.Wires = applyParams(input.Wires, req.Variables, params)

	if req.FreqStartHz > 0 && req.FreqEndHz > req.FreqStartHz && req.FreqSteps >= 2 {
		return evaluateParetoBand(input, req.Objectives, req.FreqStartHz, req.FreqEndHz, req.FreqSteps)
	}

	result, err := Simulate(input)
	if err != nil {
		// Return penalty values
		m := make(map[string]float64)
		for _, obj := range req.Objectives {
			if obj.Direction == "maximize" {
				m[obj.Metric] = -1e6
			} else {
				m[obj.Metric] = 1e6
			}
		}
		return m
	}
	return extractMetrics(result)
}

func evaluateParetoBand(input SimulationInput, objectives []ParetoObjective, startHz, endHz float64, steps int) map[string]float64 {
	if steps > 20 {
		steps = 20
	}

	// For each metric, track worst-case across the band.
	// "Worst" depends on direction: for minimize, worst = max; for maximize, worst = min.
	var allMetrics []map[string]float64
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
			m := make(map[string]float64)
			for _, obj := range objectives {
				if obj.Direction == "maximize" {
					m[obj.Metric] = -1e6
				} else {
					m[obj.Metric] = 1e6
				}
			}
			return m
		}
		allMetrics = append(allMetrics, extractMetrics(result))
	}

	// Take worst-case per objective across the band
	worst := make(map[string]float64)
	for _, obj := range objectives {
		if obj.Direction == "maximize" {
			worst[obj.Metric] = math.Inf(1)
			for _, m := range allMetrics {
				if m[obj.Metric] < worst[obj.Metric] {
					worst[obj.Metric] = m[obj.Metric]
				}
			}
		} else {
			worst[obj.Metric] = math.Inf(-1)
			for _, m := range allMetrics {
				if m[obj.Metric] > worst[obj.Metric] {
					worst[obj.Metric] = m[obj.Metric]
				}
			}
		}
	}
	return worst
}

// ──────────────────────────────────────────────────────────────────────
// Non-dominated sorting (fast algorithm from Deb et al. 2002)
// ──────────────────────────────────────────────────────────────────────

func dominates(a, b []float64) bool {
	anyBetter := false
	for i := range a {
		if a[i] > b[i] {
			return false
		}
		if a[i] < b[i] {
			anyBetter = true
		}
	}
	return anyBetter
}

func nonDominatedSort(pop []individual) {
	n := len(pop)
	domCount := make([]int, n)        // number of individuals dominating i
	dominated := make([][]int, n)     // indices of individuals dominated by i

	var front0 []int

	for i := 0; i < n; i++ {
		dominated[i] = nil
		domCount[i] = 0
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			if dominates(pop[i].objVals, pop[j].objVals) {
				dominated[i] = append(dominated[i], j)
			} else if dominates(pop[j].objVals, pop[i].objVals) {
				domCount[i]++
			}
		}
		if domCount[i] == 0 {
			pop[i].rank = 0
			front0 = append(front0, i)
		}
	}

	rank := 0
	currentFront := front0
	for len(currentFront) > 0 {
		var nextFront []int
		for _, i := range currentFront {
			for _, j := range dominated[i] {
				domCount[j]--
				if domCount[j] == 0 {
					pop[j].rank = rank + 1
					nextFront = append(nextFront, j)
				}
			}
		}
		rank++
		currentFront = nextFront
	}
}

// ──────────────────────────────────────────────────────────────────────
// Crowding distance
// ──────────────────────────────────────────────────────────────────────

func assignCrowding(pop []individual, nObj int) {
	for i := range pop {
		pop[i].crowding = 0
	}

	// Group by rank
	rankMap := make(map[int][]int)
	for i, ind := range pop {
		rankMap[ind.rank] = append(rankMap[ind.rank], i)
	}

	for _, indices := range rankMap {
		if len(indices) <= 2 {
			for _, idx := range indices {
				pop[idx].crowding = math.Inf(1)
			}
			continue
		}

		for m := 0; m < nObj; m++ {
			// Sort indices by objective m
			sort.Slice(indices, func(i, j int) bool {
				return pop[indices[i]].objVals[m] < pop[indices[j]].objVals[m]
			})

			// Boundary points get infinite crowding
			pop[indices[0]].crowding = math.Inf(1)
			pop[indices[len(indices)-1]].crowding = math.Inf(1)

			fMin := pop[indices[0]].objVals[m]
			fMax := pop[indices[len(indices)-1]].objVals[m]
			span := fMax - fMin
			if span < 1e-30 {
				continue
			}

			for k := 1; k < len(indices)-1; k++ {
				pop[indices[k]].crowding += (pop[indices[k+1]].objVals[m] - pop[indices[k-1]].objVals[m]) / span
			}
		}
	}
}

// ──────────────────────────────────────────────────────────────────────
// Tournament selection
// ──────────────────────────────────────────────────────────────────────

// Binary tournament: pick 2 random, prefer lower rank; tie-break by crowding.
func tournamentSelect(pop []individual, rng *rand.Rand) individual {
	i := rng.Intn(len(pop))
	j := rng.Intn(len(pop))
	if pop[i].rank < pop[j].rank {
		return pop[i]
	}
	if pop[j].rank < pop[i].rank {
		return pop[j]
	}
	// Same rank — pick higher crowding distance
	if pop[i].crowding > pop[j].crowding {
		return pop[i]
	}
	return pop[j]
}

// ──────────────────────────────────────────────────────────────────────
// SBX crossover (Simulated Binary Crossover)
// ──────────────────────────────────────────────────────────────────────

func sbxCrossover(p1, p2 []float64, vars []OptimVariable, eta float64, rng *rand.Rand) ([]float64, []float64) {
	n := len(p1)
	c1 := make([]float64, n)
	c2 := make([]float64, n)

	for i := 0; i < n; i++ {
		if rng.Float64() < 0.5 {
			// Crossover on this variable
			if math.Abs(p1[i]-p2[i]) < 1e-14 {
				c1[i] = p1[i]
				c2[i] = p2[i]
				continue
			}

			lo := vars[i].Min
			hi := vars[i].Max

			x1 := math.Min(p1[i], p2[i])
			x2 := math.Max(p1[i], p2[i])
			diff := x2 - x1

			// Beta for lower bound
			beta1 := 1.0 + 2.0*(x1-lo)/diff
			alpha1 := 2.0 - math.Pow(beta1, -(eta+1.0))
			betaq1 := sbxBetaq(rng.Float64(), alpha1, eta)

			// Beta for upper bound
			beta2 := 1.0 + 2.0*(hi-x2)/diff
			alpha2 := 2.0 - math.Pow(beta2, -(eta+1.0))
			betaq2 := sbxBetaq(rng.Float64(), alpha2, eta)

			c1[i] = 0.5 * ((x1 + x2) - betaq1*diff)
			c2[i] = 0.5 * ((x1 + x2) + betaq2*diff)

			// Clamp
			c1[i] = math.Max(lo, math.Min(hi, c1[i]))
			c2[i] = math.Max(lo, math.Min(hi, c2[i]))
		} else {
			c1[i] = p1[i]
			c2[i] = p2[i]
		}
	}
	return c1, c2
}

func sbxBetaq(u, alpha, eta float64) float64 {
	if u <= 1.0/alpha {
		return math.Pow(u*alpha, 1.0/(eta+1.0))
	}
	return math.Pow(1.0/(2.0-u*alpha), 1.0/(eta+1.0))
}

// ──────────────────────────────────────────────────────────────────────
// Polynomial mutation
// ──────────────────────────────────────────────────────────────────────

func polyMutate(x []float64, vars []OptimVariable, eta float64, rng *rand.Rand) {
	pm := 1.0 / float64(len(x)) // per-variable mutation probability
	for i := range x {
		if rng.Float64() >= pm {
			continue
		}
		lo := vars[i].Min
		hi := vars[i].Max
		delta := hi - lo
		if delta < 1e-30 {
			continue
		}

		u := rng.Float64()
		var deltaq float64
		if u < 0.5 {
			xy := 1.0 - (x[i]-lo)/delta
			val := 2.0*u + (1.0-2.0*u)*math.Pow(xy, eta+1.0)
			deltaq = math.Pow(val, 1.0/(eta+1.0)) - 1.0
		} else {
			xy := 1.0 - (hi-x[i])/delta
			val := 2.0*(1.0-u) + 2.0*(u-0.5)*math.Pow(xy, eta+1.0)
			deltaq = 1.0 - math.Pow(val, 1.0/(eta+1.0))
		}
		x[i] += deltaq * delta
		x[i] = math.Max(lo, math.Min(hi, x[i]))
	}
}

// ──────────────────────────────────────────────────────────────────────
// Selection for next generation
// ──────────────────────────────────────────────────────────────────────

func selectNextGeneration(merged []individual, popSize int) []individual {
	// Group by rank
	maxRank := 0
	for _, ind := range merged {
		if ind.rank > maxRank {
			maxRank = ind.rank
		}
	}

	next := make([]individual, 0, popSize)
	for r := 0; r <= maxRank; r++ {
		var front []individual
		for _, ind := range merged {
			if ind.rank == r {
				front = append(front, ind)
			}
		}
		if len(next)+len(front) <= popSize {
			next = append(next, front...)
		} else {
			// This front would overflow; sort by crowding distance (descending)
			// and take only enough to fill.
			sort.Slice(front, func(i, j int) bool {
				return front[i].crowding > front[j].crowding
			})
			remaining := popSize - len(next)
			next = append(next, front[:remaining]...)
			break
		}
	}
	return next
}

// ──────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────

func individualToSolution(ind individual, req ParetoRequest, maximize []bool) ParetoSolution {
	params := make(map[string]float64, len(req.Variables))
	for d, v := range req.Variables {
		params[v.Name] = ind.params[d]
	}

	// Metrics are raw (un-negated) values
	metrics := make(map[string]float64)
	for k, v := range ind.metrics {
		metrics[k] = v
	}

	wires := applyParams(req.Input.Wires, req.Variables, ind.params)

	return ParetoSolution{
		Params:  params,
		Metrics: metrics,
		Rank:    ind.rank,
		Wires:   wires,
	}
}
