package mom

import (
	"math"
	"testing"
)

// makeIsotropic builds a synthetic 2°-grid pattern that is uniform 0
// dBi everywhere — i.e. a pure isotropic radiator.  Useful for
// verifying that the metrics extractor produces sensible defaults on a
// pathological pattern.
func makeIsotropic(gainDB float64) []PatternPoint {
	const step = 2.0
	out := []PatternPoint{}
	for it := 0; it <= 90; it++ {
		theta := float64(it) * step
		for ip := 0; ip <= 180; ip++ {
			out = append(out, PatternPoint{
				ThetaDeg: theta,
				PhiDeg:   float64(ip) * step,
				GainDB:   gainDB,
			})
		}
	}
	return out
}

// makeCosinePattern fakes a (cos²θ)-shaped pattern peaked at θ=90°
// (horizon, broadside-of-vertical-dipole-like) so we can sanity-check
// peak-finding, beamwidth, and front/back behaviour.
func makeCosinePattern() []PatternPoint {
	const step = 2.0
	out := []PatternPoint{}
	for it := 0; it <= 90; it++ {
		theta := float64(it) * step
		t := theta * math.Pi / 180.0
		// Peak at θ=90°: gain = cos²(θ - 90°) → 1 at horizon, 0 at zenith/nadir.
		gLin := math.Pow(math.Sin(t), 2.0) + 1e-6
		gDB := 10 * math.Log10(gLin)
		for ip := 0; ip <= 180; ip++ {
			out = append(out, PatternPoint{
				ThetaDeg: theta,
				PhiDeg:   float64(ip) * step,
				GainDB:   gDB,
			})
		}
	}
	return out
}

func TestMetrics_IsotropicPattern(t *testing.T) {
	p := makeIsotropic(0)
	m, c := ComputeFarFieldMetrics(p, 1.0)
	if math.Abs(m.PeakGainDB) > 0.01 {
		t.Fatalf("isotropic peak gain should be 0 dBi, got %v", m.PeakGainDB)
	}
	if m.FrontToBackDB != 0 {
		t.Fatalf("isotropic F/B should be 0, got %v", m.FrontToBackDB)
	}
	// Efficiency proxy on isotropic 0 dBi pattern integrates to 4π → ratio ≈ 1.
	if math.Abs(m.RadiationEfficiency-1.0) > 0.05 {
		t.Fatalf("isotropic efficiency proxy should be ≈1, got %v", m.RadiationEfficiency)
	}
	if len(c.AzimuthDeg) == 0 || len(c.ElevationDeg) == 0 {
		t.Fatal("polar cuts should be non-empty")
	}
}

func TestMetrics_CosineDipoleLikePattern(t *testing.T) {
	p := makeCosinePattern()
	m, c := ComputeFarFieldMetrics(p, 1.0)
	// Peak should be at theta = 90° (horizon).
	if math.Abs(m.PeakThetaDeg-90) > 2.001 {
		t.Fatalf("cosine pattern peak theta should be ≈90°, got %v", m.PeakThetaDeg)
	}
	// F/B for a θ-symmetric pattern is 0 (gain at 90° = gain at 90°
	// after antipode reflection of (90°, φ) → (90°, φ+180°)).
	if math.Abs(m.FrontToBackDB) > 0.01 {
		t.Fatalf("symmetric cosine pattern F/B should be 0, got %v", m.FrontToBackDB)
	}
	// 3 dB azimuthal beamwidth for the φ-uniform cut should equal the
	// full 360° sweep (gain doesn't drop below -3 dB anywhere in φ).
	if m.BeamwidthAzDeg != 0 && m.BeamwidthAzDeg < 90 {
		t.Fatalf("azimuth beamwidth on uniform cut should be 0 (no -3dB crossing) or wide, got %v", m.BeamwidthAzDeg)
	}
	// 3 dB elevation beamwidth for cos²θ centred at 90° is 90° total
	// (45° each side).  Allow ±4° for grid quantisation.
	if math.Abs(m.BeamwidthElDeg-90) > 4 {
		t.Fatalf("elevation beamwidth should be ≈90°, got %v", m.BeamwidthElDeg)
	}
	if len(c.AzimuthDeg) == 0 {
		t.Fatal("azimuth cut should have samples")
	}
}

// End-to-end: run a real dipole through Simulate() and verify metrics
// land in the result.
func TestSimulate_PopulatesMetricsAndCuts(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	geom := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: -halfL, Z1: 0,
			X2: 0, Y2: halfL, Z2: 0,
			Radius:   1e-3,
			Segments: 21,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}
	res, err := Simulate(geom)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}

	// Peak gain should match the legacy GainDBi field.
	if math.Abs(res.Metrics.PeakGainDB-res.GainDBi) > 1e-9 {
		t.Fatalf("metrics peak (%v) ≠ legacy GainDBi (%v)", res.Metrics.PeakGainDB, res.GainDBi)
	}
	// A free-space dipole should have a sensible (~2.15 dBi) directivity.
	if res.Metrics.PeakGainDB < 1.0 || res.Metrics.PeakGainDB > 4.0 {
		t.Fatalf("dipole peak gain unrealistic: %v dBi", res.Metrics.PeakGainDB)
	}
	// Lossless dipole: efficiency should hover near 1.
	if res.Metrics.RadiationEfficiency < 0.85 || res.Metrics.RadiationEfficiency > 1.20 {
		t.Fatalf("lossless dipole efficiency proxy off: %v", res.Metrics.RadiationEfficiency)
	}
	// Polar cuts should be populated.
	if len(res.Cuts.AzimuthDeg) == 0 || len(res.Cuts.ElevationDeg) == 0 {
		t.Fatal("polar cuts empty")
	}
	if len(res.Cuts.AzimuthDeg) != len(res.Cuts.AzimuthGainDB) {
		t.Fatal("azimuth cut x/y length mismatch")
	}
	if len(res.Cuts.ElevationDeg) != len(res.Cuts.ElevationGainDB) {
		t.Fatal("elevation cut x/y length mismatch")
	}
	// Input power should be ≈ Re(1 · conj(I_feed)).  A non-zero feed
	// current means non-zero input power.
	if res.Metrics.InputPowerW <= 0 {
		t.Fatalf("expected positive input power, got %v", res.Metrics.InputPowerW)
	}
}

// Adding a 50 Ω lossy series load at the feed of an otherwise lossless
// dipole should drop the radiation efficiency well below 1.
func TestMetrics_LoadDropsEfficiency(t *testing.T) {
	freq := 14e6
	wavelength := C0 / freq
	halfL := wavelength / 4
	geom := SimulationInput{
		Frequency: freq,
		Wires: []Wire{{
			X1: 0, Y1: -halfL, Z1: 0,
			X2: 0, Y2: halfL, Z2: 0,
			Radius:   1e-3,
			Segments: 21,
		}},
		Source: Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
		Ground: GroundConfig{Type: "free_space"},
	}
	base, err := Simulate(geom)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	geom.Loads = []Load{{
		WireIndex:    0,
		SegmentIndex: 10,
		Topology:     LoadSeriesRLC,
		R:            500, // big lossy resistor in series with feed
	}}
	loaded, err := Simulate(geom)
	if err != nil {
		t.Fatalf("loaded: %v", err)
	}
	if loaded.Metrics.RadiationEfficiency >= base.Metrics.RadiationEfficiency {
		t.Fatalf("efficiency should drop with a 500 Ω feed load: base=%.3f loaded=%.3f",
			base.Metrics.RadiationEfficiency, loaded.Metrics.RadiationEfficiency)
	}
	if loaded.Metrics.RadiationEfficiency > 0.5 {
		t.Fatalf("expected efficiency well below 0.5 with a 500 Ω lossy load, got %.3f",
			loaded.Metrics.RadiationEfficiency)
	}
}
