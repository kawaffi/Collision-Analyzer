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
		{
			name: "two massive particles at rest",
			a:    Muon{E: 3},
			b:    Muon{E: 3},
			want: 6,
		},
		{
			name: "two massless particles in the same direction",
			a:    Muon{E: 5, Pz: 5},
			b:    Muon{E: 5, Pz: 5},
			want: 0,
		},
		{
			name: "momenta on different axes",
			a:    Muon{E: 5, Px: 3},
			b:    Muon{E: 5, Py: 4},
			want: math.Sqrt(75),
		},
		{
			name: "slightly negative squared mass from rounding",
			a:    Muon{E: 5, Pz: 5},
			b:    Muon{E: 5, Pz: 5 + 1e-12},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InvariantMass(tt.a, tt.b)
			if math.IsNaN(got) || math.Abs(got-tt.want) > tolerance {
				t.Errorf("InvariantMass() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInvariantMassIsSymmetric(t *testing.T) {
	a := Muon{E: 12.5, Px: 3.1, Py: -4.2, Pz: 10.7}
	b := Muon{E: 8.3, Px: -1.5, Py: 2.2, Pz: -6.9}

	if got, want := InvariantMass(a, b), InvariantMass(b, a); got != want {
		t.Errorf("InvariantMass(a, b) = %v, InvariantMass(b, a) = %v", got, want)
	}
}
