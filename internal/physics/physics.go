package physics

import "math"

type Muon struct {
	E, Px, Py, Pz float64
}

func InvariantMass(a, b Muon) float64 {
	e := a.E + b.E
	px := a.Px + b.Px
	py := a.Py + b.Py
	pz := a.Pz + b.Pz

	return math.Sqrt(e*e - px*px - py*py - pz*pz)
}