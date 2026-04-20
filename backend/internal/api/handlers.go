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

package api

import (
	"fmt"
	"net/http"

	"antenna-studio/backend/internal/geometry"
	"antenna-studio/backend/internal/mom"

	"github.com/gin-gonic/gin"
)

// HandleSimulate is the Gin handler for POST /api/simulate.
// It runs a single-frequency MoM simulation and returns impedance, SWR,
// gain, far-field pattern, and current distribution.
//
// Request flow: bind JSON -> semantic validation -> convert to solver input -> solve -> respond.
// Returns 400 for invalid input, 500 if the solver fails internally.
func HandleSimulate(c *gin.Context) {
	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	input := simulateRequestToInput(req)
	result, err := mom.Simulate(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "simulation failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, SolverResultToResponse(result))
}

// HandleSweep is the Gin handler for POST /api/sweep.
// It runs the MoM solver at evenly-spaced frequencies between FreqStart and
// FreqEnd, returning parallel arrays of frequency, SWR, and impedance values.
// The geometry is validated once using the start frequency, then the solver
// iterates across the full range. Frequencies in the request are in MHz;
// the solver expects Hz, so they are converted before calling mom.Sweep.
func HandleSweep(c *gin.Context) {
	var req SweepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	// Reuse SimulateRequest.Validate() by converting with the start frequency
	simReq := req.ToSimulateRequest()
	if err := simReq.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	input := simulateRequestToInput(simReq)

	// Convert MHz to Hz for the solver
	freqStartHz := req.FreqStart * 1e6
	freqEndHz := req.FreqEnd * 1e6

	opts := mom.SweepOptions{
		Mode:    mom.SweepMode(req.SweepMode),
		Anchors: req.SweepAnchors,
	}
	result, err := mom.SweepWithOptions(input, freqStartHz, freqEndHz, req.FreqSteps, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "sweep failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, SweepResultToResponse(result))
}

// HandleGetTemplates is the Gin handler for GET /api/templates.
// It returns a JSON array of available antenna preset templates, each with
// its name, description, and configurable parameters. The Generate function
// is excluded from the response (json:"-") since it is not serializable.
// The frontend uses this list to populate the template picker UI.
func HandleGetTemplates(c *gin.Context) {
	templates := geometry.GetTemplates()

	// templateInfo is a local projection that omits the Generate function
	type templateInfo struct {
		Name        string                   `json:"name"`
		Description string                   `json:"description"`
		Parameters  []geometry.TemplateParam `json:"parameters"`
	}

	resp := make([]templateInfo, len(templates))
	for i, t := range templates {
		resp[i] = templateInfo{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}

	c.JSON(http.StatusOK, resp)
}

// HandleGenerateTemplate is the Gin handler for POST /api/templates/:name.
// It looks up the named template, applies the provided parameter overrides
// (or defaults if the body is empty), and returns the generated wire geometry,
// source placement, and ground config. The frontend loads this result directly
// into the 3D editor. Returns 404 if the template name is not recognized.
func HandleGenerateTemplate(c *gin.Context) {
	name := c.Param("name")

	var params map[string]float64
	if err := c.ShouldBindJSON(&params); err != nil {
		// Allow empty body; use defaults
		params = make(map[string]float64)
	}

	templates := geometry.GetTemplates()
	for _, t := range templates {
		if t.Name == name {
			result, err := t.Generate(params)
			if err != nil {
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
				return
			}
			c.JSON(http.StatusOK, result)
			return
		}
	}

	c.JSON(http.StatusNotFound, ErrorResponse{Error: "template not found: " + name})
}

// HandleNearField is the Gin handler for POST /api/nearfield.
// It runs a MoM simulation and then computes the near-field E/H on a
// user-specified observation grid.  The request extends SimulateRequest
// with near-field grid parameters.
func HandleNearField(c *gin.Context) {
	var req NearFieldAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := req.Sim.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Validate near-field grid parameters
	if req.Grid.Steps1 < 2 || req.Grid.Steps2 < 2 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "steps1 and steps2 must be >= 2"})
		return
	}
	if req.Grid.Steps1 > 200 || req.Grid.Steps2 > 200 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "steps1 and steps2 must be <= 200"})
		return
	}
	if req.Grid.Min1 >= req.Grid.Max1 || req.Grid.Min2 >= req.Grid.Max2 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "min must be < max for both axes"})
		return
	}
	plane := req.Grid.Plane
	if plane != "xy" && plane != "xz" && plane != "yz" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "plane must be xy, xz, or yz"})
		return
	}

	input := simulateRequestToInput(req.Sim)

	nfReq := mom.NearFieldRequest{
		Plane:      req.Grid.Plane,
		FixedCoord: req.Grid.FixedCoord,
		Min1:       req.Grid.Min1,
		Max1:       req.Grid.Max1,
		Min2:       req.Grid.Min2,
		Max2:       req.Grid.Max2,
		Steps1:     req.Grid.Steps1,
		Steps2:     req.Grid.Steps2,
	}

	result, err := mom.SimulateNearField(input, nfReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "near-field computation failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleOptimize is the Gin handler for POST /api/optimize.
// It runs a PSO optimisation loop that tunes antenna geometry parameters
// to minimise a user-defined composite objective (SWR, gain, etc.).
// The request bundles the base antenna, tuneable variables with bounds,
// objective goals, optional band evaluation, and PSO hyper-parameters.
func HandleOptimize(c *gin.Context) {
	var req OptimizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := req.Sim.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Validate variables
	validFields := map[string]bool{
		"x1": true, "y1": true, "z1": true,
		"x2": true, "y2": true, "z2": true,
		"radius": true,
	}
	for i, v := range req.Variables {
		if !validFields[v.Field] {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("variable %d: invalid field %q", i, v.Field),
			})
			return
		}
		if v.WireIndex < 0 || v.WireIndex >= len(req.Sim.Wires) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("variable %d: wire_index %d out of range", i, v.WireIndex),
			})
			return
		}
	}

	// Validate goals
	validMetrics := map[string]bool{
		"swr": true, "gain": true, "front_to_back": true,
		"impedance_r": true, "impedance_x": true, "efficiency": true,
		"beamwidth_az": true, "beamwidth_el": true,
	}
	for i, g := range req.Goals {
		if !validMetrics[g.Metric] {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("goal %d: unknown metric %q", i, g.Metric),
			})
			return
		}
	}

	// Cap iterations to prevent runaway compute
	if req.Iterations > 100 {
		req.Iterations = 100
	}
	if req.Particles > 50 {
		req.Particles = 50
	}

	input := simulateRequestToInput(req.Sim)

	// Build solver-level request
	optReq := mom.OptimRequest{
		Input:      input,
		Goals:      make([]mom.OptimGoal, len(req.Goals)),
		Variables:  make([]mom.OptimVariable, len(req.Variables)),
		Particles:  req.Particles,
		Iterations: req.Iterations,
		Seed:       req.Seed,
	}

	if req.FreqStartMHz > 0 && req.FreqEndMHz > req.FreqStartMHz {
		optReq.FreqStartHz = req.FreqStartMHz * 1e6
		optReq.FreqEndHz = req.FreqEndMHz * 1e6
		optReq.FreqSteps = req.FreqSteps
		if optReq.FreqSteps < 2 {
			optReq.FreqSteps = 5
		}
	}

	for i, v := range req.Variables {
		optReq.Variables[i] = mom.OptimVariable{
			Name:      v.Name,
			WireIndex: v.WireIndex,
			Field:     v.Field,
			Min:       v.Min,
			Max:       v.Max,
		}
	}
	for i, g := range req.Goals {
		optReq.Goals[i] = mom.OptimGoal{
			Metric: g.Metric,
			Target: g.Target,
			Weight: g.Weight,
		}
	}

	result, err := mom.RunOptimizer(optReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "optimization failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleCMA is the Gin handler for POST /api/cma.
// It runs a Characteristic Mode Analysis on the antenna structure at the
// specified frequency.  The request body is the same SimulateRequest used by
// /api/simulate (the source field is required by validation but ignored by
// the CMA solver because CMA is source-free by definition).
func HandleCMA(c *gin.Context) {
	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	input := simulateRequestToInput(req)
	result, err := mom.SimulateCMA(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "CMA failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleTransient is the Gin handler for POST /api/transient.
// It runs a frequency sweep across the specified band, then computes
// the time-domain transient response via IFFT for the chosen excitation
// pulse and transfer function (reflection, input voltage, or current).
func HandleTransient(c *gin.Context) {
	var req TransientAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := req.Sim.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Validate pulse type
	validPulses := map[string]bool{
		"":                    true,
		"gaussian":            true,
		"step":                true,
		"modulated_gaussian":  true,
	}
	if !validPulses[req.PulseType] {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("invalid pulse_type %q; must be gaussian, step, or modulated_gaussian", req.PulseType),
		})
		return
	}

	// Validate response type
	validResponses := map[string]bool{
		"":           true,
		"reflection": true,
		"input":      true,
		"current":    true,
	}
	if !validResponses[req.Response] {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("invalid response %q; must be reflection, input, or current", req.Response),
		})
		return
	}

	// Cap num_freqs to prevent excessive computation
	if req.NumFreqs > 512 {
		req.NumFreqs = 512
	}

	input := simulateRequestToInput(req.Sim)

	transReq := mom.TransientRequest{
		Input:        input,
		FreqStartHz:  req.FreqStartMHz * 1e6,
		FreqEndHz:    req.FreqEndMHz * 1e6,
		NumFreqs:     req.NumFreqs,
		PulseType:    req.PulseType,
		PulseWidthNs: req.PulseWidthNs,
		CenterFreqHz: req.CenterFreqMHz * 1e6,
		Response:     req.Response,
	}

	result, err := mom.ComputeTransient(transReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "transient analysis failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleParetoOptimize is the Gin handler for POST /api/pareto-optimize.
// It runs an NSGA-II multi-objective optimization that returns a Pareto
// front of non-dominated trade-off designs.
func HandleParetoOptimize(c *gin.Context) {
	var req ParetoOptimizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := req.Sim.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Validate variables
	validFields := map[string]bool{
		"x1": true, "y1": true, "z1": true,
		"x2": true, "y2": true, "z2": true,
		"radius": true,
	}
	for i, v := range req.Variables {
		if !validFields[v.Field] {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("variable %d: invalid field %q", i, v.Field),
			})
			return
		}
		if v.WireIndex < 0 || v.WireIndex >= len(req.Sim.Wires) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("variable %d: wire_index %d out of range", i, v.WireIndex),
			})
			return
		}
	}

	// Validate objectives
	validMetrics := map[string]bool{
		"swr": true, "gain": true, "front_to_back": true,
		"impedance_r": true, "impedance_x": true, "efficiency": true,
		"beamwidth_az": true, "beamwidth_el": true,
	}
	validDirs := map[string]bool{"minimize": true, "maximize": true}
	for i, obj := range req.Objectives {
		if !validMetrics[obj.Metric] {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("objective %d: unknown metric %q", i, obj.Metric),
			})
			return
		}
		if !validDirs[obj.Direction] {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("objective %d: direction must be 'minimize' or 'maximize'", i),
			})
			return
		}
	}

	// Cap to prevent runaway compute
	if req.Generations > 60 {
		req.Generations = 60
	}
	if req.PopSize > 80 {
		req.PopSize = 80
	}

	input := simulateRequestToInput(req.Sim)

	paretoReq := mom.ParetoRequest{
		Input:       input,
		Variables:   make([]mom.OptimVariable, len(req.Variables)),
		Objectives:  make([]mom.ParetoObjective, len(req.Objectives)),
		PopSize:     req.PopSize,
		Generations: req.Generations,
		Seed:        req.Seed,
	}

	if req.FreqStartMHz > 0 && req.FreqEndMHz > req.FreqStartMHz {
		paretoReq.FreqStartHz = req.FreqStartMHz * 1e6
		paretoReq.FreqEndHz = req.FreqEndMHz * 1e6
		paretoReq.FreqSteps = req.FreqSteps
		if paretoReq.FreqSteps < 2 {
			paretoReq.FreqSteps = 5
		}
	}

	for i, v := range req.Variables {
		paretoReq.Variables[i] = mom.OptimVariable{
			Name:      v.Name,
			WireIndex: v.WireIndex,
			Field:     v.Field,
			Min:       v.Min,
			Max:       v.Max,
		}
	}
	for i, obj := range req.Objectives {
		paretoReq.Objectives[i] = mom.ParetoObjective{
			Metric:    obj.Metric,
			Direction: obj.Direction,
		}
	}

	result, err := mom.RunParetoOptimizer(paretoReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Pareto optimization failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// simulateRequestToInput converts an API request DTO to the MoM solver's
// internal SimulationInput type. This is the API-to-domain boundary.
// It converts frequency from MHz to Hz and maps DTOs to solver structs.
// If the source voltage is zero (omitted from JSON), it defaults to 1V
// so the solver always has a non-zero excitation.
func simulateRequestToInput(req SimulateRequest) mom.SimulationInput {
	wires := make([]mom.Wire, len(req.Wires))
	for i, w := range req.Wires {
		wires[i] = mom.Wire{
			X1:       w.X1,
			Y1:       w.Y1,
			Z1:       w.Z1,
			X2:       w.X2,
			Y2:       w.Y2,
			Z2:       w.Z2,
			Radius:   w.Radius,
			Segments: w.Segments,
			Material:         mom.MaterialName(w.Material),
			CoatingThickness: w.CoatingThickness,
			CoatingEpsR:      w.CoatingEpsR,
			CoatingLossTan:   w.CoatingLossTan,
		}
	}

	// Default to 1V excitation when voltage is omitted or zero
	voltage := complex(req.Source.Voltage, 0)
	if req.Source.Voltage == 0 {
		voltage = 1 + 0i
	}

	loads := make([]mom.Load, len(req.Loads))
	for i, ld := range req.Loads {
		topo := mom.LoadTopology(ld.Topology)
		if topo == "" {
			topo = mom.LoadSeriesRLC
		}
		loads[i] = mom.Load{
			WireIndex:    ld.WireIndex,
			SegmentIndex: ld.SegmentIndex,
			Topology:     topo,
			R:            ld.R,
			L:            ld.L,
			C:            ld.C,
		}
	}

	lines := make([]mom.TransmissionLine, len(req.TransmissionLines))
	for i, tl := range req.TransmissionLines {
		lines[i] = mom.TransmissionLine{
			A: mom.TLEnd{WireIndex: tl.A.WireIndex, SegmentIndex: tl.A.SegmentIndex},
			B: mom.TLEnd{WireIndex: tl.B.WireIndex, SegmentIndex: tl.B.SegmentIndex},
			Z0: tl.Z0, Length: tl.Length,
			VelocityFactor: tl.VelocityFactor, LossDbPerM: tl.LossDbPerM,
		}
	}

	return mom.SimulationInput{
		Wires:     wires,
		Frequency: req.FrequencyMHz * 1e6,
		Ground: mom.GroundConfig{
			Type:           req.Ground.Type,
			Conductivity:   req.Ground.Conductivity,
			Permittivity:   req.Ground.Permittivity,
			MoisturePreset: req.Ground.MoisturePreset,
			RegionPreset:   req.Ground.RegionPreset,
		},
		Source: mom.Source{
			WireIndex:    req.Source.WireIndex,
			SegmentIndex: req.Source.SegmentIndex,
			Voltage:      voltage,
		},
		Loads:              loads,
		TransmissionLines:  lines,
		ReferenceImpedance: req.ReferenceImpedance,
		BasisOrder:         mom.BasisOrder(req.BasisOrder),
		Weather: mom.WeatherConfig{
			Preset:    req.Weather.Preset,
			Thickness: req.Weather.Thickness,
			EpsR:      req.Weather.EpsR,
			LossTan:   req.Weather.LossTan,
		},
	}
}

// HandleConvergence is the Gin handler for POST /api/convergence.
// It runs the MoM solver at the user's segmentation (1x) and again at 2x
// segmentation, then reports the relative change in impedance, SWR, and gain.
// A small delta means the mesh is well-resolved; a large delta means the user
// should increase segment counts.
func HandleConvergence(c *gin.Context) {
	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	input := simulateRequestToInput(req)
	result, err := mom.RunConvergenceCheck(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("convergence check failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, result)
}
