package match

import (
	"fmt"
	"math"
	"math/cmplx"
)

// designLNetwork solves Y_total = Y_load + jB_shunt then Z_after = Z_total +
// jX_series so the chain ends at Z0 + 0j.  Two cases by R_load vs Z0:
// step-down (R_load > Z0) puts the shunt on the load side, the series on the
// source side; step-up swaps them.  The formulas below are the
// admittance / impedance derivation, fully accounting for reactive loads.
func designLNetwork(req Request) (Solution, error) {
	sol := Solution{Topology: "L"}
	if req.LoadR <= 0 {
		return sol, fmt.Errorf("L-network needs LoadR > 0")
	}
	z0 := req.SourceZ0
	rl := req.LoadR
	xl := req.LoadX

	if rl >= z0 {
		// Step-down: shunt on load side, series on source side.
		denom := rl*rl + xl*xl
		gL := rl / denom    // Re(Y_load)
		bL := -xl / denom   // Im(Y_load)
		btotSq := gL/z0 - gL*gL
		if btotSq < 0 {
			return sol, fmt.Errorf("L-network step-down infeasible: load conductance %g exceeds 1/Z0 %g", gL, 1/z0)
		}
		// Two solutions; pick the capacitive shunt (positive susceptance
		// branch) which is usually preferred at HF/VHF.
		bTot := math.Sqrt(btotSq)
		bShunt := bTot - bL
		xShunt := -1 / bShunt
		// After the shunt, Y_total = gL + j bTot, so
		// Z_total = (gL - j bTot) / (gL^2 + bTot^2); the series element
		// must cancel its imaginary part.
		zMag2 := gL*gL + bTot*bTot
		xAfter := -bTot / zMag2
		xSeries := -xAfter // make total imag = 0

		sol.Components = []Component{
			componentFromX(xSeries, req.FreqHz, "series", "series arm (source side)"),
			componentFromX(xShunt, req.FreqHz, "shunt", "shunt arm (load side)"),
		}
		Q := math.Sqrt(rl/z0 - 1)
		sol.Notes = fmt.Sprintf("step-down L; Q = %.2f", Q)
		return sol, nil
	}

	// Step-up: shunt on source side, series on load side.  Apply the
	// series element first to lift the resistance, then the shunt to
	// cancel the remaining reactance.
	rsTarget := z0
	// Choose X_series so that the resulting Re(Y) equals 1/rsTarget.
	// (rl + j(xl + xSeries)) → Y = (rl - j(xl+xSeries)) / (rl^2+(xl+xSeries)^2)
	// Re(Y) = rl / (rl^2 + (xl+xSeries)^2) = 1/rsTarget
	// => (xl + xSeries)^2 = rl*rsTarget - rl^2
	disc := rl*rsTarget - rl*rl
	if disc < 0 {
		return sol, fmt.Errorf("L-network step-up infeasible (rl*z0 < rl^2)")
	}
	root := math.Sqrt(disc)
	// Two solutions: prefer the one whose result is reasonable; default to the
	// positive root (inductive series element).
	xSeries := root - xl
	// The Y after series:
	xTot := xl + xSeries
	denom := rl*rl + xTot*xTot
	bAfter := -xTot / denom
	_ = denom // also used implicitly via bAfter
	// Now we want B_shunt so that resulting Y has Im = 0:
	bShunt := -bAfter
	xShunt := -1 / bShunt

	sol.Components = []Component{
		componentFromX(xShunt, req.FreqHz, "shunt", "shunt arm (source side)"),
		componentFromX(xSeries, req.FreqHz, "series", "series arm (load side)"),
	}
	Q := math.Sqrt(z0/rl - 1)
	sol.Notes = fmt.Sprintf("step-up L; Q = %.2f", Q)
	return sol, nil
}

// designPiNetwork returns a 3-element pi (CLC or LCL) matching network
// with user-specified loaded Q.  Topology: shunt-series-shunt.
//
// Reference: ARRL Handbook chapter on impedance matching.  Closed-form
// derivation:
//   R_virtual = max(R_in, R_out) / (1 + Q^2)
//   X_C1 = R_in / Q
//   X_C2 = R_out / sqrt(R_out * (1 + Q^2) / R_virtual - 1)
//   X_L  = (Q*R_in + R_in*R_out / X_C2) / (1 + Q^2)
func designPiNetwork(req Request) (Solution, error) {
	sol := Solution{Topology: "pi"}
	if req.LoadR <= 0 {
		return sol, fmt.Errorf("pi-network needs LoadR > 0")
	}
	if req.LoadX != 0 {
		// Treat reactive load by absorbing X_load into the load-side shunt.
		// For a quick design we collapse onto the resistive part and warn.
		sol.Notes = "load reactance ignored for pi design; absorb with a series stub"
	}
	Q := req.QFactor
	z0 := req.SourceZ0
	rl := req.LoadR

	// Choose the higher resistance as R_in (NEC/ARRL convention).
	rIn, rOut := z0, rl
	if rOut > rIn {
		rIn, rOut = rOut, rIn
	}
	rv := rIn / (1 + Q*Q)
	if rOut <= rv {
		return sol, fmt.Errorf("pi: Q=%.1f too low for this transform; need Q > %.2f", Q, math.Sqrt(rIn/rOut-1))
	}
	xC1 := rIn / Q
	xC2 := rOut / math.Sqrt(rOut/rv-1)
	xL := (Q*rIn + rIn*rOut/xC2) / (1 + Q*Q)

	sol.Components = []Component{
		componentFromX(-xC1, req.FreqHz, "shunt", "C1 (source-side shunt)"),
		componentFromX(xL, req.FreqHz, "series", "L (series)"),
		componentFromX(-xC2, req.FreqHz, "shunt", "C2 (load-side shunt)"),
	}
	if sol.Notes == "" {
		sol.Notes = fmt.Sprintf("Q = %.1f", Q)
	}
	return sol, nil
}

// designTNetwork returns a 3-element T network (LCL or CLC).
// Topology: series-shunt-series.  Dual of pi.
//
// Closed-form: X_L1 = Q * R_in, X_L2 = X-of-equivalent-output,
// X_C  = (R_in*Q + R_out*X_L2/X_L1) / (1 + Q^2-equivalent)... use the
// standard transformer-equivalent treatment.
func designTNetwork(req Request) (Solution, error) {
	sol := Solution{Topology: "T"}
	if req.LoadR <= 0 {
		return sol, fmt.Errorf("T-network needs LoadR > 0")
	}
	Q := req.QFactor
	z0 := req.SourceZ0
	rl := req.LoadR

	rLow, rHigh := z0, rl
	if rHigh < rLow {
		rLow, rHigh = rHigh, rLow
	}
	rv := rLow * (1 + Q*Q)
	if rv <= rHigh {
		return sol, fmt.Errorf("T: Q=%.1f too low; need Q > %.2f", Q, math.Sqrt(rHigh/rLow-1))
	}
	xL1 := Q * z0
	xL2 := math.Sqrt(rl*(rv-rl)) // X seen at load side
	xC := rv / Q
	if rl < z0 {
		xL1, xL2 = xL2, xL1
	}
	sol.Components = []Component{
		componentFromX(xL1, req.FreqHz, "series", "L1 (source-side series)"),
		componentFromX(-xC, req.FreqHz, "shunt", "C (shunt)"),
		componentFromX(xL2, req.FreqHz, "series", "L2 (load-side series)"),
	}
	sol.Notes = fmt.Sprintf("Q = %.1f", Q)
	if req.LoadX != 0 {
		sol.Notes += "; load reactance ignored, absorb with a stub"
	}
	return sol, nil
}

// designGammaMatch returns the classical Yagi gamma match: a shorted
// stub from the boom + a series capacitor.  Closed-form due to ARRL
// Handbook (Healey, "Gamma Matching"):
//
//   X_C_series = -Z0 * sqrt(R_load / Z0 - 1)
//   X_stub     = Z0 * sqrt(R_load / Z0 - 1) (inductive)
//
// Only valid when R_load > Z0; reactive part of load is absorbed into
// the stub length adjustment.
func designGammaMatch(req Request) (Solution, error) {
	sol := Solution{Topology: "gamma"}
	if req.LoadR <= req.SourceZ0 {
		return sol, fmt.Errorf("gamma match requires LoadR > SourceZ0; got %.1f vs %.1f", req.LoadR, req.SourceZ0)
	}
	z0 := req.SourceZ0
	rl := req.LoadR
	Q := math.Sqrt(rl/z0 - 1)
	xC := -z0 * Q       // series capacitor (negative reactance)
	xStub := z0 * Q     // inductive shorted stub
	// Adjust stub for load reactance.
	xStub -= req.LoadX

	sol.Components = []Component{
		componentFromX(xC, req.FreqHz, "series", "Cgamma (series cap)"),
		componentFromX(xStub, req.FreqHz, "shunt", "Lgamma (gamma rod / shorted stub)"),
	}
	sol.Notes = fmt.Sprintf("Q = %.2f; physical rod length depends on conductor spacing", Q)
	return sol, nil
}

// designBetaMatch returns the hairpin / beta match: a shunt inductor
// across the feed plus a series capacitor.  Used on driven elements
// shortened below resonance to remove the residual capacitive
// reactance.
//
// Convention: load is capacitive (X_load < 0).  Beta inductor cancels
// the capacitive part; the result is an L-net step-up to Z0.
func designBetaMatch(req Request) (Solution, error) {
	sol := Solution{Topology: "beta"}
	if req.LoadX >= 0 {
		return sol, fmt.Errorf("beta match expects a capacitive load (LoadX < 0); got X = %g", req.LoadX)
	}
	z0 := req.SourceZ0
	rl := req.LoadR
	xl := req.LoadX
	// Y_load = 1 / (R + jX)
	yl := 1 / complex(rl, xl)
	gl := real(yl)
	bl := imag(yl)
	if gl <= 0 || gl > 1/z0 {
		return sol, fmt.Errorf("beta match cannot transform Re(Y_load)=%g to 1/Z0=%g", gl, 1/z0)
	}
	// We need the shunt susceptance to take Y_load to 1/Z0 - jB' where
	// B' is removed by a series cap.
	bAdded := math.Sqrt(gl*(1/z0)-gl*gl) - bl
	xShunt := -1 / bAdded // shunt inductor susceptance is negative
	// Series cap then cancels the residual reactance of the parallel
	// combination.
	yPar := complex(gl, bl+bAdded)
	zPar := 1 / yPar
	xSeries := -imag(zPar)

	sol.Components = []Component{
		componentFromX(xShunt, req.FreqHz, "shunt", "Lbeta (hairpin)"),
		componentFromX(xSeries, req.FreqHz, "series", "Cbeta (series cap)"),
	}
	_ = cmplx.Abs(zPar)
	sol.Notes = "hairpin loop physical length follows from L = X/(2πf)"
	return sol, nil
}
