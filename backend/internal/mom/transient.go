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

package mom

import (
	"fmt"
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/dsp/fourier"
)

// ──────────────────────────────────────────────────────────────────────
// Time-domain transient analysis via frequency-domain IFFT
// ──────────────────────────────────────────────────────────────────────
//
// The antenna is characterised by its driving-point impedance Z(f)
// obtained from a dense frequency sweep.  We compute the feed-point
// reflection coefficient Γ(f) = (Z−Z₀)/(Z+Z₀), then apply the inverse
// FFT to get the time-domain impulse response of the reflected wave.
//
// The excitation pulse shape (Gaussian, step, modulated Gaussian, or
// custom) is specified in the frequency domain; the time-domain
// response is the IFFT of  Pulse(f) · H(f)  where H(f) is one of:
//
//   - "reflection":  Γ(f)           →  reflected voltage at the feed
//   - "input":       1/(1+Γ(f))     →  voltage delivered to antenna
//   - "current":     1/(Z(f))       →  feed-point current
//
// This is a standard technique in UWB antenna characterization (see
// Shlivinski et al., IEEE TAP 1997; Lamensdorf & Susman, IEEE A&P
// Magazine 1994).

// ──────────────────────────────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────────────────────────────

// TransientRequest specifies the time-domain analysis parameters.
type TransientRequest struct {
	Input SimulationInput

	// Frequency sweep range for the underlying MoM evaluation.
	FreqStartHz float64 `json:"freq_start_hz"`
	FreqEndHz   float64 `json:"freq_end_hz"`
	NumFreqs    int     `json:"num_freqs"` // number of frequency points (default 128)

	// Excitation pulse shape.
	PulseType string  `json:"pulse_type"` // "gaussian", "step", "modulated_gaussian"
	PulseWidthNs float64 `json:"pulse_width_ns"` // Gaussian sigma or step rise-time (ns)
	CenterFreqHz float64 `json:"center_freq_hz"` // carrier for modulated Gaussian (Hz)

	// Which transfer function to compute.
	Response string `json:"response"` // "reflection", "input", "current"
}

// TransientPoint is one sample in the time-domain waveform.
type TransientPoint struct {
	TimeNs    float64 `json:"time_ns"`
	Amplitude float64 `json:"amplitude"`
}

// TransientResult is the complete output of a transient analysis.
type TransientResult struct {
	// Time-domain waveform (real part of the IFFT).
	Waveform []TransientPoint `json:"waveform"`

	// The excitation pulse in the time domain for reference.
	Excitation []TransientPoint `json:"excitation"`

	// Frequency-domain data used (for optional plotting).
	Frequencies []float64 `json:"frequencies"` // MHz
	MagResponse []float64 `json:"mag_response"` // |H(f)| in dB
	PhaseResponse []float64 `json:"phase_response"` // arg(H(f)) in degrees

	// Summary metrics.
	PeakAmplitude   float64 `json:"peak_amplitude"`
	PeakTimeNs      float64 `json:"peak_time_ns"`
	RingdownTimeNs  float64 `json:"ringdown_time_ns"`  // time to -20 dB of peak
	PulseFWHMNs     float64 `json:"pulse_fwhm_ns"`     // FWHM of output pulse
	ResponseType    string  `json:"response_type"`
}

// ──────────────────────────────────────────────────────────────────────
// Engine
// ──────────────────────────────────────────────────────────────────────

// ComputeTransient runs a frequency sweep and applies IFFT to produce
// a time-domain transient response at the antenna feed point.
func ComputeTransient(req TransientRequest) (*TransientResult, error) {
	if req.FreqStartHz <= 0 || req.FreqEndHz <= req.FreqStartHz {
		return nil, fmt.Errorf("invalid frequency range")
	}

	numFreqs := req.NumFreqs
	if numFreqs <= 0 {
		numFreqs = 128
	}
	// Round up to power of 2 for efficient FFT
	nfft := nextPow2(numFreqs)
	if nfft < 16 {
		nfft = 16
	}
	if nfft > 512 {
		nfft = 512
	}

	pulseType := req.PulseType
	if pulseType == "" {
		pulseType = "gaussian"
	}
	responseType := req.Response
	if responseType == "" {
		responseType = "reflection"
	}
	pulseWidth := req.PulseWidthNs
	if pulseWidth <= 0 {
		pulseWidth = 2.0 // 2 ns default
	}

	z0 := req.Input.ReferenceImpedance
	if z0 <= 0 {
		z0 = DefaultReferenceImpedance
	}

	// ── 1. Run frequency sweep ──
	// We need nfft/2+1 positive-frequency points for a real-valued IFFT.
	nPos := nfft/2 + 1
	df := (req.FreqEndHz - req.FreqStartHz) / float64(nPos-1)

	freqs := make([]float64, nPos)
	zVals := make([]complex128, nPos) // complex impedance at each freq

	for i := 0; i < nPos; i++ {
		f := req.FreqStartHz + float64(i)*df
		freqs[i] = f

		inp := req.Input
		inp.Frequency = f
		result, err := Simulate(inp)
		if err != nil {
			// Use Z₀ for failed points (Γ=0, transparent)
			zVals[i] = complex(z0, 0)
			continue
		}
		zVals[i] = complex(result.Impedance.R, result.Impedance.X)
	}

	// ── 2. Compute transfer function H(f) ──
	hFreq := make([]complex128, nPos)
	for i := 0; i < nPos; i++ {
		z := zVals[i]
		z0c := complex(z0, 0)
		switch responseType {
		case "reflection":
			// Γ(f) = (Z - Z₀) / (Z + Z₀)
			hFreq[i] = (z - z0c) / (z + z0c)
		case "input":
			// Voltage delivered: V_in/V_source ∝ Z/(Z+Z₀)
			hFreq[i] = z / (z + z0c)
		case "current":
			// Feed current: I = V / Z
			if cmplx.Abs(z) > 1e-10 {
				hFreq[i] = 1.0 / z
			}
		default:
			hFreq[i] = (z - z0c) / (z + z0c)
		}
	}

	// ── 3. Generate excitation pulse spectrum ──
	pulseFreq := make([]complex128, nPos)
	sigmaSec := pulseWidth * 1e-9 // convert ns to seconds
	for i := 0; i < nPos; i++ {
		f := freqs[i]
		switch pulseType {
		case "gaussian":
			// Gaussian pulse: exp(-2π²σ²f²)
			arg := 2.0 * math.Pi * math.Pi * sigmaSec * sigmaSec * f * f
			pulseFreq[i] = complex(math.Exp(-arg), 0)
		case "step":
			// Step function with finite rise time: 1/(1 + j2πfτ)
			tau := sigmaSec
			pulseFreq[i] = 1.0 / complex(1.0, 2.0*math.Pi*f*tau)
		case "modulated_gaussian":
			// Gaussian envelope modulated by carrier: shifted Gaussian in freq
			fc := req.CenterFreqHz
			if fc <= 0 {
				fc = (req.FreqStartHz + req.FreqEndHz) / 2
			}
			arg := 2.0 * math.Pi * math.Pi * sigmaSec * sigmaSec * (f - fc) * (f - fc)
			pulseFreq[i] = complex(math.Exp(-arg), 0)
		default:
			pulseFreq[i] = complex(math.Exp(-2.0*math.Pi*math.Pi*sigmaSec*sigmaSec*freqs[i]*freqs[i]), 0)
		}
	}

	// ── 4. Multiply: output spectrum = H(f) × Pulse(f) ──
	outputFreq := make([]complex128, nPos)
	for i := 0; i < nPos; i++ {
		outputFreq[i] = hFreq[i] * pulseFreq[i]
	}

	// ── 5. IFFT using gonum/dsp/fourier ──
	// gonum's CmplxFFT works on full-length complex arrays.
	// For a real-valued result we use the half-complex → real IFFT.
	// Build the Hermitian-symmetric spectrum for gonum.
	fft := fourier.NewCmplxFFT(nfft)

	fullSpec := make([]complex128, nfft)
	for i := 0; i < nPos && i < nfft; i++ {
		fullSpec[i] = outputFreq[i]
	}
	// Mirror for negative frequencies (Hermitian symmetry for real output)
	for i := 1; i < nfft/2; i++ {
		fullSpec[nfft-i] = cmplx.Conj(fullSpec[i])
	}

	timeDomain := fft.Coefficients(nil, fullSpec)

	// ── Also IFFT the excitation pulse alone ──
	excFullSpec := make([]complex128, nfft)
	for i := 0; i < nPos && i < nfft; i++ {
		excFullSpec[i] = pulseFreq[i]
	}
	for i := 1; i < nfft/2; i++ {
		excFullSpec[nfft-i] = cmplx.Conj(excFullSpec[i])
	}
	excTimeDomain := fft.Coefficients(nil, excFullSpec)

	// ── 6. Build time axis ──
	// Total time window = 1/df; time step = 1/(nfft*df)
	dt := 1.0 / (float64(nfft) * df) // seconds
	dtNs := dt * 1e9                  // nanoseconds

	waveform := make([]TransientPoint, nfft)
	excitation := make([]TransientPoint, nfft)
	peakAmp := 0.0
	peakTime := 0.0

	for i := 0; i < nfft; i++ {
		t := float64(i) * dtNs
		amp := real(timeDomain[i]) / float64(nfft) // normalise IFFT
		excAmp := real(excTimeDomain[i]) / float64(nfft)

		waveform[i] = TransientPoint{TimeNs: t, Amplitude: amp}
		excitation[i] = TransientPoint{TimeNs: t, Amplitude: excAmp}

		if math.Abs(amp) > math.Abs(peakAmp) {
			peakAmp = amp
			peakTime = t
		}
	}

	// ── 7. Compute metrics ──
	ringdown := computeRingdown(waveform, peakAmp)
	fwhm := computeFWHM(waveform, peakAmp)

	// Frequency-domain magnitude/phase for plotting
	magResp := make([]float64, nPos)
	phaseResp := make([]float64, nPos)
	freqsMHz := make([]float64, nPos)
	for i := 0; i < nPos; i++ {
		freqsMHz[i] = freqs[i] / 1e6
		mag := cmplx.Abs(hFreq[i])
		if mag > 1e-30 {
			magResp[i] = 20 * math.Log10(mag)
		} else {
			magResp[i] = -60
		}
		phaseResp[i] = cmplx.Phase(hFreq[i]) * 180 / math.Pi
	}

	return &TransientResult{
		Waveform:        waveform,
		Excitation:      excitation,
		Frequencies:     freqsMHz,
		MagResponse:     magResp,
		PhaseResponse:   phaseResp,
		PeakAmplitude:   peakAmp,
		PeakTimeNs:      peakTime,
		RingdownTimeNs:  ringdown,
		PulseFWHMNs:     fwhm,
		ResponseType:    responseType,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────

func nextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// computeRingdown finds the time at which the envelope falls to -20 dB
// of the peak (i.e. 10% of peak amplitude) after the peak.
func computeRingdown(waveform []TransientPoint, peak float64) float64 {
	threshold := math.Abs(peak) * 0.1 // -20 dB
	peakIdx := 0
	for i, p := range waveform {
		if math.Abs(p.Amplitude) >= math.Abs(peak)*0.999 {
			peakIdx = i
			break
		}
	}
	for i := peakIdx; i < len(waveform); i++ {
		if math.Abs(waveform[i].Amplitude) < threshold {
			return waveform[i].TimeNs - waveform[peakIdx].TimeNs
		}
	}
	if len(waveform) > 0 {
		return waveform[len(waveform)-1].TimeNs - waveform[peakIdx].TimeNs
	}
	return 0
}

// computeFWHM estimates the full-width at half-maximum of the main pulse.
func computeFWHM(waveform []TransientPoint, peak float64) float64 {
	halfPeak := math.Abs(peak) * 0.5
	first := -1.0
	last := -1.0
	for _, p := range waveform {
		if math.Abs(p.Amplitude) >= halfPeak {
			if first < 0 {
				first = p.TimeNs
			}
			last = p.TimeNs
		}
	}
	if first >= 0 && last > first {
		return last - first
	}
	return 0
}
