package api

import (
	"net/http"

	"antenna-studio/backend/internal/match"

	"github.com/gin-gonic/gin"
)

// MatchRequestDTO is the JSON body for POST /api/match.
type MatchRequestDTO struct {
	LoadR    float64 `json:"load_r" binding:"gte=0"`
	LoadX    float64 `json:"load_x"`
	SourceZ0 float64 `json:"source_z0"`
	FreqMHz  float64 `json:"freq_mhz" binding:"required,gt=0"`
	QFactor  float64 `json:"q_factor"`
}

// HandleMatch returns matching-network designs for L, pi, T, gamma,
// and beta topologies given a load impedance and source Z0.
func HandleMatch(c *gin.Context) {
	var req MatchRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}
	r, err := match.All(match.Request{
		LoadR:    req.LoadR,
		LoadX:    req.LoadX,
		SourceZ0: req.SourceZ0,
		FreqHz:   req.FreqMHz * 1e6,
		QFactor:  req.QFactor,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, r)
}
