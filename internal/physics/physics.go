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
		// Negative M^2 is expected from floating-point rounding when the true
		// mass is ~0, so we clamp it. TODO: once ingest exists, reject events
		// whose M^2 is far below the rounding error instead of clamping them,
		// so corrupted rows are not counted as valid events.
		return 0
	}

	return math.Sqrt(m2)
}
