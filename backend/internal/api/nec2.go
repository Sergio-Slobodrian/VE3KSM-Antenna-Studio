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
	"bytes"
	"net/http"

	"antenna-studio/backend/internal/mom"
	"antenna-studio/backend/internal/nec2"

	"github.com/gin-gonic/gin"
)

// HandleNEC2Import accepts a NEC-2 deck (text/plain or octet-stream)
// in the POST body and returns the parsed geometry as JSON in the same
// shape as a template-generation result, so the frontend can drop the
// payload straight into the store via loadTemplate.
func HandleNEC2Import(c *gin.Context) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(c.Request.Body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "could not read body: " + err.Error()})
		return
	}
	parsed, err := nec2.Parse(&buf)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "NEC parse failed: " + err.Error()})
		return
	}
	geom, err := nec2.ToGeometry(parsed)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "NEC geometry conversion failed: " + err.Error()})
		return
	}

	type tlOut struct {
		AWireIndex     int     `json:"a_wire_index"`
		ASegmentIndex  int     `json:"a_segment_index"`
		BWireIndex     int     `json:"b_wire_index"`
		BSegmentIndex  int     `json:"b_segment_index"`
		Z0             float64 `json:"z0"`
		Length         float64 `json:"length"`
		VelocityFactor float64 `json:"velocity_factor"`
		LossDbPerM     float64 `json:"loss_db_per_m"`
	}
	type loadOut struct {
		WireIndex    int     `json:"wire_index"`
		SegmentIndex int     `json:"segment_index"`
		Topology     string  `json:"topology"`
		R            float64 `json:"r"`
		L            float64 `json:"l"`
		C            float64 `json:"c"`
	}
	wires := make([]map[string]interface{}, len(geom.Input.Wires))
	for i, w := range geom.Input.Wires {
		wires[i] = map[string]interface{}{
			"x1": w.X1, "y1": w.Y1, "z1": w.Z1,
			"x2": w.X2, "y2": w.Y2, "z2": w.Z2,
			"radius": w.Radius, "segments": w.Segments,
			"material": string(w.Material),
		}
	}
	loads := make([]loadOut, len(geom.Input.Loads))
	for i, l := range geom.Input.Loads {
		loads[i] = loadOut{
			WireIndex: l.WireIndex, SegmentIndex: l.SegmentIndex,
			Topology: string(l.Topology), R: l.R, L: l.L, C: l.C,
		}
	}
	tls := make([]tlOut, len(geom.Input.TransmissionLines))
	for i, tl := range geom.Input.TransmissionLines {
		tls[i] = tlOut{
			AWireIndex: tl.A.WireIndex, ASegmentIndex: tl.A.SegmentIndex,
			BWireIndex: tl.B.WireIndex, BSegmentIndex: tl.B.SegmentIndex,
			Z0: tl.Z0, Length: tl.Length,
			VelocityFactor: tl.VelocityFactor, LossDbPerM: tl.LossDbPerM,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"wires":              wires,
		"loads":              loads,
		"transmission_lines": tls,
		"source": map[string]interface{}{
			"wire_index":    geom.Input.Source.WireIndex,
			"segment_index": geom.Input.Source.SegmentIndex,
			"voltage":       real(geom.Input.Source.Voltage),
		},
		"ground": map[string]interface{}{
			"type":         geom.Input.Ground.Type,
			"conductivity": geom.Input.Ground.Conductivity,
			"permittivity": geom.Input.Ground.Permittivity,
		},
		"frequency": map[string]interface{}{
			"frequency_mhz":  geom.Input.Frequency / 1e6,
			"freq_start_mhz": geom.FreqStartHz / 1e6,
			"freq_end_mhz":   geom.FreqEndHz / 1e6,
			"freq_steps":     geom.FreqSteps,
		},
		"comments":      geom.Comments,
		"ignored_cards": geom.IgnoredCards,
	})
}

// HandleNEC2Export takes a SimulateRequest (or SweepRequest, both share
// the relevant fields) and returns a NEC-2 deck.
func HandleNEC2Export(c *gin.Context) {
	var req SweepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}
	sim := req.ToSimulateRequest()
	if err := sim.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	input := simulateRequestToInput(sim)
	gw := nec2.FromInput(input)

	opts := nec2.WriteOptions{
		Comments: []string{"VE3KSM Antenna Studio export"},
	}
	if req.FreqStart > 0 && req.FreqEnd > req.FreqStart && req.FreqSteps >= 2 {
		opts.FreqStartMHz = req.FreqStart
		opts.FreqStepMHz = (req.FreqEnd - req.FreqStart) / float64(req.FreqSteps-1)
		opts.FreqSteps = req.FreqSteps
	} else {
		opts.FreqStartMHz = input.Frequency / 1e6
		opts.FreqStepMHz = 0
		opts.FreqSteps = 1
	}

	var buf bytes.Buffer
	warnings, err := nec2.Write(&buf, gw, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "NEC export failed: " + err.Error()})
		return
	}
	for _, w := range warnings {
		c.Writer.Header().Add("X-NEC2-Warning", w)
	}
	c.Header("Content-Disposition", `attachment; filename="antenna.nec"`)
	c.Data(http.StatusOK, "text/plain; charset=utf-8", buf.Bytes())
}

// suppress "imported and not used" if mom isn't otherwise referenced
var _ = mom.DefaultReferenceImpedance
