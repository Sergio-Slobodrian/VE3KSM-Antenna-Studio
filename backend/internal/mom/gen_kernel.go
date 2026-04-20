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
	"math"
	"runtime"
	"sync"

	"gonum.org/v1/gonum/mat"
)

// GenKernel computes the MPIE impedance matrix element Z_mn between two
// generalised basis functions.  It is structurally identical to TriangleKernel
// but evaluates the abstract shape functions (ShapeLeft/ShapeRight) instead of
// hard-coded (1±t)/2 linear weights.
//
// For triangle order this produces identical results to TriangleKernel.
// For sinusoidal order the φ(t) calls evaluate sin(k·s)/sin(k·Δl).
// For quadratic order the φ(t) calls evaluate the Hermite cubic rise/fall.
//
// Quadrature order is automatically increased for non-triangle bases because
// the integrands are more oscillatory.
func GenKernel(bM, bN GenBasis, k, omega float64, segments []Segment) (vectorTerm, scalarTerm complex128) {
	// Base quadrature order: triangle=8, sinusoidal/quadratic=12
	nQuad := 8
	if bM.Order != BasisTriangle {
		nQuad = 12
	}

	type halfInfo struct {
		seg       *Segment
		shape     BasisFunc
		chargeDen float64
	}

	halfsM := make([]halfInfo, 0, 2)
	if bM.SegLeft != nil && bM.ShapeLeft != nil {
		halfsM = append(halfsM, halfInfo{bM.SegLeft, bM.ShapeLeft, bM.ChargeDensLeft})
	}
	if bM.SegRight != nil && bM.ShapeRight != nil {
		halfsM = append(halfsM, halfInfo{bM.SegRight, bM.ShapeRight, bM.ChargeDensRight})
	}

	halfsN := make([]halfInfo, 0, 2)
	if bN.SegLeft != nil && bN.ShapeLeft != nil {
		halfsN = append(halfsN, halfInfo{bN.SegLeft, bN.ShapeLeft, bN.ChargeDensLeft})
	}
	if bN.SegRight != nil && bN.ShapeRight != nil {
		halfsN = append(halfsN, halfInfo{bN.SegRight, bN.ShapeRight, bN.ChargeDensRight})
	}

	nodes, weights := GaussLegendre(nQuad)
	nodesHQ, weightsHQ := GaussLegendre(nQuad * 2)

	for _, hm := range halfsM {
		for _, hn := range halfsN {
			segA := hm.seg
			segB := hn.seg

			selfTerm := segA.Index == segB.Index
			useRadius := selfTerm
			radius := segA.Radius
			if segB.Radius > radius {
				radius = segB.Radius
			}

			qNodes := nodes
			qWeights := weights
			nq := nQuad
			if selfTerm {
				qNodes = nodesHQ
				qWeights = weightsHQ
				nq = nQuad * 2
			}

			dirDot := segA.Direction[0]*segB.Direction[0] +
				segA.Direction[1]*segB.Direction[1] +
				segA.Direction[2]*segB.Direction[2]

			var vecInt, scaInt complex128

			for p := 0; p < nq; p++ {
				wp := qWeights[p]
				tp := qNodes[p]
				pa := [3]float64{
					segA.Center[0] + tp*segA.HalfLength*segA.Direction[0],
					segA.Center[1] + tp*segA.HalfLength*segA.Direction[1],
					segA.Center[2] + tp*segA.HalfLength*segA.Direction[2],
				}
				phiM := hm.shape.Phi(tp)

				for q := 0; q < nq; q++ {
					wq := qWeights[q]
					tq := qNodes[q]
					pb := [3]float64{
						segB.Center[0] + tq*segB.HalfLength*segB.Direction[0],
						segB.Center[1] + tq*segB.HalfLength*segB.Direction[1],
						segB.Center[2] + tq*segB.HalfLength*segB.Direction[2],
					}
					phiN := hn.shape.Phi(tq)

					R := dist(pa, pb, useRadius, radius)
					psiVal := psi(k, R)

					vecInt += complex(wp*wq*phiM*phiN*dirDot, 0) * psiVal
					scaInt += complex(wp*wq, 0) * psiVal
				}
			}

			jacobian := complex(segA.HalfLength*segB.HalfLength, 0)
			vecInt *= jacobian
			scaInt *= jacobian

			vectorTerm += vecInt
			scalarTerm += complex(hm.chargeDen*hn.chargeDen, 0) * scaInt
		}
	}

	return vectorTerm, scalarTerm
}

// GenKernelPerfectGround computes the PEC ground-image contribution for
// generalised basis functions (same structure as TriangleKernelPerfectGround
// but with abstract shape functions).
func GenKernelPerfectGround(bM, bN GenBasis, k float64) (vectorTerm, scalarTerm complex128) {
	nQuad := 8
	if bM.Order != BasisTriangle {
		nQuad = 12
	}

	type halfInfo struct {
		seg       *Segment
		shape     BasisFunc
		chargeDen float64
	}

	halfsM := make([]halfInfo, 0, 2)
	if bM.SegLeft != nil && bM.ShapeLeft != nil {
		halfsM = append(halfsM, halfInfo{bM.SegLeft, bM.ShapeLeft, bM.ChargeDensLeft})
	}
	if bM.SegRight != nil && bM.ShapeRight != nil {
		halfsM = append(halfsM, halfInfo{bM.SegRight, bM.ShapeRight, bM.ChargeDensRight})
	}

	halfsN := make([]halfInfo, 0, 2)
	if bN.SegLeft != nil && bN.ShapeLeft != nil {
		halfsN = append(halfsN, halfInfo{bN.SegLeft, bN.ShapeLeft, bN.ChargeDensLeft})
	}
	if bN.SegRight != nil && bN.ShapeRight != nil {
		halfsN = append(halfsN, halfInfo{bN.SegRight, bN.ShapeRight, bN.ChargeDensRight})
	}

	nodes, weights := GaussLegendre(nQuad)
	nodesHQ, weightsHQ := GaussLegendre(nQuad * 2)

	for _, hm := range halfsM {
		for _, hn := range halfsN {
			segA := hm.seg
			segB := hn.seg

			selfTerm := segA.Index == segB.Index
			radius := segA.Radius
			if segB.Radius > radius {
				radius = segB.Radius
			}

			qNodes := nodes
			qWeights := weights
			nq := nQuad
			if selfTerm {
				qNodes = nodesHQ
				qWeights = weightsHQ
				nq = nQuad * 2
			}

			dirDotImage := -segA.Direction[0]*segB.Direction[0] -
				segA.Direction[1]*segB.Direction[1] +
				segA.Direction[2]*segB.Direction[2]

			var vecInt, scaInt complex128

			for p := 0; p < nq; p++ {
				wp := qWeights[p]
				tp := qNodes[p]
				pa := [3]float64{
					segA.Center[0] + tp*segA.HalfLength*segA.Direction[0],
					segA.Center[1] + tp*segA.HalfLength*segA.Direction[1],
					segA.Center[2] + tp*segA.HalfLength*segA.Direction[2],
				}
				phiM := hm.shape.Phi(tp)

				for q := 0; q < nq; q++ {
					wq := qWeights[q]
					tq := qNodes[q]
					pb := [3]float64{
						segB.Center[0] + tq*segB.HalfLength*segB.Direction[0],
						segB.Center[1] + tq*segB.HalfLength*segB.Direction[1],
						segB.Center[2] + tq*segB.HalfLength*segB.Direction[2],
					}
					phiN := hn.shape.Phi(tq)

					pbImage := [3]float64{pb[0], pb[1], -pb[2]}
					RImage := dist(pa, pbImage, selfTerm, radius)
					psiImage := psi(k, RImage)

					vecInt += complex(wp*wq*phiM*phiN*dirDotImage, 0) * psiImage
					scaInt -= complex(wp*wq, 0) * psiImage
				}
			}

			jacobian := complex(segA.HalfLength*segB.HalfLength, 0)
			vecInt *= jacobian
			scaInt *= jacobian

			vectorTerm += vecInt
			scalarTerm += complex(hm.chargeDen*hn.chargeDen, 0) * scaInt
		}
	}

	return vectorTerm, scalarTerm
}

// BuildGenZMatrix assembles the impedance matrix using generalised basis
// functions.  Uses GenKernel instead of TriangleKernel.
func BuildGenZMatrix(genBases []GenBasis, segments []Segment, k, omega float64) *mat.CDense {
	n := len(genBases)
	Z := mat.NewCDense(n, n, nil)

	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}

	type job struct{ i, j int }
	jobs := make(chan job, 256)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for jb := range jobs {
				vecTerm, scaTerm := GenKernel(genBases[jb.i], genBases[jb.j], k, omega, segments)
				val := vecPrefactor*vecTerm + scaPrefactor*scaTerm
				mu.Lock()
				Z.Set(jb.i, jb.j, val)
				mu.Unlock()
			}
		}()
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			jobs <- job{i, j}
		}
	}
	close(jobs)
	wg.Wait()

	return Z
}

// AddGenGroundBasis adds PEC ground image contributions for generalised bases.
func AddGenGroundBasis(Z *mat.CDense, genBases []GenBasis, k, omega float64) {
	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	n := len(genBases)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			vecTerm, scaTerm := GenKernelPerfectGround(genBases[i], genBases[j], k)
			val := vecPrefactor*vecTerm + scaPrefactor*scaTerm
			old := Z.At(i, j)
			Z.Set(i, j, old+val)
		}
	}
}
