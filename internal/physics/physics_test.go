package physics

import (
	"math"
	"testing"
)

func TestInvariantMass(t *testing.T) {
	const tolerance = 1e-9

	tests := []struct {
		name string
		a, b Muon
		want float64
	}{
		{
			name: "two massless particles back to back",
			a:    Muon{E: 5, Px: 0, Py: 0, Pz: 5},
			b:    Muon{E: 5, Px: 0, Py: 0, Pz: -5},
			want: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InvariantMass(tt.a, tt.b)
			if math.Abs(got-tt.want) > tolerance {
				t.Errorf("InvariantMass() = %v, want %v", got, tt.want)
			}
		})
	}
}