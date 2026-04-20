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

import "gonum.org/v1/gonum/mat"

// cdenseAdder adapts a *mat.CDense to the zMatSetter interface used by
// applyMaterialLoss.  It performs Z[i,j] += v with the locking
// discipline matching the rest of the solver (single-threaded use; the
// material-loss pass runs after the parallel Z-fill is complete).
type cdenseAdder struct{ Z *mat.CDense }

func (a cdenseAdder) Add(i, j int, v complex128) {
	cur := a.Z.At(i, j)
	a.Z.Set(i, j, cur+v)
}
