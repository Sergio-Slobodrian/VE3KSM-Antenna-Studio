package api

import (
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
			Material: mom.MaterialName(w.Material),
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
			Type:         req.Ground.Type,
			Conductivity: req.Ground.Conductivity,
			Permittivity: req.Ground.Permittivity,
		},
		Source: mom.Source{
			WireIndex:    req.Source.WireIndex,
			SegmentIndex: req.Source.SegmentIndex,
			Voltage:      voltage,
		},
		Loads:              loads,
		TransmissionLines:  lines,
		ReferenceImpedance: req.ReferenceImpedance,
	}
}
