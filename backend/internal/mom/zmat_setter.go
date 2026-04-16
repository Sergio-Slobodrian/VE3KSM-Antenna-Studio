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
