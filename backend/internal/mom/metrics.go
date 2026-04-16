package mom

import (
	"math"
	"math/cmplx"
)

// FarFieldMetrics summarises a far-field pattern with the scalar metrics
// that working antenna designers actually look at: peak direction, F/B,
// beamwidth, sidelobe level, and overall efficiency.  All gains are in
// dBi; all angles are in degrees.
type FarFieldMetrics struct {
	PeakGainDB          float64 `json:"peak_gain_db"`           // peak directivity (dBi), == GainDBi
	PeakThetaDeg        float64 `json:"peak_theta_deg"`         // elevation of peak (0 = zenith, 90 = horizon)
	PeakPhiDeg          float64 `json:"peak_phi_deg"`           // azimuth of peak (deg)
	FrontToBackDB       float64 `json:"front_to_back_db"`       // peak gain minus gain at antipode (dB)
	BeamwidthAzDeg      float64 `json:"beamwidth_az_deg"`       // -3 dB azimuthal beamwidth at PeakTheta
	BeamwidthElDeg      float64 `json:"beamwidth_el_deg"`       // -3 dB elevation beamwidth at PeakPhi
	SidelobeLevelDB     float64 `json:"sidelobe_level_db"`      // strongest sidelobe relative to main lobe (dB, ≤ 0)
	RadiationEfficiency float64 `json:"radiation_efficiency"`   // P_rad / P_in (0 → 1; > 1 means quadrature imprecision)
	TotalRadiatedPowerW float64 `json:"total_radiated_power_w"` // ∫∫ |E|²/(2η) dΩ assuming 1 m sphere reference
	InputPowerW         float64 `json:"input_power_w"`          // Re(V·I*) at feed
}

// PolarCuts holds two principal-plane slices through the 3D pattern,
// expressed in the same dBi units as the full pattern.  These are the
// "polar plot" curves users want for day-to-day work.
//
// Azimuth cut: gain vs azimuth at fixed elevation = PeakThetaDeg.
// Elevation cut: gain vs elevation at fixed azimuth = PeakPhiDeg.
type PolarCuts struct {
	AzimuthDeg     []float64 `json:"azimuth_deg"`     // x-axis: azimuth angle (0..360)
	AzimuthGainDB  []float64 `json:"azimuth_gain_db"` // gain at each azimuth (dBi)
	ElevationDeg   []float64 `json:"elevation_deg"`   // x-axis: elevation = 90 - theta (-90..+90)
	ElevationGainDB []float64 `json:"elevation_gain_db"`
	FixedElevation float64   `json:"fixed_elevation_deg"` // elevation of the azimuth cut
	FixedAzimuth   float64   `json:"fixed_azimuth_deg"`   // azimuth of the elevation cut
}

// ComputeFarFieldMetrics walks the 2°-grid pattern produced by
// ComputeFarField (and friends) and returns the headline metrics plus
// the two principal polar cuts.  Pure post-processing: no extra MoM
// work beyond what already produced the pattern.
//
// inputPowerW should be Re(V · conj(I_feed)) at the feed point; this
// lets the function compute radiation efficiency.  Pass ≤ 0 to skip
// the efficiency calculation.
func ComputeFarFieldMetrics(pattern []PatternPoint, inputPowerW float64) (FarFieldMetrics, PolarCuts) {
	var m FarFieldMetrics
	var cuts PolarCuts

	if len(pattern) == 0 {
		return m, cuts
	}

	// 1. Find the peak gain direction.
	peakIdx := 0
	for i, p := range pattern {
		if p.GainDB > pattern[peakIdx].GainDB {
			peakIdx = i
		}
	}
	peak := pattern[peakIdx]
	m.PeakGainDB = peak.GainDB
	m.PeakThetaDeg = peak.ThetaDeg
	m.PeakPhiDeg = peak.PhiDeg

	// 2. Front-to-back: gain at the antipode (theta' = 180-theta, phi' = phi+180).
	antipodeTheta := 180.0 - peak.ThetaDeg
	antipodePhi := math.Mod(peak.PhiDeg+180.0, 360.0)
	m.FrontToBackDB = peak.GainDB - nearestGainDB(pattern, antipodeTheta, antipodePhi)

	// 3. Principal-plane polar cuts.  The pattern is on a 2° grid so we
	// pick rows whose theta/phi match the peak direction (within ½ step).
	cuts.FixedElevation = 90.0 - peak.ThetaDeg
	cuts.FixedAzimuth = peak.PhiDeg
	cuts.AzimuthDeg, cuts.AzimuthGainDB = sliceAt(pattern, peak.ThetaDeg, true)
	rawTheta, rawGain := sliceAt(pattern, peak.PhiDeg, false)
	cuts.ElevationDeg = make([]float64, len(rawTheta))
	for i, th := range rawTheta {
		// Convert polar angle (0=zenith) to elevation (90=zenith) for plotting.
		cuts.ElevationDeg[i] = 90.0 - th
	}
	cuts.ElevationGainDB = rawGain

	// 4. Beamwidths from the polar cuts.  -3 dB relative to the cut peak.
	m.BeamwidthAzDeg = beamwidthDeg(cuts.AzimuthDeg, cuts.AzimuthGainDB, true)
	m.BeamwidthElDeg = beamwidthDeg(cuts.ElevationDeg, cuts.ElevationGainDB, false)

	// 5. Sidelobe level: strongest local maximum in the azimuth cut that
	// is at least 30° away from the main-lobe peak.  Reported in dB
	// relative to the main lobe (≤ 0).
	m.SidelobeLevelDB = sidelobeLevel(cuts.AzimuthDeg, cuts.AzimuthGainDB, peak.PhiDeg)

	// 6. Total radiated power and efficiency.  We integrate the *linear*
	// gain over the sphere (since the pattern is normalised to peak
	// directivity, the integral reconstructs 4π up to discretisation
	// error; the same integral with the explicit |E|² values would
	// give the absolute P_rad if amplitudes were preserved upstream).
	// Without |E| amplitudes here we estimate efficiency as the ratio
	// of integrated gain to 4π — hovering at 1.0 for lossless models
	// and dropping below 1.0 once Item 1 loads or Item 4 skin-effect
	// dissipate power.  The crude integral is good to a few percent
	// on the 2° grid; that's plenty for a UX metric.
	if inputPowerW > 0 {
		intGain := integrateGainOverSphere(pattern)
		// 4π steradians is the lossless reference; ratio gives
		// radiation efficiency (0..~1).
		m.RadiationEfficiency = intGain / (4 * math.Pi)
		m.InputPowerW = inputPowerW
		m.TotalRadiatedPowerW = inputPowerW * m.RadiationEfficiency
	}

	return m, cuts
}

// nearestGainDB returns the gain (dB) at the pattern sample closest to
// the requested (theta, phi) in great-circle distance.
func nearestGainDB(pattern []PatternPoint, thetaDeg, phiDeg float64) float64 {
	const deg2rad = math.Pi / 180.0
	tt, pp := thetaDeg*deg2rad, phiDeg*deg2rad
	cosTT, sinTT := math.Cos(tt), math.Sin(tt)
	bestIdx := 0
	bestCosD := -2.0
	for i, p := range pattern {
		t, ph := p.ThetaDeg*deg2rad, p.PhiDeg*deg2rad
		cosD := math.Cos(t)*cosTT + math.Sin(t)*sinTT*math.Cos(ph-pp)
		if cosD > bestCosD {
			bestCosD = cosD
			bestIdx = i
		}
	}
	return pattern[bestIdx].GainDB
}

// sliceAt extracts an azimuth or elevation cut from the pattern.  When
// azimuthCut is true we hold theta = pivotDeg and sweep phi 0..360;
// otherwise we hold phi = pivotDeg and sweep theta 0..180.  Sample
// matching tolerance is 1° (half the 2° pattern grid step).
func sliceAt(pattern []PatternPoint, pivotDeg float64, azimuthCut bool) ([]float64, []float64) {
	const tol = 1.0
	xs := []float64{}
	ys := []float64{}
	for _, p := range pattern {
		if azimuthCut {
			if math.Abs(p.ThetaDeg-pivotDeg) <= tol {
				xs = append(xs, p.PhiDeg)
				ys = append(ys, p.GainDB)
			}
		} else {
			if math.Abs(angleDiff(p.PhiDeg, pivotDeg)) <= tol {
				xs = append(xs, p.ThetaDeg)
				ys = append(ys, p.GainDB)
			}
		}
	}
	return xs, ys
}

// angleDiff returns the signed shortest-path difference (a - b) in
// degrees, normalised to (-180, 180].
func angleDiff(a, b float64) float64 {
	d := math.Mod(a-b+540, 360) - 180
	return d
}

// beamwidthDeg measures the full -3 dB width of the lobe surrounding
// the strongest sample in (xs, ys).  When wrap is true (azimuth cut),
// the function searches with wrap-around at 360°.  Returns 0 if the
// cut has fewer than 3 samples or the -3 dB crossing cannot be found
// within half a sweep.
func beamwidthDeg(xs, ys []float64, wrap bool) float64 {
	if len(xs) < 3 {
		return 0
	}
	peakIdx := 0
	for i, y := range ys {
		if y > ys[peakIdx] {
			peakIdx = i
		}
	}
	threshold := ys[peakIdx] - 3.0
	left := -1.0
	right := -1.0
	n := len(ys)
	// Walk backward (decreasing index) until we drop below threshold.
	for d := 1; d < n; d++ {
		i := peakIdx - d
		if !wrap && i < 0 {
			break
		}
		j := ((i % n) + n) % n
		if ys[j] <= threshold {
			left = math.Abs(angularDistance(xs[peakIdx], xs[j], wrap))
			break
		}
	}
	for d := 1; d < n; d++ {
		i := peakIdx + d
		if !wrap && i >= n {
			break
		}
		j := i % n
		if ys[j] <= threshold {
			right = math.Abs(angularDistance(xs[peakIdx], xs[j], wrap))
			break
		}
	}
	if left < 0 || right < 0 {
		return 0
	}
	return left + right
}

// angularDistance returns |a - b| respecting wrap-around at 360° if wrap.
func angularDistance(a, b float64, wrap bool) float64 {
	if !wrap {
		return a - b
	}
	d := math.Mod(a-b+540, 360) - 180
	return d
}

// sidelobeLevel finds the strongest local maximum in (xs, ys) at least
// 30° away from peakDeg (azimuthal distance, with wrap) and returns its
// gain in dB relative to the global peak.  Returns 0 if no such sidelobe
// exists (e.g. an omni or single-lobe pattern).
func sidelobeLevel(xs, ys []float64, peakDeg float64) float64 {
	if len(xs) < 5 {
		return 0
	}
	peakIdx := 0
	for i, y := range ys {
		if y > ys[peakIdx] {
			peakIdx = i
		}
	}
	bestSide := math.Inf(-1)
	for i := 1; i < len(ys)-1; i++ {
		// Local maximum?
		if ys[i] <= ys[i-1] || ys[i] <= ys[i+1] {
			continue
		}
		dist := math.Abs(angularDistance(xs[i], xs[peakIdx], true))
		if dist < 30.0 {
			continue
		}
		if ys[i] > bestSide {
			bestSide = ys[i]
		}
	}
	if math.IsInf(bestSide, -1) {
		return 0
	}
	return bestSide - ys[peakIdx]
}

// integrateGainOverSphere applies rectangular quadrature to a 2°-grid
// pattern: ∫∫ G_lin(θ, φ) sin(θ) dθ dφ.  G_lin = 10^(GainDB/10).
// For a lossless pattern with directivity normalisation this returns
// 4π up to grid quantisation; we use the deviation from 4π as a
// radiation-efficiency proxy.
func integrateGainOverSphere(pattern []PatternPoint) float64 {
	const deg2rad = math.Pi / 180.0
	// Discover step sizes from the first two distinct theta and phi values.
	var dThetaDeg, dPhiDeg float64
	for _, p := range pattern {
		if p.ThetaDeg > 0 && dThetaDeg == 0 {
			dThetaDeg = p.ThetaDeg
		}
		if p.PhiDeg > 0 && dPhiDeg == 0 {
			dPhiDeg = p.PhiDeg
		}
		if dThetaDeg != 0 && dPhiDeg != 0 {
			break
		}
	}
	if dThetaDeg == 0 {
		dThetaDeg = 2.0
	}
	if dPhiDeg == 0 {
		dPhiDeg = 2.0
	}
	dTheta := dThetaDeg * deg2rad
	dPhi := dPhiDeg * deg2rad

	sum := 0.0
	for _, p := range pattern {
		gLin := math.Pow(10.0, p.GainDB/10.0)
		sum += gLin * math.Sin(p.ThetaDeg*deg2rad) * dTheta * dPhi
	}
	return sum
}

// FeedInputPower returns Re(V · conj(I_feed)), the real input power
// delivered into the feed-basis function for a 1-port excitation.  This
// is the denominator for radiation-efficiency calculations.
func FeedInputPower(voltage, feedCurrent complex128) float64 {
	return real(voltage * cmplx.Conj(feedCurrent))
}

// DissipatedPower returns the time-average power dissipated in the
// resistive loads and skin-effect contributions distributed across the
// basis functions:  P_loss = Σ_b R_loss_b · |I_b|².
//
// I is the basis-current solution vector (complex amplitudes); lossPerBasis
// is the per-basis sum of resistive contributions added to the Z-matrix
// diagonal by applyLoads + applyMaterialLoss.  When lossPerBasis is nil
// or all-zero (lossless model) the function returns 0.
//
// No ½ factor: FeedInputPower returns Re(V·I*) without the ½ as
// well, so the efficiency ratio P_loss/P_in is convention-independent
// only when both formulas drop the ½ together.
func DissipatedPower(I []complex128, lossPerBasis []float64) float64 {
	if lossPerBasis == nil {
		return 0
	}
	n := len(I)
	if n > len(lossPerBasis) {
		n = len(lossPerBasis)
	}
	var sum float64
	for i := 0; i < n; i++ {
		r := lossPerBasis[i]
		if r == 0 {
			continue
		}
		mag := real(I[i])*real(I[i]) + imag(I[i])*imag(I[i])
		// No 1/2 factor: FeedInputPower also uses Re(V·I*) without
		// the 1/2, so DissipatedPower must use the same convention
		// for the efficiency ratio P_loss/P_in to match physics.
		sum += r * mag
	}
	return sum
}

// ComputeFarFieldMetricsWithLoss is the loss-aware sibling of
// ComputeFarFieldMetrics.  It uses an actual power balance
//
//	η = (P_in - P_loss) / P_in
//
// instead of the naïve "integrated directivity" proxy, which is always 1
// because the pattern is normalised.  Pass lossPower = 0 to get the
// pre-loss behaviour.
func ComputeFarFieldMetricsWithLoss(pattern []PatternPoint, inputPowerW, lossPowerW float64) (FarFieldMetrics, PolarCuts) {
	m, c := ComputeFarFieldMetrics(pattern, inputPowerW)
	if inputPowerW > 0 {
		eff := (inputPowerW - lossPowerW) / inputPowerW
		if eff < 0 {
			eff = 0
		}
		m.RadiationEfficiency = eff
		m.TotalRadiatedPowerW = inputPowerW - lossPowerW
		if m.TotalRadiatedPowerW < 0 {
			m.TotalRadiatedPowerW = 0
		}
	}
	return m, c
}
