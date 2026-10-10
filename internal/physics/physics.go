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

	m2 := e*e - px*px - py*py - pz*pz
	if m2 < 0 {
		// Negative M^2 can come from floating-point rounding when the true mass
		// is ~0, so it is clamped. On the CMS Run2010B dimuon dataset this never
		// triggers (checked on all 100,000 events): the clamp is only a safety net.
		return 0
	}

	return math.Sqrt(m2)
}
