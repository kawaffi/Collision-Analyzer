package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/kawaffi/collision-analyzer/internal/physics"
)

type Event struct {
	Mu1, Mu2 physics.Muon
	// PrecomputedMass is the invariant mass stored in the dataset. It is meant
	// as a test oracle, not as the value the pipeline reports.
	PrecomputedMass float64
}

type Reader struct {
	csv *csv.Reader
}

func NewReader(r io.Reader) (*Reader, error) {
	cr := csv.NewReader(r)
	if _, err := cr.Read(); err != nil {
		return nil, fmt.Errorf("ingest: reading header: %w", err)
	}
	return &Reader{csv: cr}, nil
}

func (r *Reader) Read() (Event, error) {
	rec, err := r.csv.Read()
	if err != nil {
		return Event{}, err
	}

	f := func(i int) float64 {
		v, _ := strconv.ParseFloat(rec[i], 64)
		return v
	}

	return Event{
		Mu1:             physics.Muon{E: f(3), Px: f(4), Py: f(5), Pz: f(6)},
		Mu2:             physics.Muon{E: f(12), Px: f(13), Py: f(14), Pz: f(15)},
		PrecomputedMass: f(20),
	}, nil
}
