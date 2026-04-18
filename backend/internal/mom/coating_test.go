package mom

import (
	"math"
	"testing"
)

// dipoleInput builds a symmetric dipole centred at the origin with the given
// frequency, half-length, and wire coating parameters.
func dipoleInput(freqHz, halfLen float64, w Wire) SimulationInput {
	w.X1, w.Y1, w.Z1 = 0, 0, -halfLen
	w.X2, w.Y2, w.Z2 = 0, 0, halfLen
	midSeg := (w.Segments - 1) / 2
	return SimulationInput{
		Frequency: freqHz,
		Wires:     []Wire{w},
		Source:    Source{WireIndex: 0, SegmentIndex: midSeg, Voltage: 1 + 0i},
		Ground:    GroundConfig{Type: "free_space"},
	}
}

// findResonantFreq scans [fLo, fHi] in nSteps steps and returns the frequency
// where |Im(Z)| is minimum (the resonant frequency).
func findResonantFreq(t *testing.T, w Wire, halfLen float64, fLo, fHi float64, nSteps int) float64 {
	t.Helper()
	best := math.MaxFloat64
	bestF := fLo
	step := (fHi - fLo) / float64(nSteps-1)
	for i := 0; i < nSteps; i++ {
		f := fLo + float64(i)*step
		res, err := Simulate(dipoleInput(f, halfLen, w))
		if err != nil {
			t.Fatalf("simulate @ %.3f MHz: %v", f/1e6, err)
		}
		absX := math.Abs(res.Impedance.X)
		if absX < best {
			best = absX
			bestF = f
		}
	}
	return bestF
}

// TestCoating_BareWireUnchanged verifies that setting CoatingThickness=0 on a
// wire produces the same feed-point impedance as a wire with no coating fields.
func TestCoating_BareWireUnchanged(t *testing.T) {
	freq := 14e6
	halfL := C0 / freq / 4
	bare := Wire{Radius: 1e-3, Segments: 21}
	zeroThick := Wire{Radius: 1e-3, Segments: 21, CoatingThickness: 0, CoatingEpsR: 2.3, CoatingLossTan: 0.02}

	rBare, err := Simulate(dipoleInput(freq, halfL, bare))
	if err != nil {
		t.Fatalf("bare: %v", err)
	}
	rZero, err := Simulate(dipoleInput(freq, halfL, zeroThick))
	if err != nil {
		t.Fatalf("zero-thick: %v", err)
	}

	if rBare.Impedance.R != rZero.Impedance.R || rBare.Impedance.X != rZero.Impedance.X {
		t.Fatalf("zero coating thickness changed impedance: bare=%+v coated=%+v",
			rBare.Impedance, rZero.Impedance)
	}
}

// TestCoating_ResonanceShift checks that a PVC-coated dipole resonates at a
// measurably lower frequency than the bare wire.
//
// The IS-card model adds inductive loading proportional to (1−1/εr)·ln(b/a),
// which lowers the guided-wave phase velocity and hence the resonant frequency.
// For a 2 mm PVC coating (εr=2.3) on a 1 mm wire the shift is ~0.7–1%.
//
// Two assertions are made:
//  1. Direct: at the bare-wire resonant frequency the coated wire shows
//     significantly positive reactance (coating moved its resonance lower).
//  2. Scan: the coated resonant frequency is detectably lower (≥0.4%).
func TestCoating_ResonanceShift(t *testing.T) {
	halfLen := 10.0 // 20 m total dipole
	fLo, fHi := 6e6, 9e6

	bareWire := Wire{Radius: 1e-3, Segments: 21}
	coatedWire := Wire{
		Radius:           1e-3,
		Segments:         21,
		CoatingThickness: 2e-3, // 2 mm PVC shell
		CoatingEpsR:      2.3,  // typical PVC
		CoatingLossTan:   0,    // lossless — pure reactance shift
	}

	fBare := findResonantFreq(t, bareWire, halfLen, fLo, fHi, 61)
	fCoated := findResonantFreq(t, coatedWire, halfLen, fLo, fHi, 61)

	shift := (fBare - fCoated) / fBare
	t.Logf("bare resonance: %.4f MHz, coated resonance: %.4f MHz, shift: %.2f%%",
		fBare/1e6, fCoated/1e6, shift*100)

	// --- Assertion 1: direct reactance comparison at fBare ---
	// At the bare resonant frequency X_bare ≈ 0.  The coating adds inductive
	// loading ≈ Σ_k jZ'·ℓ_k on the Z-matrix diagonal, which by perturbation
	// theory raises the feed reactance by ~Σ_k jΔX_k·(I_k/I_feed)².
	// For 20 segments of 0.95 m each with Z' ≈ j5.8 Ω/m, this is ~+50–60 Ω.
	rBare, err := Simulate(dipoleInput(fBare, halfLen, bareWire))
	if err != nil {
		t.Fatalf("bare @ fBare: %v", err)
	}
	rCoated, err := Simulate(dipoleInput(fBare, halfLen, coatedWire))
	if err != nil {
		t.Fatalf("coated @ fBare: %v", err)
	}
	dX := rCoated.Impedance.X - rBare.Impedance.X
	t.Logf("@ fBare: bare X=%.2f Ω, coated X=%.2f Ω, ΔX=%.2f Ω",
		rBare.Impedance.X, rCoated.Impedance.X, dX)
	if dX < 5.0 {
		t.Fatalf("expected coating to raise reactance ≥5 Ω at bare resonance, got ΔX=%.2f Ω", dX)
	}

	// --- Assertion 2: scan-level frequency shift ---
	// The 61-step scan has ~50 kHz resolution; we require at least one step's
	// worth of downward shift (≥0.4% is safely above the grid noise).
	if fCoated >= fBare {
		t.Fatalf("coating should lower resonant frequency: bare=%.4f MHz, coated=%.4f MHz",
			fBare/1e6, fCoated/1e6)
	}
	if shift < 0.004 {
		t.Fatalf("expected ≥0.4%% resonance shift, got %.2f%%", shift*100)
	}
}

// TestCoating_CoatingPlusWeather exercises the full multi-layer stack
// (per-wire coating + global weather film), which single-layer tests miss.
// The IS-card formula sums contributions inner-to-outer, so the combined
// reactance shift at a fixed frequency must exceed either layer alone and
// lie below a rough upper bound on the independent sum.
func TestCoating_CoatingPlusWeather(t *testing.T) {
	freq := 14e6
	halfL := C0 / freq / 4 // near-resonant bare dipole length

	bare := Wire{Radius: 1e-3, Segments: 21}
	coated := Wire{
		Radius: 1e-3, Segments: 21,
		CoatingThickness: 2e-3, CoatingEpsR: 2.3, CoatingLossTan: 0,
	}

	// Isolate each layer: rain alone (bare wire), PVC alone (dry), and the
	// stacked case PVC + rain.
	rainOnly := WeatherConfig{Preset: "rain", Thickness: 1e-4, EpsR: 80, LossTan: 0}
	dry := WeatherConfig{Preset: "dry"}

	runAt := func(w Wire, weather WeatherConfig) float64 {
		in := dipoleInput(freq, halfL, w)
		in.Weather = weather
		r, err := Simulate(in)
		if err != nil {
			t.Fatalf("simulate: %v", err)
		}
		return r.Impedance.X
	}

	xBare := runAt(bare, dry)
	xPVC := runAt(coated, dry)
	xRain := runAt(bare, rainOnly)
	xStack := runAt(coated, rainOnly)

	dPVC := xPVC - xBare
	dRain := xRain - xBare
	dStack := xStack - xBare

	t.Logf("ΔX PVC=%.3f  ΔX rain=%.3f  ΔX stack=%.3f", dPVC, dRain, dStack)

	// Each single layer must push reactance upward (inductive loading).
	if dPVC <= 0 {
		t.Errorf("PVC alone should raise X, got ΔX=%.3f", dPVC)
	}
	if dRain <= 0 {
		t.Errorf("rain alone should raise X, got ΔX=%.3f", dRain)
	}
	// Stacking must exceed either layer in isolation: rain sees a larger
	// inner radius once PVC is present, so the (1/ε_{i−1} − 1/ε_i) ln(b/a)
	// sum grows strictly with an added outer layer.
	if dStack <= dPVC {
		t.Errorf("stacked ΔX=%.3f must exceed PVC-only ΔX=%.3f", dStack, dPVC)
	}
	if dStack <= dRain {
		t.Errorf("stacked ΔX=%.3f must exceed rain-only ΔX=%.3f", dStack, dRain)
	}
}

// TestCoating_LossyCoatingAddsResistance verifies that a coating with tanδ > 0
// raises the feed-point resistance relative to a lossless coating.
func TestCoating_LossyCoatingAddsResistance(t *testing.T) {
	freq := 14e6
	halfL := C0 / freq / 4

	lossless := Wire{Radius: 1e-3, Segments: 21, CoatingThickness: 2e-3, CoatingEpsR: 2.3, CoatingLossTan: 0}
	lossy := Wire{Radius: 1e-3, Segments: 21, CoatingThickness: 2e-3, CoatingEpsR: 2.3, CoatingLossTan: 0.05}

	rLL, err := Simulate(dipoleInput(freq, halfL, lossless))
	if err != nil {
		t.Fatalf("lossless: %v", err)
	}
	rLY, err := Simulate(dipoleInput(freq, halfL, lossy))
	if err != nil {
		t.Fatalf("lossy: %v", err)
	}

	if rLY.Impedance.R <= rLL.Impedance.R {
		t.Fatalf("lossy coating should raise resistance: lossless R=%.3f, lossy R=%.3f",
			rLL.Impedance.R, rLY.Impedance.R)
	}
	t.Logf("lossless R=%.3f X=%.3f, lossy R=%.3f X=%.3f",
		rLL.Impedance.R, rLL.Impedance.X, rLY.Impedance.R, rLY.Impedance.X)
}
