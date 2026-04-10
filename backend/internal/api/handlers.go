package api

import (
	"net/http"

	"antenna-studio/backend/internal/geometry"
	"antenna-studio/backend/internal/mom"

	"github.com/gin-gonic/gin"
)

// HandleSimulate processes a single-frequency simulation request.
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

// HandleSweep processes a frequency sweep request.
func HandleSweep(c *gin.Context) {
	var req SweepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	simReq := req.ToSimulateRequest()
	if err := simReq.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	input := simulateRequestToInput(simReq)

	freqStartHz := req.FreqStart * 1e6
	freqEndHz := req.FreqEnd * 1e6

	result, err := mom.Sweep(input, freqStartHz, freqEndHz, req.FreqSteps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "sweep failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, SweepResultToResponse(result))
}

// HandleGetTemplates returns all available antenna templates.
func HandleGetTemplates(c *gin.Context) {
	templates := geometry.GetTemplates()

	type templateInfo struct {
		Name        string                    `json:"name"`
		Description string                    `json:"description"`
		Parameters  []geometry.TemplateParam   `json:"parameters"`
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

// HandleGenerateTemplate generates antenna geometry from a named template.
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

// simulateRequestToInput converts an API request DTO to the solver's input type.
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
		}
	}

	voltage := complex(req.Source.Voltage, 0)
	if req.Source.Voltage == 0 {
		voltage = 1 + 0i
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
	}
}
